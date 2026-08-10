package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"lrss/internal/db"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/service"
	"lrss/internal/settings"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com/</link>
    <description>Demo</description>
    <item>
      <title>Hello World</title>
      <link>https://example.com/hello</link>
      <guid>https://example.com/hello</guid>
      <pubDate>Mon, 01 Jan 2024 12:00:00 GMT</pubDate>
      <description><![CDATA[<p>First post with <script>alert(1)</script> content</p>]]></description>
    </item>
    <item>
      <title>Second Article</title>
      <link>https://example.com/second</link>
      <guid isPermaLink="true">https://example.com/second</guid>
      <description>Just text summary</description>
    </item>
  </channel>
</rss>`

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestLibrary_AddFeedAndRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	t.Cleanup(srv.Close)

	database := openTestDB(t)
	store := settings.NewStore(database.SQL)
	repos := repo.New(database.SQL, repo.WithEmbeddingEnabled(func(ctx context.Context) bool {
		cfg, err := store.LoadEmbeddingConfig(ctx)
		return err == nil && cfg.IsConfigured()
	}))
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})

	ctx := context.Background()
	feed, err := lib.AddFeed(ctx, srv.URL+"/rss.xml", nil)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}
	if feed.Title != "Test Feed" {
		t.Fatalf("title = %q", feed.Title)
	}
	if feed.UnreadCount != 2 {
		t.Fatalf("unread = %d want 2", feed.UnreadCount)
	}

	articles, err := lib.ListArticles(ctx, "all", 10, 0)
	if err != nil {
		t.Fatalf("ListArticles: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("articles = %d want 2", len(articles))
	}

	// Script tags sanitized on write / Get
	var withHTML *string
	for i := range articles {
		a, err := lib.GetArticle(ctx, articles[i].ID)
		if err != nil {
			t.Fatalf("GetArticle: %v", err)
		}
		if a.ContentHTML != nil {
			withHTML = a.ContentHTML
			if strings.Contains(strings.ToLower(*a.ContentHTML), "<script") {
				t.Fatalf("content still has script: %s", *a.ContentHTML)
			}
		}
	}
	_ = withHTML

	// Idempotent refresh (304 path after etag stored)
	added, err := lib.RefreshFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("RefreshFeed: %v", err)
	}
	if added != 0 {
		t.Fatalf("refresh added = %d want 0", added)
	}

	if err := lib.SetRead(ctx, articles[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := lib.MarkAllRead(ctx, "feed:"+feed.ID); err != nil {
		t.Fatal(err)
	}
	feeds, err := lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].UnreadCount != 0 {
		t.Fatalf("expected 0 unread after mark all, got %+v", feeds)
	}

	if err := lib.SetStarred(ctx, articles[1].ID, true); err != nil {
		t.Fatal(err)
	}
	starred, err := lib.ListArticles(ctx, "starred", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred) != 1 {
		t.Fatalf("starred = %d", len(starred))
	}
}
