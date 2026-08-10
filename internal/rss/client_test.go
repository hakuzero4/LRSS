package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetch_200AndParse(t *testing.T) {
	const body = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>HTTPtest</title>
    <link>https://example.com/</link>
    <item>
      <title>One</title>
      <link>https://example.com/1</link>
      <guid>g1</guid>
    </item>
  </channel>
</rss>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		w.Header().Set("ETag", `"abc123"`)
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	c := &Client{}
	res, err := c.Fetch(context.Background(), srv.URL, FetchOptions{
		Timeout:   5 * time.Second,
		UserAgent: "LRSS-test/1",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("StatusCode = %d", res.StatusCode)
	}
	if res.NotModified {
		t.Fatal("NotModified unexpected")
	}
	if res.ETag != `"abc123"` {
		t.Errorf("ETag = %q", res.ETag)
	}
	if res.LastModified == "" {
		t.Error("LastModified empty")
	}
	if res.Parsed == nil || res.Parsed.Title != "HTTPtest" {
		t.Fatalf("Parsed = %+v", res.Parsed)
	}
	if len(res.Body) == 0 {
		t.Error("Body empty")
	}

	mapped := ToParsedFeed(res.Parsed, srv.URL)
	if mapped.Title != "HTTPtest" || len(mapped.Items) != 1 {
		t.Fatalf("mapped = %+v", mapped)
	}
	if mapped.Items[0].GUID != "g1" {
		t.Errorf("GUID = %q", mapped.Items[0].GUID)
	}
}

func TestFetch_304NotModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `"etag-1"` {
			t.Errorf("If-None-Match = %q", r.Header.Get("If-None-Match"))
		}
		if r.Header.Get("If-Modified-Since") != "Mon, 02 Jan 2006 15:04:05 GMT" {
			t.Errorf("If-Modified-Since = %q", r.Header.Get("If-Modified-Since"))
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	c := &Client{}
	res, err := c.Fetch(context.Background(), srv.URL, FetchOptions{
		ETag:         `"etag-1"`,
		LastModified: "Mon, 02 Jan 2006 15:04:05 GMT",
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !res.NotModified {
		t.Fatal("expected NotModified")
	}
	if res.StatusCode != 304 {
		t.Fatalf("StatusCode = %d", res.StatusCode)
	}
	if res.Parsed != nil {
		t.Fatal("Parsed should be nil on 304")
	}
}

func TestFetch_ContextCancel(t *testing.T) {
	started := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	c := &Client{}
	_, err := c.Fetch(ctx, srv.URL, FetchOptions{Timeout: 5 * time.Second})
	if err == nil {
		t.Fatal("expected error on cancel")
	}
}

func TestFetch_EmptyURL(t *testing.T) {
	c := &Client{}
	_, err := c.Fetch(context.Background(), "", FetchOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{}
	res, err := c.Fetch(context.Background(), srv.URL, FetchOptions{})
	if err == nil {
		t.Fatal("expected error")
	}
	if res == nil || res.StatusCode != 404 {
		t.Fatalf("res = %+v", res)
	}
}

func TestFetchAndMap_304(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	c := &Client{}
	res, mapped, err := c.FetchAndMap(context.Background(), srv.URL, FetchOptions{ETag: "x"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !res.NotModified || mapped != nil {
		t.Fatalf("res=%+v mapped=%+v", res, mapped)
	}
}

func TestFetch_DefaultUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	c := &Client{}
	_, err := c.Fetch(context.Background(), srv.URL, FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(gotUA, "LRSS/") {
		t.Fatalf("User-Agent = %q", gotUA)
	}
}
