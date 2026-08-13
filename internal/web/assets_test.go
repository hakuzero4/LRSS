package web

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSpaFSWithIndex(t *testing.T) {
	t.Parallel()
	if spaFSWithIndex(nil) != nil {
		t.Fatal("nil FS should yield nil")
	}
	if spaFSWithIndex(fstest.MapFS{}) != nil {
		t.Fatal("empty FS should yield nil")
	}
	if spaFSWithIndex(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("root")},
	}) == nil {
		t.Fatal("root index should win")
	}

	nested := spaFSWithIndex(fstest.MapFS{
		"frontend/dist/index.html": &fstest.MapFile{Data: []byte("nested")},
	})
	if nested == nil {
		t.Fatal("frontend/dist index should be found")
	}
	got, err := fs.ReadFile(nested, "index.html")
	if err != nil || string(got) != "nested" {
		t.Fatalf("nested index = %q err=%v", got, err)
	}
}

func TestResolveSPA_EmbedWins(t *testing.T) {
	prev := testLookup
	testLookup = &assetLookup{diskDirs: []string{t.TempDir()}, viteURL: ""}
	t.Cleanup(func() { testLookup = prev })

	src := resolveSPA(fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><head></head></html>")},
	})
	if src.kind != "embed" {
		t.Fatalf("kind = %s (%s)", src.kind, src.desc)
	}
}

func TestResolveSPA_DiskFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html><head></head>disk</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	prev := testLookup
	testLookup = &assetLookup{diskDirs: []string{dir}, viteURL: ""}
	t.Cleanup(func() { testLookup = prev })

	src := resolveSPA(fstest.MapFS{})
	if src.kind != "disk" {
		t.Fatalf("kind = %s (%s)", src.kind, src.desc)
	}
	if !fileExists(src.root, "index.html") {
		t.Fatal("disk root missing index.html")
	}
}

func TestResolveSPA_NoneWhenIsolated(t *testing.T) {
	prev := testLookup
	testLookup = &assetLookup{}
	t.Cleanup(func() { testLookup = prev })

	src := resolveSPA(fstest.MapFS{})
	if src.kind != "none" {
		t.Fatalf("kind = %s (%s)", src.kind, src.desc)
	}
}

func TestInjectWebBootstrap(t *testing.T) {
	t.Parallel()
	in := "<html><head><title>x</title></head></html>"
	out := injectWebBootstrap(in)
	if !strings.Contains(out, "__LRSS_WEB__") {
		t.Fatalf("missing inject: %s", out)
	}
	if injectWebBootstrap(out) != out {
		t.Fatal("should be idempotent")
	}
	if !strings.Contains(injectWebBootstrap("no-head"), "__LRSS_WEB__") {
		t.Fatal("prefix inject")
	}
}

func TestSpaHandler_DiskIndex(t *testing.T) {
	dir := t.TempDir()
	html := "<!doctype html><html><head></head><body>disk</body></html>"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := spaHandler(spaSource{kind: "disk", root: os.DirFS(dir), desc: dir})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 200 {
		t.Fatalf("GET / = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "__LRSS_WEB__") || !strings.Contains(body, "disk") {
		t.Fatalf("index body = %s", body)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if rec2.Code != 200 || !strings.Contains(rec2.Body.String(), "console.log") {
		t.Fatalf("GET /app.js = %d %s", rec2.Code, rec2.Body.String())
	}
}

func TestSpaHandler_ViteProxy(t *testing.T) {
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/", "/index.html":
			_, _ = io.WriteString(w, "<!doctype html><html><head></head><body>vite</body></html>")
		case "/src/main.ts":
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = io.WriteString(w, "export default 1")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(vite.Close)

	u, err := url.Parse(vite.URL)
	if err != nil {
		t.Fatal(err)
	}
	h := spaHandler(spaSource{kind: "vite", vite: u, desc: "vite test"})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 200 {
		t.Fatalf("GET / = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "__LRSS_WEB__") || !strings.Contains(rec.Body.String(), "vite") {
		t.Fatalf("vite index = %s", rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/src/main.ts", nil))
	if rec2.Code != 200 || !strings.Contains(rec2.Body.String(), "export default 1") {
		t.Fatalf("proxy = %d %s", rec2.Code, rec2.Body.String())
	}
}

func TestSpaHandler_MissingIndex(t *testing.T) {
	prev := testLookup
	testLookup = &assetLookup{}
	t.Cleanup(func() { testLookup = prev })

	h := spaHandler(spaSource{kind: "none", desc: "none"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "index missing") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestResolveSPA_ViteWhenReachable(t *testing.T) {
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "<html><head></head></html>")
	}))
	t.Cleanup(vite.Close)

	prev := testLookup
	testLookup = &assetLookup{viteURL: vite.URL}
	t.Cleanup(func() { testLookup = prev })

	src := resolveSPA(fstest.MapFS{})
	if src.kind != "vite" {
		t.Fatalf("kind = %s (%s)", src.kind, src.desc)
	}
}
