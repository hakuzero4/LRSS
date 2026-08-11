package fulltext_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lrss/internal/fulltext"
)

func TestFetch_ExtractsArticle(t *testing.T) {
	const page = `<!DOCTYPE html><html><head><title>Hello</title></head>
<body>
  <nav>skip me</nav>
  <article>
    <h1>Hello World</h1>
    <p>This is the full article body with enough text to pass readability heuristics.
    More sentences help the extractor pick the main content area of the document.
    We keep writing so the density score is high enough for a confident parse.</p>
    <p>Second paragraph continues the story for extraction quality.</p>
  </article>
  <footer>ads and noise</footer>
</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)

	res, err := fulltext.Fetch(context.Background(), srv.URL+"/post/1", fulltext.Options{
		HTTP:              srv.Client(),
		AllowPrivateHosts: true, // httptest is 127.0.0.1
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.HTML, "full article") && !strings.Contains(res.Text, "full article") {
		t.Fatalf("missing body content: html=%q text=%q", res.HTML, res.Text)
	}
	if res.HTML == "" {
		t.Fatal("empty html")
	}
}

func TestFetch_InvalidURL(t *testing.T) {
	_, err := fulltext.Fetch(context.Background(), "javascript:alert(1)", fulltext.Options{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	_, err := fulltext.Fetch(context.Background(), srv.URL, fulltext.Options{
		HTTP:              srv.Client(),
		AllowPrivateHosts: true,
	})
	if err == nil {
		t.Fatal("expected http error")
	}
}
