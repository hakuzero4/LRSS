package settings_test

import (
	"context"
	"path/filepath"
	"testing"

	"lrss/internal/db"
	"lrss/internal/settings"
)

func TestLibraryConfig_DefaultsAndClamp(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)

	// Defaults when unset
	cfg, err := store.LoadLibraryConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutoRefresh {
		t.Fatal("default AutoRefresh want true")
	}
	if cfg.RefreshIntervalMinutes != 30 {
		t.Fatalf("default interval = %d want 30", cfg.RefreshIntervalMinutes)
	}

	// Clamp low
	if err := store.SaveLibraryConfig(ctx, settings.LibraryConfig{
		AutoRefresh:            false,
		RefreshIntervalMinutes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	cfg, err = store.LoadLibraryConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AutoRefresh {
		t.Fatal("want AutoRefresh false")
	}
	if cfg.RefreshIntervalMinutes != 5 {
		t.Fatalf("clamped low interval = %d want 5", cfg.RefreshIntervalMinutes)
	}

	// Clamp high
	if err := store.SaveLibraryConfig(ctx, settings.LibraryConfig{
		AutoRefresh:            true,
		RefreshIntervalMinutes: 999,
	}); err != nil {
		t.Fatal(err)
	}
	cfg, err = store.GetLibraryConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutoRefresh {
		t.Fatal("want AutoRefresh true")
	}
	if cfg.RefreshIntervalMinutes != 180 {
		t.Fatalf("clamped high interval = %d want 180", cfg.RefreshIntervalMinutes)
	}

	// In-range
	if err := store.SetLibraryConfig(ctx, settings.LibraryConfig{
		AutoRefresh:            true,
		RefreshIntervalMinutes: 45,
	}); err != nil {
		t.Fatal(err)
	}
	cfg, err = store.LoadLibraryConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.RefreshIntervalMinutes != 45 {
		t.Fatalf("interval = %d want 45", cfg.RefreshIntervalMinutes)
	}
}

func TestLibraryConfig_Normalize(t *testing.T) {
	c := settings.LibraryConfig{RefreshIntervalMinutes: 0}.Normalize()
	if c.RefreshIntervalMinutes != 5 {
		t.Fatalf("got %d", c.RefreshIntervalMinutes)
	}
	c = settings.LibraryConfig{RefreshIntervalMinutes: 200}.Normalize()
	if c.RefreshIntervalMinutes != 180 {
		t.Fatalf("got %d", c.RefreshIntervalMinutes)
	}
	c = settings.LibraryConfig{RefreshIntervalMinutes: 60}.Normalize()
	if c.RefreshIntervalMinutes != 60 {
		t.Fatalf("got %d", c.RefreshIntervalMinutes)
	}
}
