package appsvc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"lrss/internal/appsvc"
	"lrss/internal/db"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/service"
	"lrss/internal/settings"
)

func TestFeedService_AddAndList(t *testing.T) {
	const body = `<?xml version="1.0"?><rss version="2.0"><channel>
<title>Wails Feed</title><link>https://ex.test/</link>
<item><title>One</title><link>https://ex.test/1</link><guid>g1</guid></item>
</channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	repos := repo.New(database.SQL, repo.WithEmbeddingEnabled(func(ctx context.Context) bool {
		cfg, err := store.LoadEmbeddingConfig(ctx)
		return err == nil && cfg.IsConfigured()
	}))
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	feeds := appsvc.NewFeedService(lib)
	articles := appsvc.NewArticleService(lib, nil)

	f, err := feeds.AddFeed(srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Title != "Wails Feed" {
		t.Fatalf("title %q", f.Title)
	}
	list, err := feeds.ListFeeds()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListFeeds: %v %#v", err, list)
	}
	arts, err := articles.List("all", 10, 0)
	if err != nil || len(arts) != 1 {
		t.Fatalf("List: %v %#v", err, arts)
	}
	if err := articles.SetStarred(arts[0].ID, true); err != nil {
		t.Fatal(err)
	}
}

func TestArticleService_RecordOpened(t *testing.T) {
	const body = `<?xml version="1.0"?><rss version="2.0"><channel>
<title>Recent Feed</title><link>https://ex.test/</link>
<item><title>One</title><link>https://ex.test/1</link><guid>g1</guid></item>
<item><title>Two</title><link>https://ex.test/2</link><guid>g2</guid></item>
</channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	prefs, err := store.LoadUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefs.RecentReadLimit = 10
	if err := store.SaveUIPrefs(ctx, prefs); err != nil {
		t.Fatal(err)
	}

	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	feeds := appsvc.NewFeedService(lib)
	articles := appsvc.NewArticleService(lib, store)

	if _, err := feeds.AddFeed(srv.URL, ""); err != nil {
		t.Fatal(err)
	}
	arts, err := articles.List("all", 10, 0)
	if err != nil || len(arts) < 1 {
		t.Fatalf("List: %v %#v", err, arts)
	}
	if err := articles.RecordOpened(arts[0].ID); err != nil {
		t.Fatal(err)
	}
	recent, err := articles.List("recent", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 1 || recent[0].ID != arts[0].ID {
		t.Fatalf("recent = %#v want id %s", recent, arts[0].ID)
	}
}
