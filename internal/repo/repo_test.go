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

func TestPurgeOlderThan_KeepsStarredDeletesOld(t *testing.T) {
	r, database := openTestRepos(t, false)
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/purge"}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}

	oldPub := time.Now().UTC().Add(-120 * 24 * time.Hour).Format(time.RFC3339)
	recentPub := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	body := "purge body text"

	_, err := r.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{GUID: "old-plain", URL: "https://ex.com/old", Title: "Old Plain", ContentText: &body, PublishedAt: &oldPub},
		{GUID: "old-star", URL: "https://ex.com/old-star", Title: "Old Starred", ContentText: &body, PublishedAt: &oldPub},
		{GUID: "recent", URL: "https://ex.com/recent", Title: "Recent", ContentText: &body, PublishedAt: &recentPub},
	})
	if err != nil {
		t.Fatal(err)
	}

	all, err := r.Articles.List(ctx, "all", repo.ListOpts{Limit: 20})
	if err != nil || len(all) != 3 {
		t.Fatalf("list before: n=%d err=%v", len(all), err)
	}

	var starID, oldID string
	for _, a := range all {
		switch a.Title {
		case "Old Starred":
			starID = a.ID
		case "Old Plain":
			oldID = a.ID
		}
	}
	if starID == "" || oldID == "" {
		t.Fatal("missing seeded articles")
	}
	if err := r.Articles.SetStarred(ctx, starID, true); err != nil {
		t.Fatal(err)
	}

	// FTS rows should exist for all three before purge.
	var ftsBefore int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM articles_fts`).Scan(&ftsBefore); err != nil {
		t.Fatal(err)
	}
	if ftsBefore != 3 {
		t.Fatalf("fts before = %d want 3", ftsBefore)
	}

	deleted, err := r.Articles.PurgeOlderThan(ctx, 90)
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d want 1 (only non-starred old)", deleted)
	}

	remaining, err := r.Articles.List(ctx, "all", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining = %d want 2", len(remaining))
	}
	titles := map[string]bool{}
	for _, a := range remaining {
		titles[a.Title] = true
	}
	if !titles["Old Starred"] || !titles["Recent"] {
		t.Fatalf("remaining titles = %v", titles)
	}
	if titles["Old Plain"] {
		t.Fatal("Old Plain should have been purged")
	}

	// FTS should no longer index the purged article.
	var ftsAfter int
	if err := database.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM articles_fts`).Scan(&ftsAfter); err != nil {
		t.Fatal(err)
	}
	if ftsAfter != 2 {
		t.Fatalf("fts after = %d want 2", ftsAfter)
	}
	var ftsOld int
	if err := database.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM articles_fts WHERE article_id = ?`, oldID,
	).Scan(&ftsOld); err != nil {
		t.Fatal(err)
	}
	if ftsOld != 0 {
		t.Fatalf("purged article still in FTS")
	}

	// Idempotent second purge.
	deleted2, err := r.Articles.PurgeOlderThan(ctx, 90)
	if err != nil {
		t.Fatal(err)
	}
	if deleted2 != 0 {
		t.Fatalf("second purge deleted = %d want 0", deleted2)
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

	got, err := r.Folders.Get(ctx, f.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "News" || got.ID != f.ID {
		t.Fatalf("Get = %+v", got)
	}

	if err := r.Folders.Rename(ctx, f.ID, "  Tech  "); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	got, err = r.Folders.Get(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Tech" {
		t.Fatalf("after rename name=%q want Tech", got.Name)
	}

	if err := r.Folders.Rename(ctx, f.ID, "   "); err == nil {
		t.Fatal("expected error for empty rename")
	}
	if _, err := r.Folders.Get(ctx, "missing"); err == nil {
		t.Fatal("expected get missing error")
	}

	if err := r.Folders.Delete(ctx, f.ID); err != nil {
		t.Fatal(err)
	}
	list, err = r.Folders.List(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("after delete list=%+v err=%v", list, err)
	}
}

func TestFeed_SetFolderAndSetPaused(t *testing.T) {
	r, _ := openTestRepos(t, false)
	ctx := context.Background()

	folder, err := r.Folders.Create(ctx, "News", nil)
	if err != nil {
		t.Fatal(err)
	}
	folder2, err := r.Folders.Create(ctx, "Tech", nil)
	if err != nil {
		t.Fatal(err)
	}

	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/move"}
	if err := r.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	if feed.FolderID != nil {
		t.Fatalf("expected unfiled, got %v", feed.FolderID)
	}

	if err := r.Feeds.SetFolder(ctx, feed.ID, &folder.ID); err != nil {
		t.Fatalf("SetFolder: %v", err)
	}
	got, err := r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FolderID == nil || *got.FolderID != folder.ID {
		t.Fatalf("folder after move = %v want %s", got.FolderID, folder.ID)
	}

	if err := r.Feeds.SetFolder(ctx, feed.ID, &folder2.ID); err != nil {
		t.Fatalf("SetFolder move: %v", err)
	}
	got, err = r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FolderID == nil || *got.FolderID != folder2.ID {
		t.Fatalf("folder after second move = %v want %s", got.FolderID, folder2.ID)
	}

	empty := "  "
	if err := r.Feeds.SetFolder(ctx, feed.ID, &empty); err != nil {
		t.Fatalf("SetFolder unfiled empty: %v", err)
	}
	got, err = r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FolderID != nil {
		t.Fatalf("expected unfiled after empty folder id, got %v", *got.FolderID)
	}

	// re-file then unfile with nil
	if err := r.Feeds.SetFolder(ctx, feed.ID, &folder.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Feeds.SetFolder(ctx, feed.ID, nil); err != nil {
		t.Fatalf("SetFolder nil: %v", err)
	}
	got, err = r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FolderID != nil {
		t.Fatalf("expected unfiled after nil, got %v", *got.FolderID)
	}

	missing := "no-such-folder"
	if err := r.Feeds.SetFolder(ctx, feed.ID, &missing); err == nil {
		t.Fatal("expected error for missing folder")
	}

	if err := r.Feeds.SetPaused(ctx, feed.ID, true); err != nil {
		t.Fatalf("SetPaused: %v", err)
	}
	got, err = r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsPaused {
		t.Fatal("expected is_paused true")
	}
	if err := r.Feeds.SetPaused(ctx, feed.ID, false); err != nil {
		t.Fatal(err)
	}
	got, err = r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsPaused {
		t.Fatal("expected is_paused false")
	}

	// delete folder unfiles feeds via FK
	if err := r.Feeds.SetFolder(ctx, feed.ID, &folder.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Folders.Delete(ctx, folder.ID); err != nil {
		t.Fatal(err)
	}
	got, err = r.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FolderID != nil {
		t.Fatalf("after folder delete expected unfiled, got %v", *got.FolderID)
	}
}
