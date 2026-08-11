package fulltext_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lrss/internal/fulltext"
)

func TestValidateFetchURL_PublicAllowed(t *testing.T) {
	for _, raw := range []string{
		"https://example.com/post/1",
		"http://news.example.org/a",
		"https://cdn.example.com:443/x",
	} {
		if err := fulltext.ValidateFetchURL(raw); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
	}
}

func TestValidateFetchURL_Blocked(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/secret",
		"http://127.0.0.1:8080/x",
		"http://localhost/admin",
		"http://10.0.0.1/internal",
		"http://192.168.1.1/",
		"http://172.16.0.5/a",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"http://0.0.0.0/",
		"http://metadata.google.internal/",
		"http://100.64.0.1/",
	}
	for _, raw := range cases {
		err := fulltext.ValidateFetchURL(raw)
		if err == nil {
			t.Fatalf("expected block for %s", raw)
		}
		if !errors.Is(err, fulltext.ErrBlockedHost) && !strings.Contains(err.Error(), "blocked") {
			t.Fatalf("%s: err = %v", raw, err)
		}
	}
}

func TestHostBlocked_NamesAndIPs(t *testing.T) {
	if !fulltext.HostBlocked("127.0.0.1") || !fulltext.HostBlocked("localhost") {
		t.Fatal("loopback")
	}
	if !fulltext.HostBlocked("10.0.0.1") || !fulltext.HostBlocked("169.254.169.254") {
		t.Fatal("private/metadata")
	}
	if fulltext.HostBlocked("example.com") || fulltext.HostBlocked("news.bbc.co.uk") {
		t.Fatal("public hostnames should pass")
	}
}

func TestFetch_RejectsPrivateInitialURL(t *testing.T) {
	// No network: policy fails before Do.
	_, err := fulltext.Fetch(context.Background(), "http://127.0.0.1/x", fulltext.Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "blocked") && !errors.Is(err, fulltext.ErrBlockedHost) {
		t.Fatalf("err = %v", err)
	}
	_, err = fulltext.Fetch(context.Background(), "http://10.0.0.1/a", fulltext.Options{})
	if err == nil {
		t.Fatal("expected private error")
	}
	_, err = fulltext.Fetch(context.Background(), "http://169.254.169.254/latest", fulltext.Options{})
	if err == nil {
		t.Fatal("expected metadata error")
	}
}

func TestFetch_RejectsRedirectToPrivate(t *testing.T) {
	// Public-looking first hop (httptest is 127.0.0.1) — allow private hosts on
	// the initial URL only so we can exercise CheckRedirect against a private target.
	// Pattern: start on allowlisted loopback test server, redirect to 169.254.169.254.
	// We cannot use AllowPrivateHosts for the whole fetch; instead use a custom
	// client whose first hop is the test server and redirect target is private.
	//
	// Approach: serve on 127.0.0.1 with AllowPrivateHosts so initial validation is
	// skipped? That would also skip CheckRedirect policy.
	//
	// Better: host policy on redirect only when initial is "public". Use a
	// redirect from one httptest path is still loopback. So pure CheckRedirect
	// unit via ValidateFetchURL on redirect URL is already covered; for integration
	// we use a server that redirects to 169.254.169.254 and AllowPrivateHosts=false
	// would block initial 127.0.0.1.
	//
	// Solution: use an http.Client with a custom Transport that rewrites the first
	// request, OR test withHostPolicy via Fetch when initial URL is a public
	// hostname that our Transport maps… too heavy.
	//
	// Use httptest + temporarily allow only initial by validating redirect:
	// Call Fetch with pageURL that is blocked on redirect after we allow initial
	// via injecting a client that hits public URL first.
	//
	// Practical approach used here: start server on 127.0.0.1, but pass Options
	// with a client whose CheckRedirect is NOT set; Fetch wraps with host policy.
	// Initial URL is 127.0.0.1 → blocked before Do when AllowPrivateHosts=false.
	//
	// So for redirect test: allow private for initial only is wrong.
	// We'll use net/http/httptest and set AllowPrivateHosts: true is wrong for policy.
	//
	// Final approach: redirect test uses ValidateFetchURL for target (done above)
	// PLUS a live CheckRedirect test via internal-equivalent: Fetch with
	// HTTP client that starts at a non-IP host. Use "localhost" mapped…
	//
	// Simplest durable proof: use http.Client with Transport RoundTrip that
	// returns 302 Location: http://169.254.169.254/ for first call. Initial URL
	// is https://example.com/article (passes ValidateFetchURL). Client follows
	// redirect → host policy rejects.

	var hops int
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		hops++
		if hops == 1 {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header: http.Header{
					"Location": []string{"http://169.254.169.254/latest/meta-data/"},
				},
				Body:    http.NoBody,
				Request: req,
			}, nil
		}
		t.Fatalf("should not follow redirect to private host; got %s", req.URL)
		return nil, errors.New("unreachable")
	})
	client := &http.Client{Transport: transport}

	_, err := fulltext.Fetch(context.Background(), "https://example.com/post", fulltext.Options{
		HTTP: client,
	})
	if err == nil {
		t.Fatal("expected redirect-to-private error")
	}
	if hops != 1 {
		// CheckRedirect may count as attempt before hop 2; hop1 is the 302 response.
		// If client follows, hop would be 2 and we fail inside transport.
	}
	if !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "169.254") &&
		!errors.Is(err, fulltext.ErrBlockedHost) {
		// net/http wraps CheckRedirect errors
		if !strings.Contains(err.Error(), "redirect") && !strings.Contains(err.Error(), "fulltext") {
			t.Fatalf("err = %v", err)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestFetch_AllowPrivateHostsForTests(t *testing.T) {
	const page = `<!DOCTYPE html><html><head><title>T</title></head><body>
<article><h1>Ok</h1><p>Private host test body with enough text for readability extraction
to succeed when AllowPrivateHosts is set for httptest-based unit tests.</p>
<p>Second paragraph keeps density high enough for the parser heuristics.</p>
</article></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)

	res, err := fulltext.Fetch(context.Background(), srv.URL+"/p", fulltext.Options{
		HTTP:              srv.Client(),
		AllowPrivateHosts: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.HTML == "" && res.Text == "" {
		t.Fatal("empty result")
	}
}
