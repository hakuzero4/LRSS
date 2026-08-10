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

	// Second import: all skip
	res2, err := lib.ImportOPML(ctx, sampleOPML, false)
	if err != nil {
		t.Fatalf("ImportOPML 2: %v", err)
	}
	if res2.FeedsSkipped != 2 {
		t.Fatalf("second import skipped = %d want 2", res2.FeedsSkipped)
	}
	if res2.FeedsAdded != 0 {
		t.Fatalf("second import added = %d want 0", res2.FeedsAdded)
	}
	// Folder create always creates (duplicate names OK)
	if res2.FoldersCreated != 1 {
		t.Fatalf("second foldersCreated = %d want 1", res2.FoldersCreated)
	}
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
