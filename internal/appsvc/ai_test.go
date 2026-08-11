package appsvc_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"lrss/internal/appsvc"
	"lrss/internal/db"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/service"
	"lrss/internal/settings"
)

func TestAIService_SummarizeViaHTTP(t *testing.T) {
	const body = `<?xml version="1.0"?><rss version="2.0"><channel>
<title>AI Feed</title><link>https://ex.test/</link>
<item><title>Hello AI</title><link>https://ex.test/1</link><guid>g1</guid>
<description>Short partial summary only.</description></item>
</channel></rss>`
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(feedSrv.Close)

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "## Summary\n- hello"}},
			},
		})
	}))
	t.Cleanup(llmSrv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	if err := store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  llmSrv.URL + "/v1",
		Model:    "m",
		APIKey:   "k",
	}); err != nil {
		t.Fatal(err)
	}
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	feed, err := lib.AddFeed(ctx, feedSrv.URL+"/rss", nil)
	if err != nil {
		t.Fatal(err)
	}
	arts, err := lib.ListArticles(ctx, "feed:"+feed.ID, 10, 0, false)
	if err != nil || len(arts) == 0 {
		t.Fatalf("arts: %v %#v", err, arts)
	}

	ai := appsvc.NewAI(store, lib, database.SQL)
	ok, err := ai.IsLLMConfigured()
	if err != nil || !ok {
		t.Fatalf("configured: %v %v", ok, err)
	}
	res, err := ai.Summarize(arts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Markdown, "Summary") {
		t.Fatalf("md = %q", res.Markdown)
	}
	// cache hit
	res2, err := ai.Summarize(arts[0].ID)
	if err != nil || !res2.Cached {
		t.Fatalf("cache: %+v err=%v", res2, err)
	}
}

func TestAIService_DailyDigestEmpty(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "e.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	_ = store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  "http://127.0.0.1:9/v1",
		Model:    "m",
	})
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	ai := appsvc.NewAI(store, lib, database.SQL)
	_, err = ai.DailyDigest(5)
	if err == nil {
		t.Fatal("expected no articles error")
	}
}
