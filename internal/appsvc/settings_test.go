package appsvc_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lrss/internal/appsvc"
	"lrss/internal/db"
	"lrss/internal/job"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"
)

func TestSettingsService_UIPrefsAndPurge(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	searchSvc := search.New(database.SQL, store)
	embedWorker := job.NewEmbedWorker(database.SQL, store)
	settingsAPI := appsvc.NewSettings(store, searchSvc, embedWorker)

	// Defaults
	prefs, err := settingsAPI.GetUIPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if prefs.KeepArticlesDays != 90 || prefs.Theme != "light" {
		t.Fatalf("defaults = %+v", prefs)
	}

	// Without library, purge should fail clearly.
	if _, err := settingsAPI.PurgeOldArticles(); err == nil {
		t.Fatal("expected error without library")
	}

	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	settingsAPI.SetLibrary(lib)

	// Seed feed + old/new/starred articles via repo.
	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/s6"}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	oldPub := time.Now().UTC().Add(-200 * 24 * time.Hour).Format(time.RFC3339)
	recentPub := time.Now().UTC().Add(-1 * 24 * time.Hour).Format(time.RFC3339)
	_, err = repos.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{GUID: "old", URL: "https://ex.com/old", Title: "Old", PublishedAt: &oldPub},
		{GUID: "star", URL: "https://ex.com/star", Title: "Star", PublishedAt: &oldPub},
		{GUID: "new", URL: "https://ex.com/new", Title: "New", PublishedAt: &recentPub},
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := repos.Articles.List(ctx, "all", repo.ListOpts{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	var oldID, starID string
	for _, a := range all {
		switch a.Title {
		case "Star":
			starID = a.ID
			if err := repos.Articles.SetStarred(ctx, a.ID, true); err != nil {
				t.Fatal(err)
			}
		case "Old":
			oldID = a.ID
		}
	}
	// Retention requires BOTH publish and fetch age; backdate fetch for truly-old rows.
	oldFetch := time.Now().UTC().Add(-120 * 24 * time.Hour).Format(time.RFC3339)
	if _, err := database.SQL.ExecContext(ctx,
		`UPDATE articles SET fetched_at = ? WHERE id IN (?, ?)`, oldFetch, oldID, starID,
	); err != nil {
		t.Fatal(err)
	}

	// Save keep days = 90, then purge via API.
	prefs.KeepArticlesDays = 90
	prefs.Theme = "dark"
	if err := settingsAPI.SetUIPrefs(prefs); err != nil {
		t.Fatal(err)
	}
	got, err := settingsAPI.GetUIPrefs()
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "dark" || got.KeepArticlesDays != 90 {
		t.Fatalf("after set = %+v", got)
	}

	res, err := settingsAPI.PurgeOldArticles()
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 1 {
		t.Fatalf("deleted = %d want 1", res.Deleted)
	}

	left, err := repos.Articles.List(ctx, "all", repo.ListOpts{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 2 {
		t.Fatalf("left = %d want 2", len(left))
	}
}

func TestSettingsService_OnUIPrefs(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	searchSvc := search.New(database.SQL, store)
	embedWorker := job.NewEmbedWorker(database.SQL, store)
	settingsAPI := appsvc.NewSettings(store, searchSvc, embedWorker)

	var seen settings.UIPrefs
	var n int
	settingsAPI.SetOnUIPrefs(func(p settings.UIPrefs) {
		n++
		seen = p
	})
	prefs, err := settingsAPI.GetUIPrefs()
	if err != nil {
		t.Fatal(err)
	}
	prefs.MicaBackdrop = false
	if err := settingsAPI.SetUIPrefs(prefs); err != nil {
		t.Fatal(err)
	}
	if n != 1 || seen.MicaBackdrop {
		t.Fatalf("hook n=%d mica=%v", n, seen.MicaBackdrop)
	}
}

func TestSettingsService_PruneRecentOnSetUIPrefs(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	searchSvc := search.New(database.SQL, store)
	embedWorker := job.NewEmbedWorker(database.SQL, store)
	settingsAPI := appsvc.NewSettings(store, searchSvc, embedWorker)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	settingsAPI.SetLibrary(lib)

	feed := &model.Feed{Title: "F", FeedURL: "https://ex.com/recent-limit"}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	items := make([]repo.ParsedItem, 12)
	for i := range items {
		title := "R" + time.Now().UTC().Format("150405") + string(rune('a'+i))
		items[i] = repo.ParsedItem{
			GUID: title, URL: "https://ex.com/" + title, Title: title, PublishedAt: &now,
		}
	}
	if _, err := repos.Articles.UpsertFromParsed(ctx, feed.ID, items); err != nil {
		t.Fatal(err)
	}
	all, err := repos.Articles.List(ctx, "all", repo.ListOpts{Limit: 20})
	if err != nil || len(all) != 12 {
		t.Fatalf("seed: %v n=%d", err, len(all))
	}
	for _, a := range all {
		if err := lib.RecordOpened(ctx, a.ID, 50); err != nil {
			t.Fatal(err)
		}
	}

	prefs, err := settingsAPI.GetUIPrefs()
	if err != nil {
		t.Fatal(err)
	}
	prefs.RecentReadLimit = 10
	if err := settingsAPI.SetUIPrefs(prefs); err != nil {
		t.Fatal(err)
	}
	recent, err := repos.Articles.List(ctx, "recent", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 10 {
		t.Fatalf("after slider prune recent=%d want 10", len(recent))
	}
}
