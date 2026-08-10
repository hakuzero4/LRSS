package search

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

// FTSHit is a full-text search result.
type FTSHit struct {
	ArticleID string
	Title     string
	Rank      float64
	Snippet   string
}

// SanitizeFTS turns user input into a safe FTS5 query (OR of quoted tokens).
func SanitizeFTS(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	var tokens []string
	var b strings.Builder
	flush := func() {
		t := strings.TrimSpace(b.String())
		b.Reset()
		if t == "" {
			return
		}
		t = strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r > 127 {
				return r
			}
			return -1
		}, t)
		if t != "" {
			tokens = append(tokens, `"`+t+`"`)
		}
	}
	for _, r := range q {
		if unicode.IsSpace(r) {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return strings.Join(tokens, " OR ")
}

// SearchFTS runs FTS5 MATCH; falls back to LIKE if FTS fails or empty.
func SearchFTS(ctx context.Context, db *sql.DB, query string, limit int) ([]FTSHit, error) {
	if limit <= 0 {
		limit = 50
	}
	match := SanitizeFTS(query)
	if match == "" {
		return nil, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT article_id, title, bm25(articles_fts) AS rank,
		       snippet(articles_fts, 1, '', '', '…', 12) AS snip
		FROM articles_fts
		WHERE articles_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, match, limit)
	if err != nil {
		return searchLike(ctx, db, query, limit)
	}
	defer rows.Close()

	var hits []FTSHit
	for rows.Next() {
		var h FTSHit
		if err := rows.Scan(&h.ArticleID, &h.Title, &h.Rank, &h.Snippet); err != nil {
			return nil, err
		}
		hits = append(hits, h)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return searchLike(ctx, db, query, limit)
	}
	return hits, nil
}

func searchLike(ctx context.Context, db *sql.DB, query string, limit int) ([]FTSHit, error) {
	q := "%" + strings.TrimSpace(query) + "%"
	if q == "%%" {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, 0.0, COALESCE(summary, '')
		FROM articles
		WHERE title LIKE ? OR IFNULL(summary,'') LIKE ? OR IFNULL(content_text,'') LIKE ?
		ORDER BY published_at DESC
		LIMIT ?`, q, q, q, limit)
	if err != nil {
		return nil, fmt.Errorf("like search: %w", err)
	}
	defer rows.Close()
	var hits []FTSHit
	rank := 0.0
	for rows.Next() {
		var h FTSHit
		if err := rows.Scan(&h.ArticleID, &h.Title, &h.Rank, &h.Snippet); err != nil {
			return nil, err
		}
		h.Rank = rank
		rank++
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// UpsertFTS inserts or replaces the FTS document for an article.
func UpsertFTS(ctx context.Context, db *sql.DB, articleID, title, summary, contentText string) error {
	_, _ = db.ExecContext(ctx, `DELETE FROM articles_fts WHERE article_id = ?`, articleID)
	_, err := db.ExecContext(ctx,
		`INSERT INTO articles_fts (article_id, title, summary, content_text) VALUES (?, ?, ?, ?)`,
		articleID, title, summary, contentText)
	return err
}

// DeleteFTS removes an article from the FTS index.
func DeleteFTS(ctx context.Context, db *sql.DB, articleID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM articles_fts WHERE article_id = ?`, articleID)
	return err
}
