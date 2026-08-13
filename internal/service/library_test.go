package service_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"lrss/internal/db"
	"lrss/internal/model"
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

	articles, err := lib.ListArticles(ctx, "all", 10, 0, false)
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
	if err := lib.MarkAllRead(ctx, "feed:"+feed.ID, false); err != nil {
		t.Fatal(err)
	}
	feeds, err := lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].UnreadCount != 0 {
		t.Fatalf("expected 0 unread after mark all, got %+v", feeds)
	}

	// Folder context-menu path: MarkAllRead("folder:"+id) after MoveFeed.
	folder, err := lib.CreateFolder(ctx, "MenuFolder", nil)
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if err := lib.MoveFeed(ctx, feed.ID, &folder.ID); err != nil {
		t.Fatalf("MoveFeed: %v", err)
	}
	// Make one article unread again then mark via folder collection.
	if err := lib.SetRead(ctx, articles[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if err := lib.MarkAllRead(ctx, "folder:"+folder.ID, false); err != nil {
		t.Fatalf("MarkAllRead folder: %v", err)
	}
	feeds, err = lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].UnreadCount != 0 {
		t.Fatalf("folder MarkAllRead unread=%+v", feeds)
	}
	// DeleteFolder unfiles feeds (does not delete articles) — folder menu delete path.
	if err := lib.DeleteFolder(ctx, folder.ID); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	feeds, err = lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 || feeds[0].FolderID != nil {
		t.Fatalf("after DeleteFolder want unfiled feed, got %+v", feeds)
	}
	still, err := lib.ListArticles(ctx, "all", 10, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(still) < 1 {
		t.Fatal("articles must survive DeleteFolder")
	}

	if err := lib.SetStarred(ctx, articles[1].ID, true); err != nil {
		t.Fatal(err)
	}
	starred, err := lib.ListArticles(ctx, "starred", 10, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(starred) != 1 {
		t.Fatalf("starred = %d", len(starred))
	}
}

func TestLibrary_FolderCRUD_MoveFeed_SetPaused(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	// Create / rename / delete folder
	f, err := lib.CreateFolder(ctx, "  News  ", nil)
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if f.Name != "News" || f.ID == "" {
		t.Fatalf("folder = %+v", f)
	}

	emptyParent := ""
	_, err = lib.CreateFolder(ctx, "Root", &emptyParent)
	if err != nil {
		t.Fatalf("CreateFolder empty parent: %v", err)
	}

	if _, err := lib.CreateFolder(ctx, "   ", nil); err == nil {
		t.Fatal("expected empty name error")
	}

	if err := lib.RenameFolder(ctx, f.ID, "  Tech  "); err != nil {
		t.Fatalf("RenameFolder: %v", err)
	}
	folders, err := lib.ListFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var renamed bool
	for _, folder := range folders {
		if folder.ID == f.ID && folder.Name == "Tech" {
			renamed = true
		}
	}
	if !renamed {
		t.Fatalf("rename not visible in ListFolders: %+v", folders)
	}

	if err := lib.RenameFolder(ctx, f.ID, "  "); err == nil {
		t.Fatal("expected rename empty error")
	}

	// Move feed between folders / unfiled
	feed := &model.Feed{Title: "Feed", FeedURL: "https://example.com/lib-move"}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}

	if err := lib.MoveFeed(ctx, feed.ID, &f.ID); err != nil {
		t.Fatalf("MoveFeed: %v", err)
	}
	got, err := lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].FolderID == nil || *got[0].FolderID != f.ID {
		t.Fatalf("after MoveFeed feeds=%+v", got)
	}

	other, err := lib.CreateFolder(ctx, "Other", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.MoveFeed(ctx, feed.ID, &other.ID); err != nil {
		t.Fatalf("MoveFeed other: %v", err)
	}
	gotFeed, err := repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotFeed.FolderID == nil || *gotFeed.FolderID != other.ID {
		t.Fatalf("folder = %v want %s", gotFeed.FolderID, other.ID)
	}

	emptyFolder := ""
	if err := lib.MoveFeed(ctx, feed.ID, &emptyFolder); err != nil {
		t.Fatalf("MoveFeed unfiled: %v", err)
	}
	gotFeed, err = repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotFeed.FolderID != nil {
		t.Fatalf("expected unfiled, got %v", *gotFeed.FolderID)
	}

	// SetPaused
	if err := lib.SetPaused(ctx, feed.ID, true); err != nil {
		t.Fatalf("SetPaused: %v", err)
	}
	gotFeed, err = repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !gotFeed.IsPaused {
		t.Fatal("expected paused")
	}
	if err := lib.SetPaused(ctx, feed.ID, false); err != nil {
		t.Fatal(err)
	}

	// Delete folder unfiles feeds (via FK)
	if err := lib.MoveFeed(ctx, feed.ID, &f.ID); err != nil {
		t.Fatal(err)
	}
	if err := lib.DeleteFolder(ctx, f.ID); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	gotFeed, err = repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotFeed.FolderID != nil {
		t.Fatalf("after DeleteFolder expected unfiled, got %v", *gotFeed.FolderID)
	}
}

func TestLibrary_RenameFeedAndRefreshInterval(t *testing.T) {
	title := "Remote Title"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		body := strings.Replace(sampleRSS, "Test Feed", title, 1)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	feed, err := lib.AddFeed(ctx, srv.URL+"/rss.xml", nil)
	if err != nil {
		t.Fatalf("AddFeed: %v", err)
	}

	if err := lib.RenameFeed(ctx, feed.ID, "  My Name  "); err != nil {
		t.Fatalf("RenameFeed: %v", err)
	}
	got, err := repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "My Name" || !got.TitleUserSet {
		t.Fatalf("after rename: title=%q userSet=%v", got.Title, got.TitleUserSet)
	}

	// Refresh must keep user title even when remote title changes.
	title = "Completely Different"
	if _, err := lib.RefreshFeed(ctx, feed.ID); err != nil {
		t.Fatalf("RefreshFeed: %v", err)
	}
	got, err = repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "My Name" {
		t.Fatalf("title overwritten on refresh: %q", got.Title)
	}

	if err := lib.SetFeedRefreshInterval(ctx, feed.ID, 1); err != nil {
		t.Fatalf("SetFeedRefreshInterval clamp: %v", err)
	}
	got, err = repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RefreshIntervalMinutes != 5 {
		t.Fatalf("interval clamped low = %d want 5", got.RefreshIntervalMinutes)
	}

	if err := lib.SetFeedRefreshInterval(ctx, feed.ID, 0); err != nil {
		t.Fatal(err)
	}
	got, err = repos.Feeds.Get(ctx, feed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RefreshIntervalMinutes != 0 {
		t.Fatalf("interval reset = %d want 0", got.RefreshIntervalMinutes)
	}

	if err := lib.RenameFeed(ctx, feed.ID, "   "); err == nil {
		t.Fatal("expected empty rename to fail")
	}
}

func TestLibrary_SetFeedURL(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	a := &model.Feed{Title: "A", FeedURL: "https://example.com/a.xml"}
	b := &model.Feed{Title: "B", FeedURL: "https://example.com/b.xml"}
	if err := repos.Feeds.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := repos.Feeds.Insert(ctx, b); err != nil {
		t.Fatal(err)
	}
	etag := `"abc"`
	mod := "Wed, 01 Jan 2020 00:00:00 GMT"
	errMsg := "old error"
	if err := repos.Feeds.UpdateAfterFetch(ctx, a.ID, "", &etag, &mod, &errMsg); err != nil {
		t.Fatal(err)
	}

	// Invalid URL.
	if err := lib.SetFeedURL(ctx, a.ID, "not-a-url"); err == nil {
		t.Fatal("expected invalid url to fail")
	}
	// Conflict with B.
	if err := lib.SetFeedURL(ctx, a.ID, "https://example.com/b.xml"); err == nil {
		t.Fatal("expected duplicate url to fail")
	}
	// Same URL is no-op success.
	if err := lib.SetFeedURL(ctx, a.ID, "https://example.com/a.xml"); err != nil {
		t.Fatalf("same url: %v", err)
	}
	// Valid change clears validators and last_error.
	newURL := "https://example.com/a-new.xml"
	if err := lib.SetFeedURL(ctx, a.ID, newURL); err != nil {
		t.Fatalf("SetFeedURL: %v", err)
	}
	got, err := repos.Feeds.Get(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.FeedURL != newURL {
		t.Fatalf("FeedURL = %q want %q", got.FeedURL, newURL)
	}
	if got.ETag != nil || got.LastModified != nil {
		t.Fatalf("expected etag/last_modified cleared, etag=%v lm=%v", got.ETag, got.LastModified)
	}
	if got.LastError != nil {
		t.Fatalf("expected last_error cleared, got %v", *got.LastError)
	}
}

func TestLibrary_NSFWOfficeModeFiltersListAndCounts(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	safe := &model.Feed{Title: "Safe", FeedURL: "https://example.com/safe.xml"}
	nsfw := &model.Feed{Title: "NSFW", FeedURL: "https://example.com/nsfw.xml", IsNsfw: true}
	if err := repos.Feeds.Insert(ctx, safe); err != nil {
		t.Fatal(err)
	}
	if err := repos.Feeds.Insert(ctx, nsfw); err != nil {
		t.Fatal(err)
	}
	// Insert articles via raw SQL (minimal)
	now := "2026-06-01T12:00:00Z"
	for _, row := range []struct {
		id, feed, title string
	}{
		{"a-safe", safe.ID, "Safe Article"},
		{"a-nsfw", nsfw.ID, "NSFW Article"},
	} {
		_, err := database.SQL.ExecContext(ctx, `
			INSERT INTO articles (id, feed_id, url, title, fetched_at, is_read, is_starred)
			VALUES (?, ?, ?, ?, ?, 0, 0)`,
			row.id, row.feed, "https://example.com/"+row.id, row.title, now)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Show all
	all, err := lib.ListArticles(ctx, "all", 20, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("show all = %d want 2", len(all))
	}
	// Office hide
	office, err := lib.ListArticles(ctx, "all", 20, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(office) != 1 || office[0].Title != "Safe Article" {
		t.Fatalf("office list = %+v", office)
	}
	counts, err := lib.SmartCounts(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if counts.All != 1 || counts.Unread != 1 {
		t.Fatalf("office counts = %+v", counts)
	}
	// SetNsfw toggle
	if err := lib.SetFeedNsfw(ctx, nsfw.ID, false); err != nil {
		t.Fatal(err)
	}
	got, err := repos.Feeds.Get(ctx, nsfw.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsNsfw {
		t.Fatal("expected is_nsfw cleared")
	}
}

// Empty local article table + stored ETag used to 304 forever and stay empty.
func TestLibrary_RefreshRepopulatesEmptyFeedIgnoringETag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == `"stuck"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"stuck"`)
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	t.Cleanup(srv.Close)

	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	// Insert feed as if AddFeed left it empty but with an ETag (zombie state).
	etag := `"stuck"`
	now := time.Now().UTC().Format(time.RFC3339)
	feed := &model.Feed{
		Title:         "Zombie",
		FeedURL:       srv.URL + "/rss.xml",
		ETag:          &etag,
		LastFetchedAt: &now,
	}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	n, err := repos.Articles.CountByFeed(ctx, feed.ID)
	if err != nil || n != 0 {
		t.Fatalf("setup count = %d err=%v", n, err)
	}

	added, err := lib.RefreshFeed(ctx, feed.ID)
	if err != nil {
		t.Fatalf("RefreshFeed: %v", err)
	}
	if added != 2 {
		t.Fatalf("added = %d want 2 (must ignore ETag when local empty)", added)
	}
	n, err = repos.Articles.CountByFeed(ctx, feed.ID)
	if err != nil || n != 2 {
		t.Fatalf("after count = %d err=%v", n, err)
	}

	// Re-adding same URL with empty would have returned existing; with fix it repopulates.
	// After we already have articles, AddFeed should just return existing.
	again, err := lib.AddFeed(ctx, srv.URL+"/rss.xml", nil)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != feed.ID {
		t.Fatalf("expected same feed id")
	}
}

func TestLibrary_FolderNSFWOfficeMode(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	folder, err := lib.CreateFolder(ctx, "Adult", nil)
	if err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}
	if folder.IsNsfw {
		t.Fatal("new folder should not be nsfw")
	}
	if err := lib.SetFolderNsfw(ctx, folder.ID, true); err != nil {
		t.Fatalf("SetFolderNsfw: %v", err)
	}
	gotFolder, err := repos.Folders.Get(ctx, folder.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !gotFolder.IsNsfw {
		t.Fatal("folder should be nsfw")
	}

	safe := &model.Feed{Title: "Safe", FeedURL: "https://example.com/safe2.xml"}
	inFolder := &model.Feed{
		Title: "In NSFW folder", FeedURL: "https://example.com/in-folder.xml", FolderID: &folder.ID,
	}
	if err := repos.Feeds.Insert(ctx, safe); err != nil {
		t.Fatal(err)
	}
	if err := repos.Feeds.Insert(ctx, inFolder); err != nil {
		t.Fatal(err)
	}
	now := "2026-06-01T12:00:00Z"
	for _, row := range []struct {
		id, feed, title string
	}{
		{"fa-safe", safe.ID, "Safe Article"},
		{"fa-folder", inFolder.ID, "Folder Article"},
	} {
		_, err := database.SQL.ExecContext(ctx, `
			INSERT INTO articles (id, feed_id, url, title, fetched_at, is_read, is_starred)
			VALUES (?, ?, ?, ?, ?, 0, 0)`,
			row.id, row.feed, "https://example.com/"+row.id, row.title, now)
		if err != nil {
			t.Fatal(err)
		}
	}

	office, err := lib.ListArticles(ctx, "all", 20, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(office) != 1 || office[0].Title != "Safe Article" {
		t.Fatalf("office list = %+v", office)
	}
	counts, err := lib.SmartCounts(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if counts.All != 1 {
		t.Fatalf("office counts.All = %d want 1", counts.All)
	}

	if err := lib.SetFolderNsfw(ctx, folder.ID, false); err != nil {
		t.Fatal(err)
	}
	all, err := lib.ListArticles(ctx, "all", 20, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("after unmark folder = %d want 2", len(all))
	}
}

func TestLibrary_FolderDisplayMode(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	folder, err := lib.CreateFolder(ctx, "Photos", nil)
	if err != nil {
		t.Fatal(err)
	}
	if folder.DisplayMode != model.FolderDisplayList {
		t.Fatalf("new folder mode = %q", folder.DisplayMode)
	}
	if err := lib.SetFolderDisplayMode(ctx, folder.ID, "gallery"); err != nil {
		t.Fatal(err)
	}
	got, err := repos.Folders.Get(ctx, folder.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayMode != model.FolderDisplayCards {
		t.Fatalf("after set = %q want cards", got.DisplayMode)
	}
	list, err := repos.Folders.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].DisplayMode != model.FolderDisplayCards {
		t.Fatalf("list mode = %+v", list)
	}
	if err := lib.SetFolderDisplayMode(ctx, folder.ID, "nope"); err != nil {
		t.Fatal(err)
	}
	got, err = repos.Folders.Get(ctx, folder.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayMode != model.FolderDisplayList {
		t.Fatalf("unknown mode should normalize to list, got %q", got.DisplayMode)
	}
}

func TestEffectiveRefreshMinutesAndDue(t *testing.T) {
	now := mustTime(t, "2026-01-01T12:00:00Z")

	// Global default when feed interval is 0.
	f := model.Feed{ID: "feed-a", RefreshIntervalMinutes: 0}
	if m := service.EffectiveRefreshMinutes(f, 30); m != 30 {
		t.Fatalf("global = %d", m)
	}
	if m := service.EffectiveRefreshMinutes(f, 2); m != 5 {
		t.Fatalf("global floor = %d", m)
	}

	// Per-feed override.
	f.RefreshIntervalMinutes = 15
	if m := service.EffectiveRefreshMinutes(f, 60); m != 15 {
		t.Fatalf("per-feed = %d", m)
	}

	// Fetched 10m ago with 15m interval → not due (age).
	last := "2026-01-01T11:50:00Z"
	f.LastFetchedAt = &last
	if service.FeedRefreshDue(f, 30, now) {
		t.Fatal("should not be due yet")
	}

	// Fetched 20m ago with 15m interval → age ok; due only on phase match or 2× overdue.
	last = "2026-01-01T11:40:00Z"
	f.LastFetchedAt = &last
	// Force phase match by probing slots over the interval.
	dueOnce := false
	for min := 0; min < 15; min++ {
		probe := now.Add(time.Duration(min) * time.Minute)
		if service.FeedRefreshDue(f, 30, probe) {
			dueOnce = true
			break
		}
	}
	if !dueOnce {
		t.Fatal("expected due on some phase slot within interval")
	}

	// 2× overdue → due regardless of phase.
	last = "2026-01-01T11:00:00Z" // 60m ago, interval 15 → 4×
	f.LastFetchedAt = &last
	if !service.FeedRefreshDue(f, 30, now) {
		t.Fatal("very overdue should be due without phase wait")
	}
}

type stubRSS struct {
	calls int
}

func (s *stubRSS) Fetch(ctx context.Context, feedURL string, opts rss.FetchOptions) (*rss.FetchResult, error) {
	s.calls++
	return nil, context.Canceled // fail fast; still counts as a refresh attempt
}

func TestQueueNewArticlesForFullContent_AndDrain(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	ft := &stubFulltext{
		html: `<p>Expanded full article body with plenty of detail for the reader.</p>`,
		text: `Expanded full article body with plenty of detail for the reader.`,
	}
	lib := service.NewLibraryFromRepos(repos, &stubRSS{})
	lib.Fulltext = ft
	lib.FullContentEnabled = func(context.Context) bool { return true }
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://example.com/ft.xml"}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	// Strong truncation cue → should queue.
	sum := "This is a teaser summary that is long enough to compare against body."
	body := "Short lead. Read more…"
	res, err := repos.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{{
		GUID:        "g1",
		URL:         "https://example.com/article-1",
		Title:       "Partial piece",
		Summary:     &sum,
		ContentText: &body,
		ContentHTML: ptrHTML("<p>" + body + "</p>"),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Inserted != 1 || len(res.InsertedIDs) != 1 {
		t.Fatalf("upsert = %+v", res)
	}

	lib.QueueNewArticlesForFullContent(ctx, res.InsertedIDs)
	if lib.FulltextQueueLen() != 1 {
		t.Fatalf("queue = %d want 1", lib.FulltextQueueLen())
	}

	// Full-looking article must not queue.
	long := strings.Repeat("This is a complete paragraph about the topic. ", 40)
	res2, err := repos.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{{
		GUID:        "g2",
		URL:         "https://example.com/article-2",
		Title:       "Full piece",
		ContentText: &long,
	}})
	if err != nil {
		t.Fatal(err)
	}
	lib.QueueNewArticlesForFullContent(ctx, res2.InsertedIDs)
	if lib.FulltextQueueLen() != 1 {
		t.Fatalf("queue after full body = %d want 1", lib.FulltextQueueLen())
	}

	okN, failN, pending := lib.TryDrainFulltext(ctx, 4)
	if okN != 1 || failN != 0 {
		t.Fatalf("drain ok=%d fail=%d pending=%d calls=%d", okN, failN, pending, ft.calls)
	}
	if lib.FulltextQueueLen() != 0 {
		t.Fatalf("queue after drain = %d", lib.FulltextQueueLen())
	}
	if ft.calls != 1 {
		t.Fatalf("fulltext calls = %d", ft.calls)
	}
	got, err := repos.Articles.Get(ctx, res.InsertedIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if !got.FullContentFetched {
		t.Fatal("expected full_content_fetched")
	}
}

func ptrHTML(s string) *string { return &s }

func TestRefreshAll_UsesForceQueueAndCap(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	stub := &stubRSS{}
	lib := service.NewLibraryFromRepos(repos, stub)
	ctx := context.Background()

	for i := 0; i < 45; i++ {
		f := &model.Feed{
			Title:   "F" + strconv.Itoa(i),
			FeedURL: "https://example.com/force-" + strconv.Itoa(i) + ".xml",
		}
		if err := repos.Feeds.Insert(ctx, f); err != nil {
			t.Fatal(err)
		}
	}

	n, err := lib.EnqueueRefreshAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 45 {
		t.Fatalf("enqueued = %d want 45", n)
	}
	if lib.ForceQueueLen() != 45 {
		t.Fatalf("queue = %d", lib.ForceQueueLen())
	}

	res, ok, err := lib.TryRefreshWork(ctx, 30, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected batch lock")
	}
	if stub.calls != service.AutoRefreshMaxFeedsPerTick {
		t.Fatalf("rss calls = %d want %d", stub.calls, service.AutoRefreshMaxFeedsPerTick)
	}
	if res.FeedsPending != 45-service.AutoRefreshMaxFeedsPerTick {
		t.Fatalf("pending = %d want %d (ok=%d err=%d)",
			res.FeedsPending, 45-service.AutoRefreshMaxFeedsPerTick, res.FeedsOK, res.FeedsErr)
	}
}

func TestSelectFeedsDueForRefresh_StaggerAndCap(t *testing.T) {
	// Same last_fetched for many feeds (OPML bulk) with 30m interval.
	last := "2026-01-01T11:30:00Z" // 30m before now → just due
	now := mustTime(t, "2026-01-01T12:00:00Z")
	var feeds []model.Feed
	for i := 0; i < 90; i++ {
		id := "bulk-" + strconv.Itoa(i)
		feeds = append(feeds, model.Feed{
			ID:                     id,
			RefreshIntervalMinutes: 0,
			LastFetchedAt:          &last,
		})
	}

	// Across 30 wall-clock minutes, phase should spread due feeds; each tick capped.
	totalPicked := 0
	maxInOneTick := 0
	for min := 0; min < 30; min++ {
		probe := now.Add(time.Duration(min) * time.Minute)
		picked := service.SelectFeedsDueForRefresh(feeds, 30, probe, service.AutoRefreshMaxFeedsPerTick)
		if len(picked) > service.AutoRefreshMaxFeedsPerTick {
			t.Fatalf("tick %d: picked %d > cap", min, len(picked))
		}
		if len(picked) > maxInOneTick {
			maxInOneTick = len(picked)
		}
		totalPicked += len(picked)
	}
	// Without stagger, one tick would take all 90 (or cap 20 once). With phase,
	// load spreads: many ticks contribute and no single tick grabs everything.
	if totalPicked < 40 {
		t.Fatalf("expected staggered picks over 30m, totalPicked=%d", totalPicked)
	}
	if maxInOneTick >= 90 {
		t.Fatalf("stagger failed: one tick took %d", maxInOneTick)
	}
	// Cap must bite when catch-up marks everyone due (2× interval).
	veryOld := "2026-01-01T10:00:00Z"
	for i := range feeds {
		feeds[i].LastFetchedAt = &veryOld
	}
	capped := service.SelectFeedsDueForRefresh(feeds, 30, now, service.AutoRefreshMaxFeedsPerTick)
	if len(capped) != service.AutoRefreshMaxFeedsPerTick {
		t.Fatalf("catch-up cap = %d want %d", len(capped), service.AutoRefreshMaxFeedsPerTick)
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tt, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tt
}

type stubFulltext struct {
	html, text string
	err        error
	calls      int
	lastURL    string
}

func (s *stubFulltext) Fetch(ctx context.Context, pageURL string) (string, string, error) {
	s.calls++
	s.lastURL = pageURL
	return s.html, s.text, s.err
}

func TestLibrary_FetchFullContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(sampleRSS))
	}))
	t.Cleanup(srv.Close)

	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	stub := &stubFulltext{
		html: `<p>Expanded full article body with more detail.</p>`,
		text: `Expanded full article body with more detail.`,
	}
	lib.Fulltext = stub

	ctx := context.Background()
	feed, err := lib.AddFeed(ctx, srv.URL+"/rss.xml", nil)
	if err != nil {
		t.Fatal(err)
	}
	arts, err := lib.ListArticles(ctx, "feed:"+feed.ID, 10, 0, false)
	if err != nil || len(arts) == 0 {
		t.Fatalf("articles: %v %#v", err, arts)
	}
	id := arts[0].ID
	// Force a partial body so we can see replacement.
	_ = repos.Articles.UpdateContent(ctx, id, "<p>partial</p>", "partial")

	updated, err := lib.FetchFullContent(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 {
		t.Fatalf("fulltext calls = %d", stub.calls)
	}
	if !strings.Contains(stub.lastURL, "example.com") {
		t.Fatalf("fetched url = %q", stub.lastURL)
	}
	if updated.ContentHTML == nil || !strings.Contains(*updated.ContentHTML, "Expanded full") {
		t.Fatalf("content = %#v", updated.ContentHTML)
	}
}

// recordOpenedStore spies on ArticleStore.RecordOpened.
type recordOpenedStore struct {
	service.ArticleStore
	lastID   string
	lastKeep int
	calls    int
}

func (s *recordOpenedStore) RecordOpened(ctx context.Context, articleID string, keep int) error {
	s.lastID = articleID
	s.lastKeep = keep
	s.calls++
	return nil
}

func TestLibrary_RecordOpened(t *testing.T) {
	stub := &recordOpenedStore{}
	lib := service.NewLibrary(nil, stub, nil, nil)

	if err := lib.RecordOpened(context.Background(), "art-1", 80); err != nil {
		t.Fatal(err)
	}
	if stub.lastID != "art-1" || stub.lastKeep != 80 || stub.calls != 1 {
		t.Fatalf("got id=%q keep=%d calls=%d", stub.lastID, stub.lastKeep, stub.calls)
	}
	if err := lib.RecordOpened(context.Background(), "art-2", 0); err != nil {
		t.Fatal(err)
	}
	if stub.lastKeep != 50 {
		t.Fatalf("keep 0 → 50, got %d", stub.lastKeep)
	}
	if err := lib.RecordOpened(context.Background(), "  ", 10); err == nil {
		t.Fatal("expected empty id error")
	}
}

func TestLibrary_RecordOpened_RecentCollection(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://example.com/recent.xml"}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := repos.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{GUID: "a", URL: "https://example.com/a", Title: "First", PublishedAt: &now},
		{GUID: "b", URL: "https://example.com/b", Title: "Second", PublishedAt: &now},
	})
	if err != nil {
		t.Fatal(err)
	}
	arts, err := lib.ListArticles(ctx, "all", 10, 0, false)
	if err != nil || len(arts) != 2 {
		t.Fatalf("seed list: %v n=%d", err, len(arts))
	}

	if err := lib.RecordOpened(ctx, arts[0].ID, 50); err != nil {
		t.Fatal(err)
	}
	if err := lib.RecordOpened(ctx, arts[1].ID, 50); err != nil {
		t.Fatal(err)
	}

	recent, err := lib.ListArticles(ctx, "recent", 10, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("recent = %d want 2", len(recent))
	}
	got := map[string]bool{recent[0].ID: true, recent[1].ID: true}
	if !got[arts[0].ID] || !got[arts[1].ID] {
		t.Fatalf("recent ids = %q %q want %q %q", recent[0].ID, recent[1].ID, arts[0].ID, arts[1].ID)
	}
}

func TestLibrary_PruneOpened(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	feed := &model.Feed{Title: "F", FeedURL: "https://example.com/prune.xml"}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	items := make([]repo.ParsedItem, 12)
	for i := range items {
		title := "P" + strconv.Itoa(i)
		items[i] = repo.ParsedItem{
			GUID: title, URL: "https://example.com/" + title, Title: title, PublishedAt: &now,
		}
	}
	if _, err := repos.Articles.UpsertFromParsed(ctx, feed.ID, items); err != nil {
		t.Fatal(err)
	}
	arts, err := lib.ListArticles(ctx, "all", 20, 0, false)
	if err != nil || len(arts) != 12 {
		t.Fatalf("seed list: %v n=%d", err, len(arts))
	}
	for _, a := range arts {
		if err := lib.RecordOpened(ctx, a.ID, 50); err != nil {
			t.Fatal(err)
		}
	}
	if err := lib.PruneOpened(ctx, 10); err != nil {
		t.Fatal(err)
	}
	recent, err := lib.ListArticles(ctx, "recent", 20, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 10 {
		t.Fatalf("after prune recent=%d want 10", len(recent))
	}
}
