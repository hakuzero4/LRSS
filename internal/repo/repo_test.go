package repo_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lrss/internal/db"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/search"
)

func openTestRepos(t *testing.T, embedOn bool) (*repo.Repos, *db.DB) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "repo.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	var opts []repo.Option
	if embedOn {
		opts = append(opts, repo.WithEmbeddingEnabled(func(context.Context) bool { return true }))
	}
	return repo.New(database.SQL, opts...), database
}

func TestInsertFeedArticles_ListUnread(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	feed := &model.Feed{
		Title:   "Example",
		FeedURL: "https://example.com/feed.xml",
	}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if feed.ID == "" {
		t.Fatal("expected feed id")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	summary := "hello world"
	body := "body of article one"
	res, err := r.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{
			GUID:        "g1",
			URL:         "https://example.com/1",
			Title:       "Article One",
			Summary:     &summary,
			ContentText: &body,
			PublishedAt: &now,
		},
		{
			GUID:        "g2",
			URL:         "https://example.com/2",
			Title:       "Article Two",
			PublishedAt: &now,
		},
	})
	if err != nil {
		t.Fatalf("UpsertFromParsed: %v", err)
	}
	if res.Inserted != 2 || res.Skipped != 0 {
		t.Fatalf("upsert result = %+v want inserted=2 skipped=0", res)
	}

	unread, err := r.Articles.List(ctx, "unread", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatalf("List unread: %v", err)
	}
	if len(unread) != 2 {
		t.Fatalf("unread count = %d want 2", len(unread))
	}

	feeds, err := r.Feeds.List(ctx)
	if err != nil {
		t.Fatalf("List feeds: %v", err)
	}
	if len(feeds) != 1 || feeds[0].UnreadCount != 2 {
		t.Fatalf("feeds=%+v want one feed unread=2", feeds)
	}
}

func TestSetRead_DecreasesUnread(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/rss"}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	_, err := r.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{GUID: "a", URL: "https://ex.com/a", Title: "A"},
		{GUID: "b", URL: "https://ex.com/b", Title: "B"},
	})
	if err != nil {
		t.Fatal(err)
	}

	articles, err := r.Articles.List(ctx, "all", repo.ListOpts{Limit: 10})
	if err != nil || len(articles) != 2 {
		t.Fatalf("list: %v n=%d", err, len(articles))
	}

	if err := r.Articles.SetRead(ctx, articles[0].ID, true); err != nil {
		t.Fatalf("SetRead: %v", err)
	}

	unread, err := r.Articles.List(ctx, "unread", repo.ListOpts{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(unread) != 1 {
		t.Fatalf("unread after SetRead = %d want 1", len(unread))
	}

	got, err := r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.UnreadCount != 1 {
		t.Fatalf("UnreadCount=%d want 1", got.UnreadCount)
	}
}

func TestUpsertFTS_NoPanic(t *testing.T) {
	r, database := openTestRepos(t, false)
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/fts"}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	body := "full text search content about golang"
	res, err := r.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{GUID: "fts1", URL: "https://ex.com/fts1", Title: "Golang Tips", ContentText: &body},
	})
	if err != nil {
		t.Fatalf("UpsertFromParsed: %v", err)
	}
	if res.Inserted != 1 {
		t.Fatalf("inserted=%d", res.Inserted)
	}

	articles, err := r.Articles.List(ctx, "all", repo.ListOpts{Limit: 5})
	if err != nil || len(articles) != 1 {
		t.Fatalf("list: %v n=%d", err, len(articles))
	}
	if err := search.UpsertFTS(ctx, database.SQL, articles[0].ID, "Golang Tips", "", body); err != nil {
		t.Fatalf("UpsertFTS: %v", err)
	}

	hits, err := search.SearchFTS(ctx, database.SQL, "golang", 10)
	if err != nil {
		t.Fatalf("SearchFTS: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("expected FTS hits")
	}
}

func TestUniqueFeedGUID_NoDuplicate(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/dup"}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}

	items := []repo.ParsedItem{
		{GUID: "same", URL: "https://ex.com/1", Title: "First"},
		{GUID: "same", URL: "https://ex.com/1b", Title: "Second attempt"},
	}
	res1, err := r.Articles.UpsertFromParsed(ctx, feed.ID, items[:1])
	if err != nil {
		t.Fatal(err)
	}
	if res1.Inserted != 1 {
		t.Fatalf("first insert %+v", res1)
	}

	res2, err := r.Articles.UpsertFromParsed(ctx, feed.ID, items)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Inserted != 0 || res2.Skipped != 2 {
		t.Fatalf("second upsert %+v want inserted=0 skipped=2", res2)
	}

	all, err := r.Articles.List(ctx, "all", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("articles=%d want 1 (no duplicate guid)", len(all))
	}
	if all[0].Title != "First" {
		t.Fatalf("title=%q want First (existing not overwritten)", all[0].Title)
	}
}

func TestUpsert_EmbeddingPending(t *testing.T) {
	r, database := openTestRepos(t, true)
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/emb"}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	res, err := r.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{GUID: "e1", URL: "https://ex.com/e1", Title: "Embed me"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 1 {
		t.Fatalf("%+v", res)
	}
	var n int
	if err := database.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM article_embeddings WHERE status = 'pending'`,
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pending embeddings=%d want 1", n)
	}
}

func TestFolders_CRUD(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	f, err := r.Folders.Create(ctx, "News", nil)
	if err != nil {
		t.Fatal(err)
	}
	list, err := r.Folders.List(ctx)
	if err != nil || len(list) != 1 || list[0].Name != "News" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if err := r.Folders.Delete(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	list, err = r.Folders.List(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete list=%+v err=%v", list, err)
	}
}
