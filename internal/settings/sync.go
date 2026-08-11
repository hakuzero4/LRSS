package settings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Sync provider kinds. Subscriptions only (OPML) — not reading state.
const (
	SyncProviderNone   = "none"
	SyncProviderWebDAV = "webdav"
	SyncProviderS3     = "s3" // AWS S3, Cloudflare R2, MinIO, etc.
)

// Default remote object name for subscription OPML.
const DefaultSyncObjectKey = "lrss-subscriptions.opml"

// Key for sync config JSON blob.
const KeySyncConfig = "app.sync_config"

// SyncConfig is remote backup/sync for subscription list (OPML only).
type SyncConfig struct {
	Enabled  bool   `json:"enabled"`
	Provider string `json:"provider"` // none|webdav|s3

	// ObjectKey is the remote file/object name (default lrss-subscriptions.opml).
	ObjectKey string `json:"objectKey"`

	// WebDAV
	WebDAVURL      string `json:"webdavUrl"`
	WebDAVUsername string `json:"webdavUsername"`
	WebDAVPassword string `json:"webdavPassword"`
	// WebDAVPath is the full path under the server, e.g. /remote.php/dav/files/user/lrss.opml
	// If empty, ObjectKey is appended to WebDAVURL.
	WebDAVPath string `json:"webdavPath"`

	// S3-compatible (S3 / R2 / MinIO)
	S3Endpoint       string `json:"s3Endpoint"`
	S3Region         string `json:"s3Region"`
	S3Bucket         string `json:"s3Bucket"`
	S3AccessKey      string `json:"s3AccessKey"`
	S3SecretKey      string `json:"s3SecretKey"`
	S3ForcePathStyle bool   `json:"s3ForcePathStyle"` // MinIO usually true; R2 often false
	S3UseSSL         bool   `json:"s3UseSSL"`         // when endpoint has no scheme

	// LastPushAt / LastPullAt are RFC3339 timestamps (informational).
	LastPushAt string `json:"lastPushAt,omitempty"`
	LastPullAt string `json:"lastPullAt,omitempty"`
	LastError  string `json:"lastError,omitempty"`
}

// DefaultSyncConfig is disabled / no provider.
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		Enabled:          false,
		Provider:         SyncProviderNone,
		ObjectKey:        DefaultSyncObjectKey,
		S3Region:         "auto",
		S3ForcePathStyle: true,
		S3UseSSL:         true,
	}
}

// IsConfigured reports whether a remote can be used (enabled + valid provider fields).
func (c SyncConfig) IsConfigured() bool {
	if !c.Enabled {
		return false
	}
	switch c.Provider {
	case SyncProviderWebDAV:
		return strings.TrimSpace(c.WebDAVURL) != ""
	case SyncProviderS3:
		return strings.TrimSpace(c.S3Endpoint) != "" &&
			strings.TrimSpace(c.S3Bucket) != "" &&
			strings.TrimSpace(c.S3AccessKey) != "" &&
			strings.TrimSpace(c.S3SecretKey) != ""
	default:
		return false
	}
}

