package job

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"lrss/internal/embed"
	"lrss/internal/settings"
	"lrss/internal/vector"
)

// EmbedWorker processes pending article embeddings.
type EmbedWorker struct {
	SQL         *sql.DB
	Settings    *settings.Store
	Index       *vector.Index
	NewProvider func(settings.EmbeddingConfig) (embed.Provider, error)
}

// NewEmbedWorker constructs a worker.
func NewEmbedWorker(sqlDB *sql.DB, store *settings.Store) *EmbedWorker {
	return &EmbedWorker{
		SQL:         sqlDB,
		Settings:    store,
		Index:       vector.NewIndex(sqlDB),
		NewProvider: embed.NewProvider,
	}
}

// RunOnce embeds up to limit pending articles. Returns processed count.
func (w *EmbedWorker) RunOnce(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 16
	}
	cfg, err := w.Settings.LoadEmbeddingConfig(ctx)
	if err != nil {
		return 0, err
	}
	if !cfg.IsConfigured() {
		return 0, nil
	}
	p, err := w.NewProvider(cfg)
	if err != nil {
		return 0, err
	}

	rows, err := w.SQL.QueryContext(ctx, `
		SELECT a.id, a.title, IFNULL(a.summary,''), IFNULL(a.content_text,'')
		FROM articles a
		LEFT JOIN article_embeddings e ON e.article_id = a.id
		WHERE e.article_id IS NULL OR e.status IN ('pending', 'error')
		LIMIT ?`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type item struct {
		id, title, summary, body string
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.title, &it.summary, &it.body); err != nil {
			return 0, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	texts := make([]string, len(items))
	hashes := make([]string, len(items))
	for i, it := range items {
		texts[i] = embed.BuildInput(it.title, it.summary, it.body)
		hashes[i] = embed.ContentHash(texts[i])
	}

	vecs, err := p.Embed(ctx, texts)
	if err != nil {
		return 0, err
	}
	if len(vecs) != len(items) {
		return 0, fmt.Errorf("embed count mismatch")
	}

	for i, it := range items {
		if err := w.Index.UpsertReady(ctx, it.id, cfg.Model, hashes[i], cfg.Dimensions, vecs[i]); err != nil {
			now := time.Now().UTC().Format(time.RFC3339Nano)
			msg := err.Error()
			_, _ = w.SQL.ExecContext(ctx, `
				INSERT INTO article_embeddings (article_id, model, dimensions, status, error, created_at, updated_at)
				VALUES (?, ?, ?, 'error', ?, ?, ?)
				ON CONFLICT(article_id) DO UPDATE SET status='error', error=excluded.error, updated_at=excluded.updated_at`,
				it.id, cfg.Model, cfg.Dimensions, msg, now, now)
			continue
		}
	}

	if _, err := w.Index.Quantize(ctx); err != nil {
		// non-fatal when extension missing
		_ = err
	}
	return len(items), nil
}

// EnqueueAllPending ensures every article without ready embedding is pending.
func (w *EmbedWorker) EnqueueAllPending(ctx context.Context) error {
	rows, err := w.SQL.QueryContext(ctx, `SELECT id FROM articles`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if err := w.Index.MarkPending(ctx, id); err != nil {
			return err
		}
	}
	return rows.Err()
}
