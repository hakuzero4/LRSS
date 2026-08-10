// Package favicon discovers a site's icon URL from site/feed URLs.
package favicon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"lrss/internal/httpx"
)

const (
	defaultTimeout = 12 * time.Second
	maxHTMLBytes   = 512 << 10 // 512 KiB homepage scan
)

// link rel icon/shortcut icon/apple-touch-icon
var reLinkIcon = regexp.MustCompile(`(?i)<link[^>]+rel=["']([^"']*icon[^"']*)["'][^>]*>`)
var reHref = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)

// Resolve finds an absolute favicon URL for the given site or feed URL.
// Order: HTML <link rel=icon> on the site origin, then common static paths.
// Returns empty string when nothing usable is found (not an error).
func Resolve(ctx context.Context, siteURL, feedURL string) string {
	if ctx == nil {
		ctx = context.Background()
	}
	origin := originOf(siteURL)
	if origin == "" {
		origin = originOf(feedURL)
	}
	if origin == "" {
		return ""
	}

	client := httpx.Std(httpx.Options{
		Timeout:   defaultTimeout,
		UserAgent: "LRSS/0.1 (+favicon)",
	})

	if icon := fromHTML(ctx, client, origin); icon != "" {
		return icon
	}
	for _, path := range []string{"/favicon.ico", "/favicon.png", "/apple-touch-icon.png", "/apple-touch-icon-precomposed.png"} {
		u := origin + path
		if probeImage(ctx, client, u) {
			return u
		}
	}
	return ""
}

func originOf(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return strings.TrimRight(u.Scheme+"://"+u.Host, "/")
}

func fromHTML(ctx context.Context, client *http.Client, origin string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.8")
	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, maxHTMLBytes))
	if err != nil {
		return ""
	}
	html := string(body)
	// Prefer explicit icon links (first match wins: browsers often put best first).
	for _, m := range reLinkIcon.FindAllString(html, -1) {
		hm := reHref.FindStringSubmatch(m)
		if len(hm) < 2 {
			continue
		}
		abs := absolutize(origin, hm[1])
		if abs == "" {
			continue
		}
		if probeImage(ctx, client, abs) {
			return abs
		}
		// Some CDNs block HEAD/GET probe; still return candidate if it looks like an image URL.
		if looksLikeImageURL(abs) {
			return abs
		}
	}
	return ""
}

func probeImage(ctx context.Context, client *http.Client, iconURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconURL, nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "image/*,*/*;q=0.1")
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 2048))
		return false
	}
	// Read a few bytes to ensure non-empty body.
	buf := make([]byte, 16)
	n, _ := io.ReadFull(res.Body, buf)
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	if n < 4 {
		return false
	}
	ct := strings.ToLower(res.Header.Get("Content-Type"))
	if strings.Contains(ct, "image/") || strings.Contains(ct, "icon") {
		return true
	}
	// favicon.ico often served as octet-stream
	if looksLikeImageURL(iconURL) || isICO(buf[:n]) || isPNG(buf[:n]) || isGIF(buf[:n]) || isJPEG(buf[:n]) {
		return true
	}
	// Reject obvious HTML error pages
	if strings.Contains(ct, "text/html") {
		return false
	}
	return looksLikeImageURL(iconURL)
}

func looksLikeImageURL(u string) bool {
	path := strings.ToLower(u)
	for _, ext := range []string{".ico", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"} {
		if strings.Contains(path, ext) {
			return true
		}
	}
	return false
}

func isICO(b []byte) bool {
	return len(b) >= 4 && b[0] == 0 && b[1] == 0 && b[2] == 1 && b[3] == 0
}
func isPNG(b []byte) bool {
	return len(b) >= 4 && b[0] == 0x89 && b[1] == 'P' && b[2] == 'N' && b[3] == 'G'
}
func isGIF(b []byte) bool {
	return len(b) >= 3 && b[0] == 'G' && b[1] == 'I' && b[2] == 'F'
}
func isJPEG(b []byte) bool {
	return len(b) >= 2 && b[0] == 0xff && b[1] == 0xd8
}

func absolutize(origin, href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "data:") {
		return ""
	}
	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}
	base, err := url.Parse(origin + "/")
	if err != nil {
		return ""
	}
	abs := base.ResolveReference(ref)
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return ""
	}
	return abs.String()
}

// ResolveOrEmpty is like Resolve but never panics; for logging wrappers.
func ResolveOrEmpty(ctx context.Context, siteURL, feedURL string) string {
	defer func() { _ = recover() }()
	return Resolve(ctx, siteURL, feedURL)
}

// MustAbs is a small helper used in tests.
func MustAbs(origin, href string) string {
	s := absolutize(origin, href)
	if s == "" {
		panic(fmt.Sprintf("bad abs %q %q", origin, href))
	}
	return s
}
