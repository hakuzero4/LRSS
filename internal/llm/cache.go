package llm

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Cache persists feature results keyed by CacheKey.
type Cache struct {
	DB *sql.DB
}

// CachedResult is a stored generation.
type CachedResult struct {
	Key         string
	ArticleID   string
	Feature     string
	Model       string
	ContentHash string
	ResultMD    string
	MetaJSON    string
	CreatedAt   string
}

// Get returns a cached result or false if missing.
func (c *Cache) Get(ctx context.Context, key string) (CachedResult, bool, error) {
	if c == nil || c.DB == nil {
		return CachedResult{}, false, nil
	}
	var r CachedResult
	var articleID, meta sql.NullString
	err := c.DB.QueryRowContext(ctx, `
		SELECT cache_key, article_id, feature, model, content_hash, result_md, meta_json, created_at
		FROM llm_feature_cache WHERE cache_key = ?`, key).Scan(
		&r.Key, &articleID, &r.Feature, &r.Model, &r.ContentHash, &r.ResultMD, &meta, &r.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return CachedResult{}, false, nil
	}
	if err != nil {
		return CachedResult{}, false, fmt.Errorf("llm cache get: %w", err)
	}
	if articleID.Valid {
		r.ArticleID = articleID.String
	}
	if meta.Valid {
		r.MetaJSON = meta.String
	}
	return r, true, nil
}

// Clear deletes all cached feature results. Returns rows removed.
func (c *Cache) Clear(ctx context.Context) (int64, error) {
	if c == nil || c.DB == nil {
		return 0, nil
	}
	res, err := c.DB.ExecContext(ctx, `DELETE FROM llm_feature_cache`)
	if err != nil {
		return 0, fmt.Errorf("llm cache clear: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// Put upserts a cache row.
func (c *Cache) Put(ctx context.Context, r CachedResult) error {
	if c == nil || c.DB == nil {
		return nil
	}
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	var article any
	if r.ArticleID != "" {
		article = r.ArticleID
	}
	var meta any
	if r.MetaJSON != "" {
		meta = r.MetaJSON
	}
	_, err := c.DB.ExecContext(ctx, `
		INSERT INTO llm_feature_cache (
			cache_key, article_id, feature, model, content_hash, result_md, meta_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cache_key) DO UPDATE SET
			result_md = excluded.result_md,
			meta_json = excluded.meta_json,
			created_at = excluded.created_at`,
		r.Key, article, r.Feature, r.Model, r.ContentHash, r.ResultMD, meta, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("llm cache put: %w", err)
	}
	return nil
}
