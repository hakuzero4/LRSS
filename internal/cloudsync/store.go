// Package cloudsync uploads/downloads subscription OPML to WebDAV or S3-compatible storage.
// Scope: subscription list only — never reading state or article bodies.
package cloudsync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"lrss/internal/httpx"
	"lrss/internal/settings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	defaultTimeout = 45 * time.Second
	defaultUA      = "LRSS/0.1 (+https://local; sync)"
	maxOPMLBytes   = 8 << 20 // 8 MiB
)

// BlobStore puts/gets raw OPML bytes for a configured remote.
type BlobStore interface {
	// Put writes body as the subscription OPML object.
	Put(ctx context.Context, body []byte) error
	// Get downloads the subscription OPML object.
	Get(ctx context.Context) ([]byte, error)
	// Ping checks credentials / reachability (may write nothing, or HEAD/list).
	Ping(ctx context.Context) error
}

// NewStore builds a BlobStore from SyncConfig.
func NewStore(cfg settings.SyncConfig) (BlobStore, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled || cfg.Provider == settings.SyncProviderNone {
		return nil, fmt.Errorf("sync is not enabled")
	}
	switch cfg.Provider {
	case settings.SyncProviderWebDAV:
		return newWebDAV(cfg)
	case settings.SyncProviderS3:
		return newS3(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

// —— WebDAV ——

type webdavStore struct {
	url      string
	user     string
	password string
	http     *http.Client
}

func newWebDAV(cfg settings.SyncConfig) (*webdavStore, error) {
	target, err := webdavTargetURL(cfg)
	if err != nil {
		return nil, err
	}
	return &webdavStore{
		url:      target,
		user:     cfg.WebDAVUsername,
		password: cfg.WebDAVPassword,
		http: httpx.Std(httpx.Options{
			Timeout:   defaultTimeout,
			UserAgent: defaultUA,
		}),
	}, nil
}

func webdavTargetURL(cfg settings.SyncConfig) (string, error) {
	base := strings.TrimSpace(cfg.WebDAVURL)
	if base == "" {
		return "", fmt.Errorf("webdav url required")
	}
	if p := strings.TrimSpace(cfg.WebDAVPath); p != "" {
		// Absolute path on same host, or full URL.
		if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
			return p, nil
		}
		u, err := url.Parse(base)
		if err != nil {
			return "", err
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		u.Path = path.Clean(p)
		return u.String(), nil
	}
	// Append object key to base URL path.
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	key := cfg.ObjectKey
	if key == "" {
		key = settings.DefaultSyncObjectKey
	}
	u.Path = path.Join(strings.TrimSuffix(u.Path, "/"), key)
	return u.String(), nil
}

func (w *webdavStore) Put(ctx context.Context, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, w.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("User-Agent", defaultUA)
	if w.user != "" || w.password != "" {
		req.SetBasicAuth(w.user, w.password)
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("webdav put: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webdav put: http %d", resp.StatusCode)
	}
	return nil
}

func (w *webdavStore) Get(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUA)
	if w.user != "" || w.password != "" {
		req.SetBasicAuth(w.user, w.password)
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("webdav get: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("webdav: remote file not found")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("webdav get: http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxOPMLBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxOPMLBytes {
		return nil, fmt.Errorf("webdav: file too large")
	}
	return b, nil
}

func (w *webdavStore) Ping(ctx context.Context) error {
	// Prefer PROPFIND / HEAD; some servers reject HEAD on collections — try GET range or PUT empty is bad.
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, w.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", defaultUA)
	if w.user != "" || w.password != "" {
		req.SetBasicAuth(w.user, w.password)
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("webdav ping: %w", err)
	}
	defer resp.Body.Close()
	// 404 means auth/path mostly OK but no file yet — treat as success for connection test.
	if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		return nil
	}
	// Some WebDAV hosts reject HEAD → try OPTIONS on parent.
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusForbidden {
		return w.pingOptions(ctx)
	}
	return fmt.Errorf("webdav ping: http %d", resp.StatusCode)
}

func (w *webdavStore) pingOptions(ctx context.Context) error {
	u, err := url.Parse(w.url)
	if err != nil {
		return err
	}
	// Parent directory
	u.Path = path.Dir(u.Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodOptions, u.String(), nil)
	if err != nil {
		return err
	}
	if w.user != "" || w.password != "" {
		req.SetBasicAuth(w.user, w.password)
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return fmt.Errorf("webdav options: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		// 401 clearly auth fail
		if resp.StatusCode == http.StatusUnauthorized {
			return fmt.Errorf("webdav: unauthorized")
		}
		return nil
	}
	return fmt.Errorf("webdav options: http %d", resp.StatusCode)
}

// —— S3 / R2 / MinIO ——

type s3Store struct {
	client *s3.Client
	bucket string
	key    string
}

func newS3(cfg settings.SyncConfig) (*s3Store, error) {
	endpoint, err := normalizeS3Endpoint(cfg)
	if err != nil {
		return nil, err
	}
	region := cfg.S3Region
	if region == "" {
		region = "auto"
	}
	key := cfg.ObjectKey
	if key == "" {
		key = settings.DefaultSyncObjectKey
	}

	awsCfg := aws.Config{
		Region: region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.S3AccessKey, cfg.S3SecretKey, "",
		),
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = cfg.S3ForcePathStyle
		// Newer AWS SDK defaults add checksum headers that many S3-compatible
		// backends (MinIO, R2, some gateways) reject with HTTP 400 Bad Request.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &s3Store{client: client, bucket: cfg.S3Bucket, key: key}, nil
}

func normalizeS3Endpoint(cfg settings.SyncConfig) (string, error) {
	ep := strings.TrimSpace(cfg.S3Endpoint)
	if ep == "" {
		return "", fmt.Errorf("s3 endpoint required")
	}
	if !strings.Contains(ep, "://") {
		if cfg.S3UseSSL {
			ep = "https://" + ep
		} else {
			ep = "http://" + ep
		}
	}
	u, err := url.Parse(ep)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid s3 endpoint")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("s3 endpoint must be http(s)")
	}
	return strings.TrimRight(ep, "/"), nil
}

func (s *s3Store) Put(ctx context.Context, body []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.key),
		Body:          bytes.NewReader(body),
		ContentLength: aws.Int64(int64(len(body))),
		ContentType:   aws.String("application/xml; charset=utf-8"),
	})
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}

