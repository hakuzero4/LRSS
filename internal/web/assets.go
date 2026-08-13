package web

import (
	"context"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FRONTEND_DEVSERVER_URL is set by `wails3 task dev` (Vite on 127.0.0.1).
// LRSS_WEB_DIST overrides the on-disk SPA directory (must contain index.html).
const (
	envFrontendDevServer = "FRONTEND_DEVSERVER_URL"
	envWebDist           = "LRSS_WEB_DIST"
)

const webBootstrap = `<script>window.__LRSS_WEB__=true;</script>`

// testLookup, when non-nil, replaces env/disk/Vite discovery (embed still wins).
// Used by tests so CI without frontend/dist cannot accidentally serve the repo.
var testLookup *assetLookup

type assetLookup struct {
	diskDirs []string
	viteURL  string
}

type spaSource struct {
	kind string // embed | disk | vite | none
	root fs.FS
	vite *url.URL
	desc string
}

var viteHTTP = &http.Client{Timeout: 4 * time.Second}

// resolveSPA picks a UI source. Production binaries embed frontend/dist.
// Dev builds (`//go:build !production`) ship an empty embed.FS, so Web access
// must proxy Vite or read frontend/dist from disk — otherwise the browser
// sees "index missing".
func resolveSPA(embedded fs.FS) spaSource {
	if root := spaFSWithIndex(embedded); root != nil {
		return spaSource{kind: "embed", root: root, desc: "embedded frontend/dist"}
	}
	if u := viteDevURL(); u != nil && viteReachable(u) {
		return spaSource{kind: "vite", vite: u, desc: "vite " + u.String()}
	}
	if root, dir := diskSPA(); root != nil {
		return spaSource{kind: "disk", root: root, desc: dir}
	}
	return spaSource{kind: "none", desc: "no index.html (build frontend/dist or use a production binary)"}
}

func spaFSWithIndex(assets fs.FS) fs.FS {
	if assets == nil {
		return nil
	}
	if fileExists(assets, "index.html") {
		return assets
	}
	for _, prefix := range []string{"frontend/dist", "dist"} {
		sub, err := fs.Sub(assets, prefix)
		if err != nil {
			continue
		}
		if fileExists(sub, "index.html") {
			return sub
		}
	}
	return nil
}

func fileExists(fsys fs.FS, name string) bool {
	st, err := fs.Stat(fsys, name)
	return err == nil && !st.IsDir()
}

func viteDevURL() *url.URL {
	raw := strings.TrimSpace(os.Getenv(envFrontendDevServer))
	if testLookup != nil {
		raw = strings.TrimSpace(testLookup.viteURL)
	}
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil
	}
	return u
}

func viteReachable(u *url.URL) bool {
	endpoint := strings.TrimRight(u.String(), "/") + "/index.html"
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	resp, err := viteHTTP.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode > 0 && resp.StatusCode < 500
}

func diskSPA() (fs.FS, string) {
	for _, dir := range diskCandidates() {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		abs := dir
		if a, err := filepath.Abs(dir); err == nil {
			abs = a
		}
		info, err := os.Stat(filepath.Join(abs, "index.html"))
		if err != nil || info.IsDir() {
			continue
		}
		return os.DirFS(abs), abs
	}
	return nil, ""
}

func diskCandidates() []string {
	if testLookup != nil {
		return testLookup.diskDirs
	}
	var out []string
	if d := strings.TrimSpace(os.Getenv(envWebDist)); d != "" {
		out = append(out, d)
	}
	out = append(out, filepath.Join("frontend", "dist"))
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		out = append(out,
			filepath.Join(exeDir, "frontend", "dist"),
			filepath.Join(exeDir, "dist"),
			filepath.Join(exeDir, "..", "frontend", "dist"),
		)
	}
	return out
}

func spaHandler(src spaSource) http.Handler {
	var files http.Handler
	if src.root != nil {
		files = http.FileServer(http.FS(src.root))
	}
	var proxy http.Handler
	if src.vite != nil {
		proxy = newViteProxy(src.vite)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api") {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || path == "index.html" {
			serveIndex(w, src)
			return
		}
		if proxy != nil {
			proxy.ServeHTTP(w, r)
			return
		}
		if src.root != nil {
			if f, err := src.root.Open(path); err == nil {
				_ = f.Close()
				files.ServeHTTP(w, r)
				return
			}
			serveWebIndex(w, src.root)
			return
		}
		serveMissingIndex(w)
	})
}

func serveIndex(w http.ResponseWriter, src spaSource) {
	if src.vite != nil && serveViteIndex(w, src.vite) {
		return
	}
	if src.root != nil {
		serveWebIndex(w, src.root)
		return
	}
	if root, _ := diskSPA(); root != nil {
		serveWebIndex(w, root)
		return
	}
	serveMissingIndex(w)
}

func serveWebIndex(w http.ResponseWriter, root fs.FS) {
	raw, err := fs.ReadFile(root, "index.html")
	if err != nil {
		serveMissingIndex(w)
		return
	}
	html := injectWebBootstrap(string(raw))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(html))
}

func serveViteIndex(w http.ResponseWriter, target *url.URL) bool {
	endpoint := strings.TrimRight(target.String(), "/") + "/index.html"
	resp, err := viteHTTP.Get(endpoint)
	if err != nil {
		log.Printf("web access: vite index: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return false
	}
	html := injectWebBootstrap(string(raw))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(html))
	return true
}

func injectWebBootstrap(html string) string {
	// Only skip when the flag is actually assigned. A mere mention
	// (e.g. the desktop Mica guard checking window.__LRSS_WEB__) must
	// not block injection — otherwise the browser loads Wails bindings
	// and library bootstrap fails with a blank "Error".
	if hasWebBootstrap(html) {
		return html
	}
	if strings.Contains(html, "<head>") {
		return strings.Replace(html, "<head>", "<head>"+webBootstrap, 1)
	}
	return webBootstrap + html
}

func hasWebBootstrap(html string) bool {
	return strings.Contains(html, "__LRSS_WEB__=true") ||
		strings.Contains(html, "__LRSS_WEB__ = true")
}

func newViteProxy(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	orig := proxy.Director
	proxy.Director = func(req *http.Request) {
		orig(req)
		req.Host = target.Host
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("web access: vite proxy %s: %v", r.URL.Path, err)
		http.Error(w, "vite proxy failed", http.StatusBadGateway)
	}
	return proxy
}

func serveMissingIndex(w http.ResponseWriter) {
	const page = `<!doctype html>
<meta charset="utf-8">
<title>LRSS — index missing</title>
<body style="font:16px/1.5 system-ui,sans-serif;max-width:40rem;margin:3rem auto;padding:0 1rem">
<h1>Web UI not available</h1>
<p>LRSS could not find <code>index.html</code> for Web access (<code>index missing</code>).</p>
<p>Development builds (<code>wails3 task dev</code>) do not embed the SPA.
The desktop window uses Vite; the browser server needs either a running Vite
dev server or a built frontend.</p>
<pre>cd frontend && npm run build</pre>
<p>Then toggle Web access off and on. Release binaries already embed the UI.</p>
</body>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(page))
}
