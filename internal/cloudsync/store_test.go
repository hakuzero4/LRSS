package cloudsync_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lrss/internal/cloudsync"
	"lrss/internal/settings"
)

func TestWebDAV_PutGet(t *testing.T) {
	var stored []byte
	var lastMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastMethod = r.Method
		user, pass, ok := r.BasicAuth()
		if !ok || user != "u" || pass != "p" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.Method {
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			stored = b
			w.WriteHeader(http.StatusCreated)
		case http.MethodGet:
			if stored == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write(stored)
		case http.MethodHead:
			if stored == nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(srv.Close)

	cfg := settings.SyncConfig{
		Enabled:        true,
		Provider:       settings.SyncProviderWebDAV,
		WebDAVURL:      srv.URL + "/dav",
		WebDAVUsername: "u",
		WebDAVPassword: "p",
		ObjectKey:      "subs.opml",
	}
	store, err := cloudsync.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping empty: %v", err)
	}
	body := []byte(`<?xml version="1.0"?><opml version="2.0"><head><title>t</title></head><body/></opml>`)
	if err := store.Put(ctx, body); err != nil {
		t.Fatal(err)
	}
	if lastMethod != http.MethodPut {
		t.Fatalf("method = %s", lastMethod)
	}
	got, err := store.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(body) {
		t.Fatalf("got %q", got)
	}
}

func TestWebDAV_TargetPath(t *testing.T) {
	// Ensure object key is under base path via round-trip server path check.
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	cfg := settings.SyncConfig{
		Enabled:   true,
		Provider:  settings.SyncProviderWebDAV,
		WebDAVURL: srv.URL + "/remote/dav",
		ObjectKey: "lrss-subscriptions.opml",
	}
	store, err := cloudsync.NewStore(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_ = store.Put(context.Background(), []byte("<opml/>"))
	if !strings.Contains(gotPath, "lrss-subscriptions.opml") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestNewStore_Disabled(t *testing.T) {
	_, err := cloudsync.NewStore(settings.DefaultSyncConfig())
	if err == nil {
		t.Fatal("expected error when disabled")
	}
}

func TestSyncConfig_ValidateS3(t *testing.T) {
	cfg := settings.SyncConfig{
		Enabled:     true,
		Provider:    settings.SyncProviderS3,
		S3Endpoint:  "https://account.r2.cloudflarestorage.com",
		S3Bucket:    "lrss",
		S3AccessKey: "ak",
		S3SecretKey: "sk",
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if !cfg.IsConfigured() {
		t.Fatal("should be configured")
	}
	m := cfg.Masked()
	if m.S3SecretKey == "sk" {
		t.Fatal("secret should be masked")
	}
}