func (s *s3Store) Get(ctx context.Context) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.key),
	})
	if err != nil {
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	defer out.Body.Close()
	b, err := io.ReadAll(io.LimitReader(out.Body, maxOPMLBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxOPMLBytes {
		return nil, fmt.Errorf("s3: object too large")
	}
	return b, nil
}

func (s *s3Store) Ping(ctx context.Context) error {
	// Prefer ListObjectsV2: validates endpoint + credentials + bucket without
	// requiring the OPML object to exist. HeadObject alone often 400s on
	// misconfigured path-style / checksum / empty keys for first-time setup.
	_, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		MaxKeys: aws.Int32(1),
		Prefix:  aws.String(s.key),
	})
	if err == nil {
		return nil
	}
	// Retry without prefix (some backends dislike prefix on empty bucket).
	_, err2 := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		MaxKeys: aws.Int32(1),
	})
	if err2 == nil {
		return nil
	}
	return fmt.Errorf("s3 ping: %w", simplifyS3Err(err2))
}

// simplifyS3Err shortens verbose AWS SDK error chains for UI toasts.
func simplifyS3Err(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "nosuchbucket"):
		return fmt.Errorf("bucket not found (check name / endpoint)")
	case strings.Contains(low, "invalidaccesskeyid"), strings.Contains(low, "invalidaccesskey"):
		return fmt.Errorf("invalid access key")
	case strings.Contains(low, "signaturedoesnotmatch"):
		return fmt.Errorf("signature mismatch (check secret key / region / path-style)")
	case strings.Contains(low, "accessdenied"), strings.Contains(low, "403"):
		return fmt.Errorf("access denied (check key permissions on bucket)")
	case strings.Contains(low, "moved permanently"), strings.Contains(low, "permanentredirect"):
		return fmt.Errorf("wrong endpoint or region (got permanent redirect)")
	case strings.Contains(low, "bad request"), strings.Contains(low, "statuscode: 400"):
		return fmt.Errorf("%w — try toggling path-style; R2 region=auto; MinIO usually path-style ON", err)
	default:
		return err
	}
}
