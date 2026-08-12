package settings_test

import (
	"context"
	"path/filepath"
	"testing"

	"lrss/internal/db"
	"lrss/internal/settings"
)

func TestWebAccessConfig_DefaultsAndRoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)

	cfg, err := store.LoadWebAccessConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	def := settings.DefaultWebAccessConfig()
	if cfg != def {
		t.Fatalf("defaults = %+v want %+v", cfg, def)
	}

	want := settings.WebAccessConfig{
		Enabled: true,
		Bind:    "lan",
		Port:    19000,
		Token:   "secret-token-abc",
	}
	saved, err := store.SaveWebAccessConfig(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Token != want.Token {
		t.Fatalf("token = %q want %q", saved.Token, want.Token)
	}
	got, err := store.GetWebAccessConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round-trip = %+v want %+v", got, want)
	}
}

func TestWebAccessConfig_Normalize(t *testing.T) {
	c := settings.WebAccessConfig{Bind: "LAN", Port: 80, Token: "  x  "}.Normalize()
	if c.Bind != "lan" {
		t.Fatalf("bind = %q", c.Bind)
	}
	if c.Port != settings.DefaultWebPort {
		t.Fatalf("port low clamp = %d", c.Port)
	}
	if c.Token != "x" {
		t.Fatalf("token trim = %q", c.Token)
	}

	c2 := settings.WebAccessConfig{Bind: "weird", Port: 70000}.Normalize()
	if c2.Bind != "localhost" || c2.Port != settings.DefaultWebPort {
		t.Fatalf("normalize invalid = %+v", c2)
	}
}

func TestWebAccessConfig_EnsureTokenForLAN(t *testing.T) {
	cfg := settings.WebAccessConfig{Enabled: true, Bind: "lan", Port: 18765}
	out, gen := cfg.EnsureTokenForLAN()
	if !gen || out.Token == "" || len(out.Token) != 64 {
		t.Fatalf("expected generated 64-char token, got gen=%v tok=%q", gen, out.Token)
	}

	// localhost does not force token
	local := settings.WebAccessConfig{Enabled: true, Bind: "localhost"}
	out2, gen2 := local.EnsureTokenForLAN()
	if gen2 || out2.Token != "" {
		t.Fatalf("localhost should not generate token: gen=%v tok=%q", gen2, out2.Token)
	}
}

func TestWebAccessConfig_SaveLANGeneratesToken(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)

	saved, err := store.SaveWebAccessConfig(ctx, settings.WebAccessConfig{
		Enabled: true,
		Bind:    "lan",
		Port:    18765,
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Token == "" {
		t.Fatal("expected auto token for LAN enable")
	}
}
