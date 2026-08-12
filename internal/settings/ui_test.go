package settings_test

import (
	"context"
	"path/filepath"
	"testing"

	"lrss/internal/db"
	"lrss/internal/settings"
)

func TestUIPrefs_DefaultsAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)

	// Defaults when unset
	cfg, err := store.LoadUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	def := settings.DefaultUIPrefs()
	if cfg != def {
		t.Fatalf("defaults = %+v want %+v", cfg, def)
	}
	if cfg.KeepArticlesDays != 90 {
		t.Fatalf("KeepArticlesDays = %d want 90", cfg.KeepArticlesDays)
	}
	if !cfg.EnableKeyboardShortcuts || !cfg.MarkAsReadOnOpen {
		t.Fatal("expected keyboard shortcuts and markAsReadOnOpen true")
	}

	// Round-trip
	want := settings.UIPrefs{
		MarkAsReadOnOpen:        false,
		MarkAsReadOnScrollEnd:   true,
		OpenOnStartup:           "starred",
		HideReadOnStartup:       true,
		Theme:                   "dark",
		Accent:                  "teal",
		CompactSidebar:          true,
		FontSize:                "lg",
		ShowUnreadOnly:          true,
		OpenLinksInBrowser:      false,
		ReaderWidth:             "wide",
		DefaultFolderId:         "folder-1",
		FetchFullContent:        true,
		KeepArticlesDays:        30,
		HideDuplicateTitles:     false,
		BlockKeywords:           "spam,ads",
		EnableKeyboardShortcuts: false,
		NotifyOnNewArticles:     true,
		NotifySound:             true,
		HardwareAcceleration:    false,
		ClearCacheOnQuit:        true,
		DeveloperMode:           true,
	}
	if err := store.SaveUIPrefs(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round-trip = %+v want %+v", got, want)
	}
}

func TestUIPrefs_NormalizeLocale(t *testing.T) {
	got := (settings.UIPrefs{Locale: "en"}).Normalize().Locale
	if got != "en-US" {
		t.Fatalf("en → %q", got)
	}
	got = (settings.UIPrefs{Locale: "zh-Hans"}).Normalize().Locale
	if got != "zh-CN" {
		t.Fatalf("zh-Hans → %q", got)
	}
	got = (settings.UIPrefs{Locale: ""}).Normalize().Locale
	if got != "" {
		t.Fatalf("empty should stay empty, got %q", got)
	}
}

func TestUIPrefs_NormalizeKeepArticlesDays(t *testing.T) {
	c := settings.UIPrefs{KeepArticlesDays: 0}.Normalize()
	if c.KeepArticlesDays != 7 {
		t.Fatalf("low clamp = %d want 7", c.KeepArticlesDays)
	}
	c = settings.UIPrefs{KeepArticlesDays: 3}.Normalize()
	if c.KeepArticlesDays != 7 {
		t.Fatalf("low clamp = %d want 7", c.KeepArticlesDays)
	}
	c = settings.UIPrefs{KeepArticlesDays: 400}.Normalize()
	if c.KeepArticlesDays != 365 {
		t.Fatalf("high clamp = %d want 365", c.KeepArticlesDays)
	}
	c = settings.UIPrefs{KeepArticlesDays: 90}.Normalize()
	if c.KeepArticlesDays != 90 {
		t.Fatalf("in-range = %d want 90", c.KeepArticlesDays)
	}

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)

	if err := store.SetUIPrefs(ctx, settings.UIPrefs{KeepArticlesDays: 1, Theme: "system"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeepArticlesDays != 7 {
		t.Fatalf("persisted clamp = %d want 7", cfg.KeepArticlesDays)
	}
	if cfg.Theme != "system" {
		t.Fatalf("theme = %q want system", cfg.Theme)
	}
}

func TestUIPrefs_PartialJSONKeepsDefaults(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)

	// Write partial JSON (only theme + keep days).
	if err := store.Set(ctx, settings.KeyUIPrefs, `{"theme":"dark","keepArticlesDays":120}`); err != nil {
		t.Fatal(err)
	}
	cfg, err := store.LoadUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "dark" {
		t.Fatalf("theme = %q", cfg.Theme)
	}
	if cfg.KeepArticlesDays != 120 {
		t.Fatalf("days = %d", cfg.KeepArticlesDays)
	}
	// Fields absent from JSON should remain defaults (not zero).
	if !cfg.MarkAsReadOnOpen {
		t.Fatal("MarkAsReadOnOpen should stay default true")
	}
	if cfg.Accent != "purple" {
		t.Fatalf("Accent = %q want purple default", cfg.Accent)
	}
	if !cfg.EnableKeyboardShortcuts {
		t.Fatal("EnableKeyboardShortcuts should stay default true")
	}
	// Nested readerToolbar missing from partial JSON → all buttons visible.
	defTB := settings.DefaultReaderToolbarButtons()
	if cfg.ReaderToolbar != defTB {
		t.Fatalf("ReaderToolbar = %+v want default %+v", cfg.ReaderToolbar, defTB)
	}
}

func TestUIPrefs_ReaderToolbarRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)

	cfg := settings.DefaultUIPrefs()
	cfg.ReaderToolbar.Summarize = false
	cfg.ReaderToolbar.Translate = false
	cfg.ReaderToolbar.AI = false
	if err := store.SaveUIPrefs(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ReaderToolbar.Summarize || got.ReaderToolbar.Translate || got.ReaderToolbar.AI {
		t.Fatalf("hidden buttons still true: %+v", got.ReaderToolbar)
	}
	if !got.ReaderToolbar.Read || !got.ReaderToolbar.FetchFull || !got.ReaderToolbar.OpenOriginal {
		t.Fatalf("visible buttons lost: %+v", got.ReaderToolbar)
	}
	if got.ReaderToolbar.Zen || got.ReaderToolbar.Star {
		t.Fatalf("default-hidden buttons should stay false when not flipped: %+v", got.ReaderToolbar)
	}
}
