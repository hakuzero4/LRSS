package search_test

import (
	"context"
	"path/filepath"
	"testing"

	"lrss/internal/db"
	"lrss/internal/embed"
	"lrss/internal/job"
	"lrss/internal/search"
	"lrss/internal/settings"
	"lrss/internal/vector"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func seedArticles(t *testing.T, database *db.DB) {
	t.Helper()
	ctx := context.Background()
	_, err := database.SQL.ExecContext(ctx, `
		INSERT INTO feeds (id, title, feed_url, created_at, updated_at)
		VALUES ('f1', 'Feed', 'https://ex.com/rss', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatal(err)
	}
	articles := []struct {
		id, title, body string
	}{
		{"a1", "Go performance profiling", "pprof and benchmarks for Go services"},
		{"a2", "Vue component design", "composition API and props patterns"},
		{"a3", "SQLite vector search", "embedding nearest neighbor in SQLite"},
	}
	for _, a := range articles {
		_, err := database.SQL.ExecContext(ctx, `
			INSERT INTO articles (id, feed_id, guid, url, title, summary, content_text, fetched_at)
			VALUES (?, 'f1', ?, ?, ?, ?, ?, '2026-01-01T00:00:00Z')`,
			a.id, a.id, "https://ex.com/"+a.id, a.title, a.body, a.body)
		if err != nil {
			t.Fatal(err)
		}
		if err := search.UpsertFTS(ctx, database.SQL, a.id, a.title, a.body, a.body); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSearch_FTSOnlyWhenEmbeddingDisabled(t *testing.T) {
	database := openTestDB(t)
	seedArticles(t, database)
	store := settings.NewStore(database.SQL)
	svc := search.New(database.SQL, store)

	res, err := svc.Search(context.Background(), "Vue component", search.Options{Mode: "auto", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.ModeUsed != settings.SearchModeFTS {
		t.Fatalf("mode = %s want fts", res.ModeUsed)
	}
	if len(res.Hits) == 0 {
		t.Fatal("expected hits")
	}
	if res.Hits[0].ArticleID != "a2" && res.Hits[0].Title == "" {
		t.Fatalf("unexpected hit %+v", res.Hits[0])
	}
}

func TestSearch_VectorWithFakeProvider(t *testing.T) {
	database := openTestDB(t)
	seedArticles(t, database)
	ctx := context.Background()
	store := settings.NewStore(database.SQL)

	cfg := settings.EmbeddingConfig{
		Provider:   settings.ProviderOpenAICompatible,
		BaseURL:    "http://127.0.0.1:9", // unused — we inject Fake
		Model:      "fake",
		Dimensions: 32,
		BatchSize:  8,
	}
	if err := store.SaveEmbeddingConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	fake := embed.NewFake(32)
	idx := vector.NewIndex(database.SQL)
	// Collect first — MaxOpenConns=1 cannot Exec while Rows is open.
	type row struct{ id, title, body string }
	var list []row
	rows, err := database.SQL.Query(`SELECT id, title, IFNULL(content_text,'') FROM articles`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.title, &r.body); err != nil {
			t.Fatal(err)
		}
		list = append(list, r)
	}
	_ = rows.Close()
	for _, r := range list {
		input := embed.BuildInput(r.title, "", r.body)
		vec := fake.MustEmbed(input)
		if err := idx.UpsertReady(ctx, r.id, "fake", embed.ContentHash(input), 32, vec); err != nil {
			t.Fatal(err)
		}
	}

	svc := search.New(database.SQL, store)
	svc.NewProvider = func(c settings.EmbeddingConfig) (embed.Provider, error) {
		return fake, nil
	}

	// FakeProvider is hash-based: query identical to article input ranks first.
	query := embed.BuildInput("SQLite vector search", "", "embedding nearest neighbor in SQLite")
	res, err := svc.Search(ctx, query, search.Options{Mode: "vector", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if res.ModeUsed != settings.SearchModeVector {
		t.Fatalf("mode=%s", res.ModeUsed)
	}
	if len(res.Hits) == 0 {
		t.Fatal("no vector hits")
	}
	if res.Hits[0].ArticleID != "a3" {
		t.Fatalf("top hit %s want a3 (hits=%+v)", res.Hits[0].ArticleID, res.Hits)
	}
}

func TestEmbedWorker_RunOnce(t *testing.T) {
	database := openTestDB(t)
	seedArticles(t, database)
	ctx := context.Background()
	store := settings.NewStore(database.SQL)
	cfg := settings.EmbeddingConfig{
		Provider:   settings.ProviderOpenAICompatible,
		BaseURL:    "http://127.0.0.1:9",
		Model:      "fake",
		Dimensions: 32,
	}
	if err := store.SaveEmbeddingConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}

	w := job.NewEmbedWorker(database.SQL, store)
	fake := embed.NewFake(32)
	w.NewProvider = func(settings.EmbeddingConfig) (embed.Provider, error) { return fake, nil }

	n, err := w.RunOnce(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("processed %d want 3", n)
	}
	var ready int
	_ = database.SQL.QueryRow(`SELECT COUNT(*) FROM article_embeddings WHERE status='ready'`).Scan(&ready)
	if ready != 3 {
		t.Fatalf("ready=%d", ready)
	}
}

func TestSettings_Persistence(t *testing.T) {
	database := openTestDB(t)
	store := settings.NewStore(database.SQL)
	ctx := context.Background()
	cfg := settings.EmbeddingConfig{
		Provider:   settings.ProviderOpenAICompatible,
		BaseURL:    "https://api.example.com/v1",
		APIKey:     "sk-test",
		Model:      "text-emb",
		Dimensions: 384,
	}
	if err := store.SaveEmbeddingConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadEmbeddingConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsConfigured() || got.Model != "text-emb" || got.Dimensions != 384 {
		t.Fatalf("%+v", got)
	}
}
