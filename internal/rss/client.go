package rss

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"lrss/internal/httpx"
)

const (
	defaultTimeout   = 30 * time.Second
	defaultUserAgent = "LRSS/0.1 (+https://local)"
	maxBodyBytes     = 10 << 20 // 10 MiB
)

// FetchOptions controls HTTP fetch behaviour for a feed.
type FetchOptions struct {
	Timeout      time.Duration // default 30s
	UserAgent    string        // default LRSS/0.1 (+https://local)
	ETag         string
	LastModified string
}

// FetchResult is the outcome of a conditional GET + optional parse.
type FetchResult struct {
	StatusCode   int
	NotModified  bool
	ETag         string
	LastModified string
	Body         []byte
	Parsed       *gofeed.Feed
}

// Client fetches and parses RSS/Atom feeds.
type Client struct {
	HTTP *http.Client // optional; if nil, built per request from opts.Timeout
}

// Fetch performs a conditional GET on feedURL and parses the body on 200.
// Context cancellation is respected. On 304, NotModified is true and Parsed is nil.
func (c *Client) Fetch(ctx context.Context, feedURL string, opts FetchOptions) (*FetchResult, error) {
	if strings.TrimSpace(feedURL) == "" {
		return nil, fmt.Errorf("rss: empty feed URL")
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
		ua = defaultUserAgent
	}

	httpClient := c.httpClient(timeout, ua)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("rss: build request: %w", err)
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, */*")
	if et := strings.TrimSpace(opts.ETag); et != "" {
		req.Header.Set("If-None-Match", et)
	}
	if lm := strings.TrimSpace(opts.LastModified); lm != "" {
		req.Header.Set("If-Modified-Since", lm)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rss: fetch: %w", err)
	}
	defer resp.Body.Close()

	result := &FetchResult{
		StatusCode:   resp.StatusCode,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	if resp.StatusCode == http.StatusNotModified {
		result.NotModified = true
		return result, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return result, fmt.Errorf("rss: unexpected status %d for %s", resp.StatusCode, feedURL)
	}

	limited := io.LimitReader(resp.Body, maxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return result, fmt.Errorf("rss: read body: %w", err)
	}
	if len(body) > maxBodyBytes {
		return result, fmt.Errorf("rss: body exceeds %d bytes", maxBodyBytes)
	}
	result.Body = body

	parser := gofeed.NewParser()
	feed, err := parser.ParseString(string(body))
	if err != nil {
		return result, fmt.Errorf("rss: parse: %w", err)
	}
	result.Parsed = feed
	return result, nil
}

// FetchAndMap fetches a feed and maps it to ParsedFeed.
func (c *Client) FetchAndMap(ctx context.Context, feedURL string, opts FetchOptions) (*FetchResult, *ParsedFeed, error) {
	res, err := c.Fetch(ctx, feedURL, opts)
	if err != nil {
		return res, nil, err
	}
	if res.NotModified || res.Parsed == nil {
		return res, nil, nil
	}
	return res, ToParsedFeed(res.Parsed, feedURL), nil
}

func (c *Client) httpClient(timeout time.Duration, ua string) *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}
	return httpx.Std(httpx.Options{
		Timeout:   timeout,
		UserAgent: ua,
	})
}
