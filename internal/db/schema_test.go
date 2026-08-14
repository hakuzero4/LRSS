package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"lrss/internal/db"
)

func TestMigrate_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	// Re-open must be idempotent (no double-apply errors).
	if err := database.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	database, err = db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	tables := []string{
		"folders",
		"feeds",
		"articles",
		"settings",
		"article_embeddings",
		"jobs",
		"articles_fts",
		"schema_migrations",
		"ai_briefings",
		"ai_chat_sessions",
		"ai_chat_messages",
	}
	for _, name := range tables {
		var n int
		err := database.SQL.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','view') AND name = ?`,
			name,
		).Scan(&n)
		if err != nil {
			t.Fatalf("lookup %s: %v", name, err)
		}
		if n != 1 {
			t.Fatalf("expected table %s to exist, count=%d", name, n)
		}
	}

	var version int
	err = database.SQL.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`,
	).Scan(&version)
	if err != nil {
		t.Fatalf("schema version: %v", err)
	}
	if version < 1 {
		t.Fatalf("expected migration version >= 1, got %d", version)
	}
	var openedCol int
	if err := database.SQL.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info('articles') WHERE name = 'last_opened_at'`,
	).Scan(&openedCol); err != nil {
		t.Fatalf("last_opened_at column: %v", err)
	}
	if openedCol != 1 {
		t.Fatalf("expected articles.last_opened_at after migration 012, count=%d", openedCol)
	}

	// Smoke: insert folder → feed → article respects FKs.
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO folders (id, name, sort_order, created_at, updated_at)
		VALUES ('f1', 'Dev', 0, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert folder: %v", err)
	}
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO feeds (id, folder_id, title, feed_url, created_at, updated_at)
		VALUES ('feed1', 'f1', 'Example', 'https://example.com/feed.xml',
		        '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert feed: %v", err)
	}
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO articles (id, feed_id, guid, url, title, content_text, fetched_at)
		VALUES ('a1', 'feed1', 'g1', 'https://example.com/a1', 'Hello',
		        'Hello world', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert article: %v", err)
	}
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO articles_fts (article_id, title, summary, content_text)
		VALUES ('a1', 'Hello', '', 'Hello world')`)
	if err != nil {
		t.Fatalf("insert fts: %v", err)
	}
	_, err = database.SQL.ExecContext(ctx, `
		INSERT INTO article_embeddings (article_id, model, dimensions, status, created_at, updated_at)
		VALUES ('a1', '', 0, 'pending', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert embedding row: %v", err)
	}

	var title string
	err = database.SQL.QueryRowContext(ctx,
		`SELECT title FROM articles_fts WHERE articles_fts MATCH ?`, "Hello",
	).Scan(&title)
	if err != nil {
		t.Fatalf("fts query: %v", err)
	}
	if title != "Hello" {
		t.Fatalf("fts title = %q", title)
	}
}

func TestDefaultPath(t *testing.T) {
	path, err := db.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if path == "" {
		t.Fatal("empty path")
	}
	t.Logf("default db path: %s", path)
}
