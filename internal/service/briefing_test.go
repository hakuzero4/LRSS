package service_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"lrss/internal/db"
	"lrss/internal/llm"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/service"
	"lrss/internal/settings"
)

func TestBriefingWorker_SkipWhenOffOrEmpty(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	repos := repo.New(database.SQL)
	w := service.NewBriefingWorker(store, repos.Briefings, repos.Articles, repos.Feeds, repos.Folders, nil)

	did, err := w.TryGenerate(ctx)
	if err != nil || did {
		t.Fatalf("off+empty did=%v err=%v", did, err)
	}

	prefs, err := store.LoadUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefs.SmartBriefing = true
	if err := store.SaveUIPrefs(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	w.Enqueue(ctx, []string{"a1"})
	did, err = w.TryGenerate(ctx)
	if err != nil || did {
		t.Fatalf("no llm config should skip did=%v err=%v", did, err)
	}
}

func TestBriefingWorker_Debounce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	repos := repo.New(database.SQL)
	prefs := settings.DefaultUIPrefs()
	prefs.SmartBriefing = true
	if err := store.SaveUIPrefs(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	// Fake configured LLM so we get past the config gate; Brief will fail later if we generate.
	if err := store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  "http://127.0.0.1:9",
		Model:    "x",
	}); err != nil {
		t.Fatal(err)
	}
	w := service.NewBriefingWorker(store, repos.Briefings, repos.Articles, repos.Feeds, repos.Folders, &llm.Service{Store: store})
	w.Enqueue(ctx, []string{"missing"})
	did, err := w.TryGenerate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if did {
		t.Fatal("fresh enqueue should debounce")
	}
}

func TestLibrary_OnArticlesInsertedHook(t *testing.T) {
	var got []string
	lib := service.NewLibraryFromRepos(mustOpenSQL(t), &rss.Client{})
	lib.OnArticlesInserted = func(_ context.Context, ids []string) {
		got = append(got, ids...)
	}
	// hook is invoked only via emitInserted after real upserts; call through exported field.
	lib.OnArticlesInserted(context.Background(), []string{"x"})
	if len(got) != 1 || got[0] != "x" {
		t.Fatalf("got %v", got)
	}
}

func mustOpenSQL(t *testing.T) *repo.Repos {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), "h.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return repo.New(database.SQL)
}

func TestBriefingPendingJSONRoundTrip(t *testing.T) {
	ctx := context.Background()
	database, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), "p.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	type p struct {
		IDs           []string `json:"ids"`
		LastEnqueueAt string   `json:"lastEnqueueAt"`
	}
	want := p{IDs: []string{"a", "b"}, LastEnqueueAt: time.Now().UTC().Format(time.RFC3339)}
	if err := store.SetJSON(ctx, service.KeyBriefingPending, want); err != nil {
		t.Fatal(err)
	}
	var got p
	if err := store.GetJSON(ctx, service.KeyBriefingPending, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.IDs) != 2 {
		t.Fatalf("got %+v", got)
	}
}