// Normalize trims and fills defaults.
func (c SyncConfig) Normalize() SyncConfig {
	c.Provider = strings.TrimSpace(strings.ToLower(c.Provider))
	switch c.Provider {
	case SyncProviderWebDAV, SyncProviderS3, SyncProviderNone:
	default:
		if c.Provider == "" {
			c.Provider = SyncProviderNone
		} else {
			c.Provider = SyncProviderNone
		}
	}
	c.ObjectKey = strings.TrimSpace(c.ObjectKey)
	if c.ObjectKey == "" {
		c.ObjectKey = DefaultSyncObjectKey
	}
	// Sanitize object key (no path traversal).
	c.ObjectKey = strings.ReplaceAll(c.ObjectKey, "\\", "/")
	for strings.Contains(c.ObjectKey, "..") {
		c.ObjectKey = strings.ReplaceAll(c.ObjectKey, "..", "")
	}
	c.ObjectKey = strings.TrimLeft(c.ObjectKey, "/")
	if c.ObjectKey == "" {
		c.ObjectKey = DefaultSyncObjectKey
	}

	c.WebDAVURL = strings.TrimRight(strings.TrimSpace(c.WebDAVURL), "/")
	c.WebDAVUsername = strings.TrimSpace(c.WebDAVUsername)
	c.WebDAVPassword = strings.TrimSpace(c.WebDAVPassword)
	c.WebDAVPath = strings.TrimSpace(c.WebDAVPath)

	c.S3Endpoint = strings.TrimRight(strings.TrimSpace(c.S3Endpoint), "/")
	c.S3Region = strings.TrimSpace(c.S3Region)
	if c.S3Region == "" {
		c.S3Region = "auto"
	}
	c.S3Bucket = strings.TrimSpace(c.S3Bucket)
	c.S3AccessKey = strings.TrimSpace(c.S3AccessKey)
	c.S3SecretKey = strings.TrimSpace(c.S3SecretKey)
	return c
}

// Validate checks config when enabling a provider.
func (c SyncConfig) Validate() error {
	c = c.Normalize()
	if !c.Enabled || c.Provider == SyncProviderNone {
		return nil
	}
	switch c.Provider {
	case SyncProviderWebDAV:
		if c.WebDAVURL == "" {
			return fmt.Errorf("webdav url is required")
		}
		u, err := url.Parse(c.WebDAVURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("invalid webdav url")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("webdav url must be http(s)")
		}
	case SyncProviderS3:
		if c.S3Endpoint == "" {
			return fmt.Errorf("s3 endpoint is required")
		}
		if c.S3Bucket == "" {
			return fmt.Errorf("s3 bucket is required")
		}
		if c.S3AccessKey == "" || c.S3SecretKey == "" {
			return fmt.Errorf("s3 access key and secret are required")
		}
		// Endpoint may be host-only or full URL.
		ep := c.S3Endpoint
		if !strings.Contains(ep, "://") {
			if c.S3UseSSL {
				ep = "https://" + ep
			} else {
				ep = "http://" + ep
			}
		}
		u, err := url.Parse(ep)
		if err != nil || u.Host == "" {
			return fmt.Errorf("invalid s3 endpoint")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("s3 endpoint must be http(s)")
		}
	default:
		return fmt.Errorf("unsupported sync provider %q", c.Provider)
	}
	return nil
}

// Masked redacts secrets for UI.
func (c SyncConfig) Masked() SyncConfig {
	out := c
	if out.WebDAVPassword != "" {
		out.WebDAVPassword = maskSecret(out.WebDAVPassword)
	}
	if out.S3SecretKey != "" {
		out.S3SecretKey = maskSecret(out.S3SecretKey)
	}
	if out.S3AccessKey != "" && len(out.S3AccessKey) > 6 {
		out.S3AccessKey = out.S3AccessKey[:3] + "***" + out.S3AccessKey[len(out.S3AccessKey)-2:]
	} else if out.S3AccessKey != "" {
		out.S3AccessKey = "***"
	}
	return out
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return "***"
	}
	return s[:3] + "***" + s[len(s)-2:]
}

// LoadSyncConfig loads sync prefs (defaults when unset).
func (s *Store) LoadSyncConfig(ctx context.Context) (SyncConfig, error) {
	cfg := DefaultSyncConfig()
	err := s.GetJSON(ctx, KeySyncConfig, &cfg)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg.Normalize(), nil
	}
	if err != nil {
		return DefaultSyncConfig(), err
	}
	return cfg.Normalize(), nil
}

// SaveSyncConfig validates and persists.
func (s *Store) SaveSyncConfig(ctx context.Context, cfg SyncConfig) error {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	return s.SetJSON(ctx, KeySyncConfig, cfg)
}
