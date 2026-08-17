package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	keepSourceManual = "manual"
	keepSourceFilter = "filter"
)

// Keep upserts an article_keeps row.
// source is "manual" or "filter" (anything else, including empty, becomes filter).
// A later filter keep does not overwrite an existing manual row.
// folder_id is NULL on insert and is never overwritten on conflict.
func (r *ArticleRepo) Keep(ctx context.Context, articleID, reason, source string, confidence float64, topics []string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("keep: article id required")
	}
	src := strings.ToLower(strings.TrimSpace(source))
	if src != keepSourceManual {
		src = keepSourceFilter
	}
	if topics == nil {
		topics = []string{}
	}
	raw, err := json.Marshal(topics)
	if err != nil {
		return fmt.Errorf("keep topics: %w", err)
	}
	_, err = r.DB.ExecContext(ctx, `
		INSERT INTO article_keeps (article_id, reason, confidence, topics, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(article_id) DO UPDATE SET
			reason = excluded.reason,
			confidence = excluded.confidence,
			topics = excluded.topics,
			source = excluded.source
		WHERE excluded.source = 'manual' OR article_keeps.source != 'manual'`,
		articleID, strings.TrimSpace(reason), confidence, string(raw), src, nowUTC(),
	)
	if err != nil {
		return fmt.Errorf("keep article: %w", err)
	}
	return nil
}

// Unkeep deletes the article_keeps row (no-op if missing).
func (r *ArticleRepo) Unkeep(ctx context.Context, articleID string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("unkeep: article id required")
	}
	_, err := r.DB.ExecContext(ctx, `DELETE FROM article_keeps WHERE article_id = ?`, articleID)
	if err != nil {
		return fmt.Errorf("unkeep article: %w", err)
	}
	return nil
}

// IsKept reports whether article_id has a keep row.
func (r *ArticleRepo) IsKept(ctx context.Context, articleID string) (bool, error) {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return false, nil
	}
	var n int
	err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM article_keeps WHERE article_id = ?`, articleID,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("is kept: %w", err)
	}
	return n > 0, nil
}

// CountKeeps returns the total number of article_keeps rows.
func (r *ArticleRepo) CountKeeps(ctx context.Context) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM article_keeps`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count keeps: %w", err)
	}
	return n, nil
}

// SetKeepFolder assigns a kept article to a 精选 folder.
// Empty folderID moves the article to the virtual root (folder_id NULL).
// The article must already have an article_keeps row.
func (r *ArticleRepo) SetKeepFolder(ctx context.Context, articleID, folderID string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("set keep folder: article id required")
	}
	folderID = strings.TrimSpace(folderID)

	var n int
	if err := r.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM article_keeps WHERE article_id = ?`, articleID,
	).Scan(&n); err != nil {
		return fmt.Errorf("set keep folder: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("set keep folder: article is not kept")
	}

	var folderArg any
	if folderID != "" {
		var exists int
		if err := r.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM keep_folders WHERE id = ?`, folderID,
		).Scan(&exists); err != nil {
			return fmt.Errorf("set keep folder: %w", err)
		}
		if exists == 0 {
			return fmt.Errorf("set keep folder: folder not found: %s", folderID)
		}
		folderArg = folderID
	}

	_, err := r.DB.ExecContext(ctx,
		`UPDATE article_keeps SET folder_id = ? WHERE article_id = ?`,
		folderArg, articleID,
	)
	if err != nil {
		return fmt.Errorf("set keep folder: %w", err)
	}
	return nil
}

// CountKeptUnreadByFolder returns unread kept-article counts keyed by folder id.
// Key "" is the 精选 root (folder_id IS NULL). Counts that folder only, not descendants.
func (r *ArticleRepo) CountKeptUnreadByFolder(ctx context.Context, excludeNsfw bool) (map[string]int, error) {
	nsfw := ""
	if excludeNsfw {
		nsfw = " AND " + nsfwKeepExcludeSQL
	}
	rows, err := r.DB.QueryContext(ctx, `
		SELECT IFNULL(k.folder_id, ''), COUNT(*)
		FROM article_keeps k
		JOIN articles a ON a.id = k.article_id
		WHERE a.is_read = 0`+nsfw+`
		GROUP BY k.folder_id`)
	if err != nil {
		return nil, fmt.Errorf("count kept unread by folder: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var folderID string
		var n int
		if err := rows.Scan(&folderID, &n); err != nil {
			return nil, fmt.Errorf("count kept unread by folder: %w", err)
		}
		out[folderID] = n
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("count kept unread by folder: %w", err)
	}
	return out, nil
}
