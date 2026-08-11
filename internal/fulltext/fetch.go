// Package fulltext fetches an article page and extracts readable HTML body.
// Outbound HTTP uses lrss/internal/httpx (enetx/surf fingerprint client).
package fulltext

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"lrss/internal/httpx"

	readability "codeberg.org/readeck/go-readability/v2"
)

const (
	defaultTimeout = 45 * time.Second
	defaultUA      = "LRSS/0.1 (+https://local; fulltext)"
	maxBodyBytes   = 8 << 20 // 8 MiB
)

// Result is extracted full-page content for storage / display.
type Result struct {
	HTML  string
	Text  string
	Title string
}

// Options controls Fetch.
type Options struct {
	Timeout   time.Duration
	UserAgent string
	// HTTP injects a client (tests); default is httpx.Std (surf).
	HTTP *http.Client
	// AllowPrivateHosts skips loopback/private/metadata host checks.
	// Intended for unit tests that serve over httptest (127.0.0.1).
	AllowPrivateHosts bool
}

// Fetch downloads pageURL with the fingerprint HTTP client and extracts
// the main article body via Readability.
func Fetch(ctx context.Context, pageURL string, opts Options) (Result, error) {
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return Result{}, fmt.Errorf("fulltext: empty url")
	}
	u, err := url.Parse(pageURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Result{}, fmt.Errorf("fulltext: invalid url")
	}
	if !opts.AllowPrivateHosts {
		if err := ValidateFetchURL(pageURL); err != nil {
			return Result{}, err
		}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = defaultUA
	}

	client := opts.HTTP
	if client == nil {
		client = httpx.Std(httpx.Options{
			Timeout:   timeout,
			UserAgent: ua,
		})
	}
	if !opts.AllowPrivateHosts {
		client = withHostPolicyRedirect(client)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("fulltext: request: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("fulltext: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("fulltext: http %d", resp.StatusCode)
	}

	// Prefer final URL after redirects for relative link resolution.
	finalURL := pageURL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	if !opts.AllowPrivateHosts {
		if err := ValidateFetchURL(finalURL); err != nil {
			return Result{}, err
		}
	}
	pageParsed, err := url.Parse(finalURL)
	if err != nil {
		pageParsed = u
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return Result{}, fmt.Errorf("fulltext: read body: %w", err)
	}
	if len(body) > maxBodyBytes {
		return Result{}, fmt.Errorf("fulltext: body too large")
	}

	art, err := readability.FromReader(strings.NewReader(string(body)), pageParsed)
	if err != nil {
		return Result{}, fmt.Errorf("fulltext: extract: %w", err)
	}

	var htmlBuf, textBuf strings.Builder
	if err := art.RenderHTML(&htmlBuf); err != nil {
		return Result{}, fmt.Errorf("fulltext: render html: %w", err)
	}
	if err := art.RenderText(&textBuf); err != nil {
		// Text is best-effort; HTML is required.
		textBuf.Reset()
	}

	html := strings.TrimSpace(htmlBuf.String())
	text := strings.TrimSpace(textBuf.String())
	if html == "" && text == "" {
		return Result{}, fmt.Errorf("fulltext: no readable content")
	}
	// Some pages yield text-only; wrap as simple HTML for the reader.
	if html == "" && text != "" {
		html = "<p>" + htmlEscape(text) + "</p>"
	}

	return Result{
		HTML:  html,
		Text:  text,
		Title: strings.TrimSpace(art.Title()),
	}, nil
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
	)
	return r.Replace(s)
}
