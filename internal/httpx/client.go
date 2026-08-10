// Package httpx provides shared outbound HTTP clients backed by enetx/surf.
package httpx

import (
	"net/http"
	"time"

	"github.com/enetx/surf"
)

// Options configures Std clients.
type Options struct {
	Timeout   time.Duration // default 30s
	UserAgent string        // optional; empty leaves surf default / request-level UA
}

const defaultTimeout = 30 * time.Second

// Std returns a *http.Client backed by surf (via Client.Std()).
// Callers can use it with net/http APIs (Do, Get, etc.) while using surf's transport stack.
func Std(opts Options) *http.Client {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	b := surf.NewClient().Builder().Timeout(timeout)
	if ua := opts.UserAgent; ua != "" {
		b = b.UserAgent(ua)
	}
	return b.Build().Unwrap().Std()
}
