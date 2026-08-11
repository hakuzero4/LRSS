package fulltext

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// ErrBlockedHost is returned when a fetch URL targets a non-public host
// (loopback, private, link-local, or known cloud metadata).
var ErrBlockedHost = fmt.Errorf("fulltext: blocked host (private, loopback, or metadata)")

// ValidateFetchURL checks that raw is an absolute http(s) URL whose host is
// not loopback, link-local, private (RFC1918 / ULA), CGNAT, or common cloud
// metadata endpoints. Pure — no network I/O.
func ValidateFetchURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("fulltext: empty url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("fulltext: invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("fulltext: invalid url")
	}
	if u.Host == "" {
		return fmt.Errorf("fulltext: invalid url")
	}
	if HostBlocked(u.Hostname()) {
		return fmt.Errorf("%w: %s", ErrBlockedHost, u.Hostname())
	}
	return nil
}

// HostBlocked reports whether hostname (no port) must not be fetched.
func HostBlocked(hostname string) bool {
	h := strings.ToLower(strings.TrimSpace(hostname))
	if h == "" {
		return true
	}
	// Bracketed IPv6 without port already stripped by url.Hostname().
	h = strings.TrimPrefix(h, "[")
	h = strings.TrimSuffix(h, "]")

	switch h {
	case "localhost", "metadata", "metadata.google.internal":
		return true
	}
	if strings.HasSuffix(h, ".localhost") || strings.HasSuffix(h, ".local") {
		return true
	}

	ip := net.ParseIP(h)
	if ip == nil {
		// Non-IP hostnames are allowed at this layer (DNS may still resolve
		// privately; redirect re-check catches literal private targets).
		return false
	}
	return IPBlocked(ip)
}

// IPBlocked reports whether ip is unsuitable for outbound full-text fetch.
func IPBlocked(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	// CGNAT / shared address space (RFC 6598): 100.64.0.0/10
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
	}
	return false
}

// withHostPolicyRedirect wraps client so every redirect target is re-validated.
// Does not mutate the original client.
func withHostPolicyRedirect(client *http.Client) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	clone := *client
	prev := clone.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req != nil && req.URL != nil {
			if err := ValidateFetchURL(req.URL.String()); err != nil {
				return err
			}
		}
		if prev != nil {
			return prev(req, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("fulltext: stopped after 10 redirects")
		}
		return nil
	}
	return &clone
}
