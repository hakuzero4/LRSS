package llm_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"lrss/internal/db"
	"lrss/internal/llm"
)

func TestBudgetText(t *testing.T) {
	short := llm.BudgetText("hello", 100)
	if short != "hello" {
		t.Fatalf("short = %q", short)
	}
	long := strings.Repeat("字", 50)
	got := llm.BudgetText(long, 10)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis: %q", got)
	}
	if llm.BudgetText("", 10) != "" {
		t.Fatal("empty")
	}
}

func TestContentFingerprintAndCacheKeyStable(t *testing.T) {
	a := llm.ArticleInput{Title: "T", Summary: "S", Body: "B", URL: "https://x"}
	h1 := llm.ContentFingerprint(a)
	h2 := llm.ContentFingerprint(a)
	if h1 != h2 || h1 == "" {
		t.Fatalf("hash unstable %q %q", h1, h2)
	}
	a2 := a
	a2.Body = "B2"
	if llm.ContentFingerprint(a2) == h1 {
		t.Fatal("body change should change hash")
	}
	k1 := llm.CacheKey("id1", llm.FeatureSummarize, "m", h1, "")
	k2 := llm.CacheKey("id1", llm.FeatureSummarize, "m", h1, "")
	if k1 != k2 {
		t.Fatal("cache key unstable")
	}
	if llm.CacheKey("id1", llm.FeatureTranslate, "m", h1, "en") == k1 {
		t.Fatal("feature/extra should change key")
	}
}

func TestBuildArticleBundle(t *testing.T) {
	bundle := llm.BuildArticleBundle(llm.ArticleInput{
		Title:   "Hello",
		Summary: "Sum",
		Body:    "Body text here",
		URL:     "https://example.com",
	}, 500)
	if !strings.Contains(bundle, "Hello") || !strings.Contains(bundle, "Body text") {
		t.Fatalf("bundle = %q", bundle)
	}
}

func TestLocalSuggestTags(t *testing.T) {
	tags := llm.LocalSuggestTags("Rust and WebAssembly #wasm guide", "intro to systems", 8)
	if len(tags) == 0 {
		t.Fatal("expected tags")
	}
	joined := strings.Join(tags, " ")
	if !strings.Contains(joined, "wasm") {
		t.Fatalf("tags = %v", tags)
	}
}

func TestCache_GetPut(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "c.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	c := &llm.Cache{DB: database.SQL}
	key := llm.CacheKey("a1", llm.FeatureSummarize, "gpt", "hash1", "")
	_, ok, err := c.Get(ctx, key)
	if err != nil || ok {
		t.Fatalf("empty get: ok=%v err=%v", ok, err)
	}
	if err := c.Put(ctx, llm.CachedResult{
		Key: key, ArticleID: "a1", Feature: llm.FeatureSummarize,
		Model: "gpt", ContentHash: "hash1", ResultMD: "## Hi\n",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err := c.Get(ctx, key)
	if err != nil || !ok {
		t.Fatalf("hit: ok=%v err=%v", ok, err)
	}
	if got.ResultMD != "## Hi\n" {
		t.Fatalf("result = %q", got.ResultMD)
	}
	// Second put overwrites
	if err := c.Put(ctx, llm.CachedResult{
		Key: key, ArticleID: "a1", Feature: llm.FeatureSummarize,
		Model: "gpt", ContentHash: "hash1", ResultMD: "## Bye\n",
	}); err != nil {
		t.Fatal(err)
	}
	got, ok, err = c.Get(ctx, key)
	if err != nil || !ok || got.ResultMD != "## Bye\n" {
		t.Fatalf("overwrite failed: %+v ok=%v err=%v", got, ok, err)
	}
}

func TestCache_NoDB(t *testing.T) {
	c := &llm.Cache{DB: (*sql.DB)(nil)}
	_, ok, err := c.Get(context.Background(), "x")
	if err != nil || ok {
		t.Fatal("nil db should miss without error")
	}
}
