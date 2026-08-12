package web

import (
	"net/http"
	"strings"
)

// extractToken returns the shared secret from Authorization Bearer or ?token=.
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); h != "" {
		const p = "Bearer "
		if strings.HasPrefix(h, p) {
			return strings.TrimSpace(h[len(p):])
		}
		// Also accept raw token in Authorization for convenience.
		if !strings.Contains(h, " ") {
			return strings.TrimSpace(h)
		}
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// withAPIAuth protects only /api/* when required token is non-empty.
// Static SPA assets must stay public: browsers never send ?token= on JS/CSS
// subresource requests, so wrapping the whole mux breaks the page.
func withAPIAuth(required string, h http.Handler) http.Handler {
	required = strings.TrimSpace(required)
	if required == "" {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			got := extractToken(r)
			if got == "" || got != required {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error": "unauthorized",
				})
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}
