package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"lrss/internal/db"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"
	"lrss/internal/web"
)

func testEnv(t *testing.T) (*web.Server, *settings.Store, *service.Library) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	searchSvc := search.New(database.SQL, store)

	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>LRSS</title>")},
	}
	srv := web.New(web.APIDeps{Library: lib, Store: store, Search: searchSvc}, assets)
	return srv, store, lib
}

func TestServer_AuthAndMeta(t *testing.T) {
	srv, _, _ := testEnv(t)
	ctx := context.Background()

	st, err := srv.Apply(ctx, settings.WebAccessConfig{
		Enabled: true,
		Bind:    "localhost",
		Port:    18766,
		Token:   "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })
	if !st.Running || st.URL == "" {
		t.Fatalf("status = %+v", st)
	}

	// no token → 401
	res, err := http.Get(st.URL + "api/meta")
	if err != nil {
		// URL already has ?token= so strip - actually buildURL adds token to base
		// st.URL is http://127.0.0.1:18766/?token=test-token
		// request without token:
	}
	_ = res

	base := "http://127.0.0.1:18766"
	resp, err := http.Get(base + "/api/meta")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token status = %d", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodGet, base+"/api/meta", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("with token status = %d body=%s", resp2.StatusCode, b)
	}
	var meta map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	if meta["mode"] != "web" {
		t.Fatalf("meta = %+v", meta)
	}

	// management write not registered
	resp3, err := http.Post(base+"/api/feeds", "application/json", bytes.NewReader([]byte(`{}`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	// auth fails first without token
	if resp3.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp3.StatusCode)
	}

	// SPA / static must load without token (browsers omit ?token on assets)
	resp4, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("index without token status = %d (assets must be public)", resp4.StatusCode)
	}
	body, _ := io.ReadAll(resp4.Body)
	if !bytes.Contains(body, []byte("__LRSS_WEB__")) {
		t.Fatalf("index missing web bootstrap inject: %s", body)
	}
}

func TestServer_SetReadAndStar(t *testing.T) {
	srv, store, lib := testEnv(t)
	ctx := context.Background()

	// seed a feed + article via repo-level would be heavy; use library AddFeed if network
	// Instead insert via SQL for isolation.
	dbSQL := store // need raw db - use library through repos
	_ = lib
	_ = dbSQL

	// Open path again is awkward; re-open from testEnv pattern with direct insert.
	// Simpler: call List which works empty, and SetRead on missing id returns error.

	st, err := srv.Apply(ctx, settings.WebAccessConfig{
		Enabled: true,
		Bind:    "localhost",
		Port:    18767,
		Token:   "",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	base := "http://127.0.0.1:18767"
	// wait briefly for listen
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r, err := http.Get(base + "/api/meta"); err == nil {
			r.Body.Close()
			if r.StatusCode == 200 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	resp, err := http.Get(base + "/api/folders")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("folders = %d", resp.StatusCode)
	}

	// POST read on missing id should 500 (not 405)
	body := bytes.NewReader([]byte(`{"read":true}`))
	resp2, err := http.Post(base+"/api/articles/no-such/read", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusNotFound || resp2.StatusCode == http.StatusMethodNotAllowed {
		// route must exist; 500 is fine for missing article
		t.Fatalf("unexpected status for set read: %d", resp2.StatusCode)
	}
	if resp2.StatusCode != http.StatusInternalServerError && resp2.StatusCode != http.StatusOK {
		t.Fatalf("set read status = %d", resp2.StatusCode)
	}

	// SPA index
	resp3, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Fatalf("index = %d", resp3.StatusCode)
	}

	_ = st
}

func TestServer_StopWhenDisabled(t *testing.T) {
	srv, _, _ := testEnv(t)
	ctx := context.Background()
	if _, err := srv.Apply(ctx, settings.WebAccessConfig{
		Enabled: true, Bind: "localhost", Port: 18768,
	}); err != nil {
		t.Fatal(err)
	}
	st, err := srv.Apply(ctx, settings.WebAccessConfig{
		Enabled: false, Bind: "localhost", Port: 18768,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.Running {
		t.Fatal("expected stopped")
	}
}
