package favicon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAbsolutize(t *testing.T) {
	cases := []struct {
		origin, href, want string
	}{
		{"https://example.com", "/favicon.ico", "https://example.com/favicon.ico"},
		{"https://example.com", "https://cdn.example.com/i.png", "https://cdn.example.com/i.png"},
		{"https://example.com/blog", "../icon.png", "https://example.com/icon.png"},
		{"https://example.com", "data:image/png;base64,xx", ""},
	}
	for _, c := range cases {
		got := absolutize(c.origin, c.href)
		if got != c.want {
			t.Errorf("absolutize(%q,%q)=%q want %q", c.origin, c.href, got, c.want)
		}
	}
}

func TestResolve_FromHTMLAndIco(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head>
<link rel="icon" href="/static/icon.png">
</head><body>ok</body></html>`))
	})
	// Valid 1x1 PNG
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xfe, 0xd4, 0xef, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	mux.HandleFunc("/static/icon.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Resolve uses httpx/surf which hits real network; for unit test we only
	// exercise HTML regex + absolutize against static fixtures via fromHTML-like path.
	// Direct check of link parsing:
	html := `<link rel="shortcut icon" href="/static/icon.png">`
	if !reLinkIcon.MatchString(html) {
		t.Fatal("regex should match shortcut icon")
	}
	m := reHref.FindStringSubmatch(reLinkIcon.FindString(html))
	if len(m) < 2 || !strings.Contains(m[1], "icon.png") {
		t.Fatalf("href parse: %v", m)
	}
	abs := absolutize(srv.URL, m[1])
	if abs != srv.URL+"/static/icon.png" {
		t.Fatalf("abs = %s", abs)
	}

	// probe via stdlib client against httptest
	client := srv.Client()
	if !probeImage(context.Background(), client, abs) {
		t.Fatal("probeImage should accept png")
	}
}

func TestOriginOf(t *testing.T) {
	if originOf("https://a.b.com/path?q=1") != "https://a.b.com" {
		t.Fatal(originOf("https://a.b.com/path?q=1"))
	}
	if originOf("not-a-url") != "" {
		t.Fatal("expected empty")
	}
}
