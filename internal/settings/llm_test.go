package settings_test

import (
	"context"
	"path/filepath"
	"testing"

	"lrss/internal/db"
	"lrss/internal/settings"
)

func TestLLMConfig_RoundTrip(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	cfg := settings.LLMConfig{
		Provider:     settings.LLMProviderOpenAICompatible,
		BaseURL:      "https://api.openai.com/v1",
		APIKey:       "sk-secret-key",
		Model:        "gpt-4o-mini",
		Temperature:  0.4,
		MaxTokens:    1024,
		SystemPrompt: "You are a helpful RSS assistant.",
	}
	if err := store.SaveLLMConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadLLMConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsConfigured() {
		t.Fatal("expected configured")
	}
	if got.Model != "gpt-4o-mini" || got.APIKey != "sk-secret-key" {
		t.Fatalf("got %+v", got)
	}
	masked := got.Masked()
	if masked.APIKey == "sk-secret-key" || masked.APIKey == "" {
		t.Fatalf("mask failed: %q", masked.APIKey)
	}
}

func TestLLMConfig_Validate(t *testing.T) {
	bad := settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  "not-a-url",
		Model:    "m",
	}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected invalid url error")
	}
	ok := settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  "http://127.0.0.1:11434/v1",
		Model:    "llama3",
	}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
}
