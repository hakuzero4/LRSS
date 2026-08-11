package service_test

import (
	"context"
	"strings"
	"testing"

	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/service"
)

const sampleOPML = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>Test</title></head>
  <body>
    <outline text="News" title="News">
      <outline type="rss" text="Example" title="Example"
               xmlUrl="https://example.com/feed.xml"
               htmlUrl="https://example.com"/>
    </outline>
    <outline type="rss" text="Bare" xmlUrl="https://bare.example/rss"/>
  </body>
</opml>`

func TestLibrary_ImportOPML_NoFetch(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	res, err := lib.ImportOPML(ctx, sampleOPML, false)
	if err != nil {
		t.Fatalf("ImportOPML: %v", err)
	}
	if res.FoldersCreated != 1 {
		t.Fatalf("foldersCreated = %d want 1", res.FoldersCreated)
	}
	if res.FeedsAdded != 2 {
		t.Fatalf("feedsAdded = %d want 2", res.FeedsAdded)
	}
	if len(res.AddedFeedIDs) != 2 {
		t.Fatalf("addedFeedIds = %d want 2", len(res.AddedFeedIDs))
	}
	if res.FeedsSkipped != 0 {
		t.Fatalf("feedsSkipped = %d want 0", res.FeedsSkipped)
	}

	feeds, err := lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 2 {
		t.Fatalf("feeds = %d want 2", len(feeds))
	}
	folders, err := lib.ListFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0].Name != "News" {
		t.Fatalf("folders = %+v", folders)
	}

	// Second import: unchanged feeds skipped — folders reused, no duplicates
	res2, err := lib.ImportOPML(ctx, sampleOPML, false)
	if err != nil {
		t.Fatalf("ImportOPML 2: %v", err)
	}
	if res2.FeedsSkipped != 2 {
		t.Fatalf("second import skipped = %d want 2", res2.FeedsSkipped)
	}
	if res2.FeedsUpdated != 0 {
		t.Fatalf("second import updated = %d want 0 (already matched OPML)", res2.FeedsUpdated)
	}
	if res2.FeedsAdded != 0 {
		t.Fatalf("second import added = %d want 0", res2.FeedsAdded)
	}
	if res2.FoldersCreated != 0 {
		t.Fatalf("second foldersCreated = %d want 0 (reuse existing)", res2.FoldersCreated)
	}
	folders2, err := lib.ListFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders2) != 1 {
		t.Fatalf("after re-import folders = %d want 1 (no duplicates)", len(folders2))
	}
}

func TestLibrary_ImportOPML_UpdatesExistingFolderAndTitle(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	// Pre-subscribe at root with a different title (not user-locked).
	preOPML := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0"><body>
  <outline type="rss" text="Old Title" xmlUrl="https://example.com/feed.xml"/>
</body></opml>`
	if _, err := lib.ImportOPML(ctx, preOPML, false); err != nil {
		t.Fatal(err)
	}
	feeds, err := lib.ListFeeds(ctx)
	if err != nil || len(feeds) != 1 {
		t.Fatalf("pre feeds: %v len=%d", err, len(feeds))
	}
	if feeds[0].FolderID != nil {
		t.Fatalf("pre feed should be unfiled")
	}
	exampleID := feeds[0].ID

	res, err := lib.ImportOPML(ctx, sampleOPML, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.FeedsAdded != 1 {
		t.Fatalf("feedsAdded = %d want 1 (bare only)", res.FeedsAdded)
	}
	if res.FeedsUpdated != 1 {
		t.Fatalf("feedsUpdated = %d want 1 (example.com moved+renamed)", res.FeedsUpdated)
	}
	if res.FeedsSkipped != 0 {
		t.Fatalf("feedsSkipped = %d want 0", res.FeedsSkipped)
	}

	got, err := lib.Feeds.Get(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}
	folders, err := lib.ListFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 {
		t.Fatalf("folders = %d want 1", len(folders))
	}
	if got.FolderID == nil || *got.FolderID != folders[0].ID {
		t.Fatalf("example folder = %v want %s", got.FolderID, folders[0].ID)
	}
	if got.Title != "Example" {
		t.Fatalf("example title = %q want Example", got.Title)
	}

	// User rename locks title — re-import must not overwrite it, but restores folder.
	if err := lib.RenameFeed(ctx, exampleID, "My Custom"); err != nil {
		t.Fatal(err)
	}
	if err := lib.MoveFeed(ctx, exampleID, nil); err != nil {
		t.Fatal(err)
	}
	res3, err := lib.ImportOPML(ctx, sampleOPML, false)
	if err != nil {
		t.Fatal(err)
	}
	if res3.FeedsUpdated < 1 {
		t.Fatalf("feedsUpdated = %d want >= 1 (folder re-sync)", res3.FeedsUpdated)
	}
	got, err = lib.Feeds.Get(ctx, exampleID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "My Custom" {
		t.Fatalf("user title overwritten: %q", got.Title)
	}
	if got.FolderID == nil || *got.FolderID != folders[0].ID {
		t.Fatalf("folder not restored from OPML: %v", got.FolderID)
	}
}

func TestLibrary_ImportOPML_ReusesFolderByName(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	// Pre-create folder with same name (different case) as OPML "News".
	pre, err := lib.CreateFolder(ctx, "news", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := lib.ImportOPML(ctx, sampleOPML, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.FoldersCreated != 0 {
		t.Fatalf("FoldersCreated = %d want 0 (case-insensitive reuse)", res.FoldersCreated)
	}
	folders, err := lib.ListFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 {
		t.Fatalf("folders = %d want 1", len(folders))
	}
	// Feed under OPML "News" should land in pre-existing folder.
	feeds, err := lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := range feeds {
		if strings.Contains(feeds[i].FeedURL, "example.com/feed") {
			if feeds[i].FolderID == nil || *feeds[i].FolderID != pre.ID {
				t.Fatalf("example feed folder = %v want %s", feeds[i].FolderID, pre.ID)
			}
			return
		}
	}
	t.Fatal("example feed not found")
}

func TestLibrary_ExportOPML_ContainsXMLURL(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	if _, err := lib.ImportOPML(ctx, sampleOPML, false); err != nil {
		t.Fatal(err)
	}

	xml, err := lib.ExportOPML(ctx)
	if err != nil {
		t.Fatalf("ExportOPML: %v", err)
	}
	if !strings.Contains(xml, `xmlUrl="https://example.com/feed.xml"`) &&
		!strings.Contains(xml, `xmlUrl="https://example.com/feed.xml"`) {
		// encoding/xml may emit attributes in any order; check substring
		if !strings.Contains(xml, "https://example.com/feed.xml") {
			t.Fatalf("export missing feed url:\n%s", xml)
		}
	}
	if !strings.Contains(xml, "https://bare.example/rss") {
		t.Fatalf("export missing bare feed:\n%s", xml)
	}
	if !strings.Contains(xml, "News") {
		t.Fatalf("export missing folder:\n%s", xml)
	}
	if !strings.Contains(xml, `version="2.0"`) {
		t.Fatalf("export not opml 2.0:\n%s", xml)
	}
}

func TestLibrary_ClearAllSubscriptions(t *testing.T) {
	database := openTestDB(t)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ctx := context.Background()

	if _, err := lib.ImportOPML(ctx, sampleOPML, false); err != nil {
		t.Fatal(err)
	}
	feeds, err := lib.ListFeeds(ctx)
	if err != nil || len(feeds) == 0 {
		t.Fatalf("expected feeds before clear: %v len=%d", err, len(feeds))
	}

	res, err := lib.ClearAllSubscriptions(ctx)
	if err != nil {
		t.Fatalf("ClearAllSubscriptions: %v", err)
	}
	if res.FeedsDeleted < 2 {
		t.Fatalf("feedsDeleted = %d want >= 2", res.FeedsDeleted)
	}
	if res.FoldersDeleted < 1 {
		t.Fatalf("foldersDeleted = %d want >= 1", res.FoldersDeleted)
	}

	feeds, err = lib.ListFeeds(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 0 {
		t.Fatalf("feeds after clear = %d want 0", len(feeds))
	}
	folders, err := lib.ListFolders(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(folders) != 0 {
		t.Fatalf("folders after clear = %d want 0", len(folders))
	}

	// Idempotent second clear
	res2, err := lib.ClearAllSubscriptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res2.FeedsDeleted != 0 || res2.FoldersDeleted != 0 {
		t.Fatalf("second clear = %+v want zeros", res2)
	}
}
