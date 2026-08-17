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

func TestArticleService_KeepFolderTree(t *testing.T) {
	const body = `<?xml version="1.0"?><rss version="2.0"><channel>
<title>Keep Tree Feed</title><link>https://ex.test/</link>
<item><title>One</title><link>https://ex.test/1</link><guid>g1</guid></item>
</channel></rss>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "keep-tree.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	lib := service.NewLibraryFromRepos(repo.New(database.SQL), &rss.Client{})
	feeds := appsvc.NewFeedService(lib)
	articles := appsvc.NewArticleService(lib, nil)

	if _, err := feeds.AddFeed(srv.URL, ""); err != nil {
		t.Fatal(err)
	}
	arts, err := articles.List("all", 10, 0)
	if err != nil || len(arts) != 1 {
		t.Fatalf("List: %v %#v", err, arts)
	}
	id := arts[0].ID
	if err := articles.Keep(id); err != nil {
		t.Fatal(err)
	}

	folder, err := articles.CreateKeepFolder("Rust", "")
	if err != nil {
		t.Fatal(err)
	}
	if folder.Name != "Rust" || folder.ID == "" {
		t.Fatalf("folder = %+v", folder)
	}
	listed, err := articles.ListKeepFolders()
	if err != nil || len(listed) != 1 || listed[0].ID != folder.ID {
		t.Fatalf("ListKeepFolders = %#v err=%v", listed, err)
	}
	if err := articles.SetKeepFolder(id, folder.ID); err != nil {
		t.Fatal(err)
	}
	inFolder, err := articles.List("kept:"+folder.ID, 10, 0)
	if err != nil || len(inFolder) != 1 || inFolder[0].ID != id {
		t.Fatalf("kept:folder = %#v err=%v", inFolder, err)
	}
	if inFolder[0].KeepFolderID != folder.ID {
		t.Fatalf("KeepFolderID = %q", inFolder[0].KeepFolderID)
	}
	allKept, err := articles.List("kept", 10, 0)
	if err != nil || len(allKept) != 1 {
		t.Fatalf("kept root = %#v err=%v", allKept, err)
	}

	if err := articles.DeleteKeepFolder(folder.ID); err != nil {
		t.Fatal(err)
	}
	after, err := articles.List("kept", 10, 0)
	if err != nil || len(after) != 1 || after[0].KeepFolderID != "" {
		t.Fatalf("after delete folder = %#v err=%v", after, err)
	}
	gone, err := articles.List("kept:"+folder.ID, 10, 0)
	if err != nil || len(gone) != 0 {
		t.Fatalf("deleted folder list = %#v", gone)
	}
}
