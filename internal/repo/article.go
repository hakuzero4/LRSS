package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"lrss/internal/id"
	"lrss/internal/model"
	"lrss/internal/search"
	"lrss/internal/vector"
)

// ArticleRepo persists articles and keeps FTS / embed queue in sync.
type ArticleRepo struct {
	DB               *sql.DB
	vec              *vector.Index
	embeddingEnabled EmbeddingEnabledFunc
}

// ArticleRepoOpts configures ArticleRepo hooks.
type ArticleRepoOpts struct {
	EmbeddingEnabled EmbeddingEnabledFunc
	Vector           *vector.Index
}

// NewArticleRepo constructs an article repository.
func NewArticleRepo(db *sql.DB, opts ...ArticleRepoOpts) *ArticleRepo {
	r := &ArticleRepo{DB: db}
	if len(opts) > 0 {
		r.embeddingEnabled = opts[0].EmbeddingEnabled
		r.vec = opts[0].Vector
	}
	if r.vec == nil {
		r.vec = vector.NewIndex(db)
	}
	return r
}

// ListOpts controls article listing.
type ListOpts struct {
	Limit      int
	Offset     int
	Query      string // optional title/summary/content LIKE; empty skips
	UnreadOnly bool
	// ExcludeNsfw hides articles from feeds with is_nsfw=1 (office mode).
	ExcludeNsfw bool
	// Lite omits content_html / content_text / translation_raw so list pages
	// stay small. Get still returns the full body.
	Lite bool
}

// ParsedItem is a feed item ready for upsert (from the RSS parse layer).
type ParsedItem struct {
	GUID        string
	URL         string
	Title       string
	Author      *string
	Summary     *string
	ContentHTML *string
	ContentText *string
	ImageURL    *string
	PublishedAt *string
}

// UpsertResult summarizes UpsertFromParsed.
type UpsertResult struct {
	Inserted int
	Skipped  int
	// InsertedIDs are new article ids (same order as successful inserts).
	// Used for post-refresh full-content queueing without re-scanning the feed.
	InsertedIDs []string
}

// SmartCounts is the sidebar badge totals (independent of the current list page).
type SmartCounts struct {
	Unread  int `json:"unread"`
	Today   int `json:"today"`
	Starred int `json:"starred"`
	All     int `json:"all"`
	Recent  int `json:"recent"`
	// Kept is the unread count of articles in article_keeps (sidebar badge).
	Kept int `json:"kept"`
}

// nsfwFeedExcludeSQL is AND-ed when ExcludeNsfw / CountSmart hideNsfw.
// Hides articles from feeds marked is_nsfw OR feeds in folders marked is_nsfw.
const nsfwFeedExcludeSQL = `feed_id NOT IN (
	SELECT f.id FROM feeds f
	LEFT JOIN folders fo ON fo.id = f.folder_id
	WHERE f.is_nsfw = 1 OR IFNULL(fo.is_nsfw, 0) = 1
)`

// nsfwKeepExcludeSQL is the same office-mode hide, qualified for article_keeps joins.
const nsfwKeepExcludeSQL = `a.feed_id NOT IN (
	SELECT f.id FROM feeds f
	LEFT JOIN folders fo ON fo.id = f.folder_id
	WHERE f.is_nsfw = 1 OR IFNULL(fo.is_nsfw, 0) = 1
)`

// articleKeepSelect is LEFT JOIN columns scanned after full_content_fetched.
const articleKeepSelect = `CASE WHEN k.article_id IS NOT NULL THEN 1 ELSE 0 END,
		       IFNULL(k.reason, ''),
		       IFNULL(k.confidence, 0),
		       IFNULL(k.source, ''),
		       IFNULL(k.folder_id, '')`

// CountSmart returns true totals for smart lists (not capped by list limit).
// "today" uses the same UTC day window as collectionWhere("today").
// excludeNsfw=true omits articles belonging to is_nsfw feeds (office mode).
func (r *ArticleRepo) CountSmart(ctx context.Context, excludeNsfw bool) (SmartCounts, error) {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	end := start.Add(24 * time.Hour)
	nsfw := ""
	nsfwKept := ""
	if excludeNsfw {
		nsfw = " AND " + nsfwFeedExcludeSQL
		nsfwKept = " AND " + nsfwKeepExcludeSQL
	}
	var c SmartCounts
	err := r.DB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM articles WHERE is_read = 0`+nsfw+`),
			(SELECT COUNT(*) FROM articles
			  WHERE COALESCE(published_at, fetched_at) >= ?
			    AND COALESCE(published_at, fetched_at) < ?`+nsfw+`),
			(SELECT COUNT(*) FROM articles WHERE is_starred = 1`+nsfw+`),
			(SELECT COUNT(*) FROM articles WHERE 1=1`+nsfw+`),
			(SELECT COUNT(*) FROM articles WHERE last_opened_at IS NOT NULL`+nsfw+`),
			(SELECT COUNT(*) FROM article_keeps k
			  JOIN articles a ON a.id = k.article_id
			  WHERE a.is_read = 0`+nsfwKept+`)`,
		start.Format(time.RFC3339), end.Format(time.RFC3339),
	).Scan(&c.Unread, &c.Today, &c.Starred, &c.All, &c.Recent, &c.Kept)
	if err != nil {
		return SmartCounts{}, fmt.Errorf("count smart: %w", err)
	}
	return c, nil
}

// List returns articles for a collection filter with optional opts.
// collection: unread | today | starred | all | recent | kept | kept:<id> | feed:<id> | folder:<id>
func (r *ArticleRepo) List(ctx context.Context, collection string, opts ListOpts) ([]model.Article, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}

	where, args, err := collectionWhere(collection)
	if err != nil {
		return nil, err
	}
	if opts.UnreadOnly {
		where = append(where, `is_read = 0`)
	}
	if opts.ExcludeNsfw {
		where = append(where, nsfwFeedExcludeSQL)
	}
	q := strings.TrimSpace(opts.Query)
	if q != "" {
		like := "%" + q + "%"
		where = append(where, `(title LIKE ? OR IFNULL(summary,'') LIKE ? OR IFNULL(content_text,'') LIKE ?)`)
		args = append(args, like, like, like)
	}

	bodyCols := `content_html, content_text, translation_raw, translation_lang`
	if opts.Lite {
		bodyCols = `NULL, NULL, NULL, translation_lang`
	}
	sqlStr := `
		SELECT id, feed_id, guid, url, title, author, summary,
		       ` + bodyCols + `,
		       image_url, published_at,
		       fetched_at, is_read, is_starred, full_content_fetched,
		       ` + articleKeepSelect + `
		FROM articles
		LEFT JOIN article_keeps k ON k.article_id = articles.id`
	if len(where) > 0 {
		sqlStr += ` WHERE ` + strings.Join(where, ` AND `)
	}
	if strings.TrimSpace(collection) == "recent" {
		sqlStr += ` ORDER BY last_opened_at DESC, id DESC LIMIT ? OFFSET ?`
	} else {
		sqlStr += ` ORDER BY COALESCE(published_at, fetched_at) DESC, id DESC LIMIT ? OFFSET ?`
	}
	args = append(args, opts.Limit, opts.Offset)

	rows, err := r.DB.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("list articles: %w", err)
	}
	defer rows.Close()

	var out []model.Article
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.Article{}
	}
	return out, nil
}

// Get loads one article by id.
func (r *ArticleRepo) Get(ctx context.Context, articleID string) (model.Article, error) {
	row := r.DB.QueryRowContext(ctx, `
		SELECT id, feed_id, guid, url, title, author, summary,
		       content_html, content_text, translation_raw, translation_lang,
		       image_url, published_at,
		       fetched_at, is_read, is_starred, full_content_fetched,
		       `+articleKeepSelect+`
		FROM articles
		LEFT JOIN article_keeps k ON k.article_id = articles.id
		WHERE articles.id = ?`, articleID)
	a, err := scanArticle(row)
	if err == sql.ErrNoRows {
		return model.Article{}, fmt.Errorf("article not found: %s", articleID)
	}
	if err != nil {
		return model.Article{}, err
	}
	return a, nil
}

// CountByFeed returns how many articles belong to feedID.
func (r *ArticleRepo) CountByFeed(ctx context.Context, feedID string) (int, error) {
	var n int
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM articles WHERE feed_id = ?`, feedID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count articles by feed: %w", err)
	}
	return n, nil
}

// UpsertFromParsed batch-inserts new articles by (feed_id, guid).
// Existing rows are skipped (is_read / is_starred never overwritten).
// On insert: UpsertFTS; if embeddingEnabled → MarkPending.
func (r *ArticleRepo) UpsertFromParsed(ctx context.Context, feedID string, items []ParsedItem) (UpsertResult, error) {
	var res UpsertResult
	if feedID == "" {
		return res, fmt.Errorf("upsert articles: feed_id required")
	}
	fetchedAt := nowUTC()

	for _, item := range items {
		guid := strings.TrimSpace(item.GUID)
		if guid == "" {
			guid = strings.TrimSpace(item.URL)
		}
		if guid == "" {
			res.Skipped++
			continue
		}
		url := strings.TrimSpace(item.URL)
		if url == "" {
			url = guid
		}
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = url
		}

		articleID := id.New()
		result, err := r.DB.ExecContext(ctx, `
			INSERT INTO articles (
				id, feed_id, guid, url, title, author, summary,
				content_html, content_text, image_url, published_at, fetched_at,
				is_read, is_starred
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)
			ON CONFLICT(feed_id, guid) DO NOTHING`,
			articleID, feedID, guid, url, title,
			nullStr(item.Author), nullStr(item.Summary),
			nullStr(item.ContentHTML), nullStr(item.ContentText), nullStr(item.ImageURL),
			nullStr(item.PublishedAt), fetchedAt,
		)
		if err != nil {
			return res, fmt.Errorf("upsert article: %w", err)
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			res.Skipped++
			continue
		}
		res.Inserted++
		res.InsertedIDs = append(res.InsertedIDs, articleID)

		summary := ""
		if item.Summary != nil {
			summary = *item.Summary
		}
		contentText := ""
		if item.ContentText != nil {
			contentText = *item.ContentText
		}
		if err := search.UpsertFTS(ctx, r.DB, articleID, title, summary, contentText); err != nil {
			return res, fmt.Errorf("upsert fts: %w", err)
		}

		if r.embeddingEnabled != nil && r.embeddingEnabled(ctx) && r.vec != nil {
			if err := r.vec.MarkPending(ctx, articleID); err != nil {
				return res, fmt.Errorf("mark pending embed: %w", err)
			}
		}
	}
	return res, nil
}

// UpdateTranslation stores bilingual translation next to the original body (never overwrites content_*).
func (r *ArticleRepo) UpdateTranslation(ctx context.Context, articleID, raw, lang string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("update translation: article id required")
	}
	raw = strings.TrimSpace(raw)
	lang = strings.TrimSpace(lang)
	res, err := r.DB.ExecContext(ctx, `
		UPDATE articles SET translation_raw = ?, translation_lang = ? WHERE id = ?`,
		nullStr(&raw), nullStr(&lang), articleID,
	)
	if err != nil {
		return fmt.Errorf("update translation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}
	return nil
}

// ClearTranslation removes stored bilingual translation (original body unchanged).
func (r *ArticleRepo) ClearTranslation(ctx context.Context, articleID string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("clear translation: article id required")
	}
	res, err := r.DB.ExecContext(ctx, `
		UPDATE articles SET translation_raw = NULL, translation_lang = NULL WHERE id = ?`, articleID)
	if err != nil {
		return fmt.Errorf("clear translation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}
	return nil
}

// UpdateSummary replaces the article summary (e.g. after AI summarize) and refreshes FTS.
func (r *ArticleRepo) UpdateSummary(ctx context.Context, articleID, summary string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("update summary: article id required")
	}
	summary = strings.TrimSpace(summary)
	res, err := r.DB.ExecContext(ctx, `
		UPDATE articles SET summary = ? WHERE id = ?`,
		nullStr(&summary), articleID,
	)
	if err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}
	var title, contentText sql.NullString
	err = r.DB.QueryRowContext(ctx,
		`SELECT title, content_text FROM articles WHERE id = ?`, articleID,
	).Scan(&title, &contentText)
	if err != nil {
		return fmt.Errorf("update summary fts load: %w", err)
	}
	if err := search.UpsertFTS(ctx, r.DB, articleID, title.String, summary, contentText.String); err != nil {
		return fmt.Errorf("update summary fts: %w", err)
	}
	if r.embeddingEnabled != nil && r.embeddingEnabled(ctx) && r.vec != nil {
		if err := r.vec.MarkPending(ctx, articleID); err != nil {
			return fmt.Errorf("update summary mark pending: %w", err)
		}
	}
	return nil
}

// UpdateContent replaces content_html / content_text (e.g. after full-text fetch)
// and refreshes FTS. Marks full_content_fetched so auto-fetch will not re-run.
// Clears translation_raw/lang so a stale bilingual overlay cannot cover the new body.
// Optionally marks embedding pending when enabled.
func (r *ArticleRepo) UpdateContent(ctx context.Context, articleID, contentHTML, contentText string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("update content: article id required")
	}
	res, err := r.DB.ExecContext(ctx, `
		UPDATE articles
		SET content_html = ?, content_text = ?, fetched_at = ?, full_content_fetched = 1,
		    translation_raw = NULL, translation_lang = NULL
		WHERE id = ?`,
		nullStr(&contentHTML), nullStr(&contentText), nowUTC(), articleID,
	)
	if err != nil {
		return fmt.Errorf("update content: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}

	// Reload title/summary for FTS document.
	var title, summary sql.NullString
	err = r.DB.QueryRowContext(ctx,
		`SELECT title, summary FROM articles WHERE id = ?`, articleID,
	).Scan(&title, &summary)
	if err != nil {
		return fmt.Errorf("update content fts load: %w", err)
	}
	if err := search.UpsertFTS(ctx, r.DB, articleID, title.String, summary.String, contentText); err != nil {
		return fmt.Errorf("update content fts: %w", err)
	}
	if r.embeddingEnabled != nil && r.embeddingEnabled(ctx) && r.vec != nil {
		if err := r.vec.MarkPending(ctx, articleID); err != nil {
			return fmt.Errorf("update content mark pending: %w", err)
		}
	}
	return nil
}

func clampRecentKeep(keep int) int {
	if keep <= 0 {
		keep = 50
	}
	if keep < 10 {
		keep = 10
	}
	if keep > 200 {
		keep = 200
	}
	return keep
}

// RecordOpened stamps last_opened_at and prunes older recently-read rows to keep entries.
func (r *ArticleRepo) RecordOpened(ctx context.Context, articleID string, keep int) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("record opened: article id required")
	}
	res, err := r.DB.ExecContext(ctx,
		`UPDATE articles SET last_opened_at = ? WHERE id = ?`,
		nowUTC(), articleID,
	)
	if err != nil {
		return fmt.Errorf("record opened: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}
	return r.PruneOpened(ctx, keep)
}

// PruneOpened clears last_opened_at on rows beyond the keep window.
// MaxOpenConns=1: consume the keep-id query before the UPDATE.
func (r *ArticleRepo) PruneOpened(ctx context.Context, keep int) error {
	keep = clampRecentKeep(keep)
	rows, err := r.DB.QueryContext(ctx, `
		SELECT id FROM articles
		WHERE last_opened_at IS NOT NULL
		ORDER BY last_opened_at DESC, id DESC
		LIMIT ?`, keep)
	if err != nil {
		return fmt.Errorf("prune opened: %w", err)
	}
	keepIDs := make([]string, 0, keep)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("prune opened: %w", err)
		}
		keepIDs = append(keepIDs, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("prune opened: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("prune opened: %w", err)
	}
	if len(keepIDs) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(keepIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(keepIDs))
	for i, id := range keepIDs {
		args[i] = id
	}
	_, err = r.DB.ExecContext(ctx, `
		UPDATE articles SET last_opened_at = NULL
		WHERE last_opened_at IS NOT NULL
		  AND id NOT IN (`+placeholders+`)`, args...)
	if err != nil {
		return fmt.Errorf("record opened prune: %w", err)
	}
	return nil
}

// SetRead updates is_read.
func (r *ArticleRepo) SetRead(ctx context.Context, articleID string, read bool) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE articles SET is_read = ? WHERE id = ?`, boolToInt(read), articleID)
	if err != nil {
		return fmt.Errorf("set read: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}
	return nil
}

// SetStarred updates is_starred.
func (r *ArticleRepo) SetStarred(ctx context.Context, articleID string, starred bool) error {
	res, err := r.DB.ExecContext(ctx, `UPDATE articles SET is_starred = ? WHERE id = ?`, boolToInt(starred), articleID)
	if err != nil {
		return fmt.Errorf("set starred: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}
	return nil
}

// MarkAllRead marks matching collection articles as read.
// excludeNsfw skips articles from is_nsfw feeds (office mode smart lists).
func (r *ArticleRepo) MarkAllRead(ctx context.Context, collection string, excludeNsfw bool) error {
	where, args, err := collectionWhere(collection)
	if err != nil {
		return err
	}
	where = append(where, `is_read = 0`)
	if excludeNsfw {
		where = append(where, nsfwFeedExcludeSQL)
	}
	sqlStr := `UPDATE articles SET is_read = 1 WHERE ` + strings.Join(where, ` AND `)
	_, err = r.DB.ExecContext(ctx, sqlStr, args...)
	if err != nil {
		return fmt.Errorf("mark all read: %w", err)
	}
	return nil
}

// Delete removes one article and its FTS row (embeddings cascade via FK).
func (r *ArticleRepo) Delete(ctx context.Context, articleID string) error {
	if err := search.DeleteFTS(ctx, r.DB, articleID); err != nil {
		return fmt.Errorf("delete article fts: %w", err)
	}
	res, err := r.DB.ExecContext(ctx, `DELETE FROM articles WHERE id = ?`, articleID)
	if err != nil {
		return fmt.Errorf("delete article: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("article not found: %s", articleID)
	}
	return nil
}

// PurgeOlderThan deletes non-starred articles that are old by BOTH publish time and
// local fetch time. Starred articles are always kept.
//
// Per-feed keep_articles_days overrides the global default when set in [7, 365];
// 0 (or out of range) uses globalDays. Using both timestamps avoids wiping a
// just-subscribed feed whose items are all older than the retention window by
// published_at alone. After purge, empty feeds re-download on refresh.
//
// globalDays is clamped to [7, 365]. Returns the number of articles deleted.
// Fully consumes the ID select before further Exec (MaxOpenConns=1).
func (r *ArticleRepo) PurgeOlderThan(ctx context.Context, globalDays int) (int, error) {
	if globalDays < 7 {
		globalDays = 7
	}
	if globalDays > 365 {
		globalDays = 365
	}

	// Collect IDs fully before any further DB ops on this connection.
	// Effective days: feed override when set, else global default.
	// julianday diff avoids building dynamic "now','-N days'" modifiers.
	rows, err := r.DB.QueryContext(ctx, `
		SELECT a.id
		FROM articles a
		JOIN feeds f ON f.id = a.feed_id
		WHERE a.is_starred = 0
		  AND (julianday('now') - julianday(date(COALESCE(a.published_at, a.fetched_at)))) >
		      CASE
		        WHEN f.keep_articles_days >= 7 AND f.keep_articles_days <= 365
		          THEN f.keep_articles_days
		        ELSE ?
		      END
		  AND (julianday('now') - julianday(date(a.fetched_at))) >
		      CASE
		        WHEN f.keep_articles_days >= 7 AND f.keep_articles_days <= 365
		          THEN f.keep_articles_days
		        ELSE ?
		      END`, globalDays, globalDays)
	if err != nil {
		return 0, fmt.Errorf("purge select: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("purge scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, fmt.Errorf("purge rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("purge close: %w", err)
	}
	if len(ids) == 0 {
		return 0, nil
	}

	// Bulk FTS clear, then articles (embeddings ON DELETE CASCADE).
	// Chunk to avoid huge IN lists on very large purges.
	const chunk = 200
	deleted := 0
	for i := 0; i < len(ids); i += chunk {
		end := i + chunk
		if end > len(ids) {
			end = len(ids)
		}
		part := ids[i:end]
		placeholders := make([]string, len(part))
		args := make([]any, len(part))
		for j, id := range part {
			placeholders[j] = "?"
			args[j] = id
		}
		inList := strings.Join(placeholders, ",")

		if _, err := r.DB.ExecContext(ctx,
			`DELETE FROM articles_fts WHERE article_id IN (`+inList+`)`, args...); err != nil {
			return deleted, fmt.Errorf("purge fts: %w", err)
		}
		res, err := r.DB.ExecContext(ctx,
			`DELETE FROM articles WHERE id IN (`+inList+`)`, args...)
		if err != nil {
			return deleted, fmt.Errorf("purge articles: %w", err)
		}
		n, _ := res.RowsAffected()
		deleted += int(n)
	}
	// Feeds left with zero articles must not keep ETags — otherwise the next
	// refresh 304s and never re-imports (archive RSS + retention race).
	if deleted > 0 {
		_, _ = r.DB.ExecContext(ctx, `
			UPDATE feeds SET etag = NULL, last_modified = NULL, updated_at = ?
			WHERE id IN (
				SELECT f.id FROM feeds f
				WHERE NOT EXISTS (SELECT 1 FROM articles a WHERE a.feed_id = f.id)
			)`, nowUTC())
	}
	return deleted, nil
}

func collectionWhere(collection string) (where []string, args []any, err error) {
	c := strings.TrimSpace(collection)
	if c == "" {
		c = "all"
	}
	switch {
	case c == "all":
		// no filter
	case c == "unread":
		where = append(where, `is_read = 0`)
	case c == "starred":
		where = append(where, `is_starred = 1`)
	case c == "today":
		start := time.Now().UTC().Truncate(24 * time.Hour)
		end := start.Add(24 * time.Hour)
		where = append(where, `COALESCE(published_at, fetched_at) >= ? AND COALESCE(published_at, fetched_at) < ?`)
		args = append(args, start.Format(time.RFC3339), end.Format(time.RFC3339))
	case c == "recent":
		where = append(where, `last_opened_at IS NOT NULL`)
	case c == "kept":
		where = append(where, `id IN (SELECT article_id FROM article_keeps)`)
	case strings.HasPrefix(c, "kept:"):
		folderID := strings.TrimPrefix(c, "kept:")
		if folderID == "" {
			return nil, nil, fmt.Errorf("invalid collection %q", collection)
		}
		where = append(where, `articles.id IN (SELECT article_id FROM article_keeps WHERE folder_id = ?)`)
		args = append(args, folderID)
	case strings.HasPrefix(c, "feed:"):
		feedID := strings.TrimPrefix(c, "feed:")
		if feedID == "" {
			return nil, nil, fmt.Errorf("invalid collection %q", collection)
		}
		where = append(where, `feed_id = ?`)
		args = append(args, feedID)
	case strings.HasPrefix(c, "folder:"):
		folderID := strings.TrimPrefix(c, "folder:")
		if folderID == "" {
			return nil, nil, fmt.Errorf("invalid collection %q", collection)
		}
		where = append(where, `feed_id IN (SELECT id FROM feeds WHERE folder_id = ?)`)
		args = append(args, folderID)
	default:
		return nil, nil, fmt.Errorf("unknown collection %q", collection)
	}
	return where, args, nil
}

func scanArticle(row scannable) (model.Article, error) {
	var a model.Article
	var guid, author, summary, contentHTML, contentText, translationRaw, translationLang sql.NullString
	var imageURL, published sql.NullString
	var read, starred, fullFetched, kept int
	var keepReason, keepSource, keepFolderID string
	var keepConf float64
	if err := row.Scan(
		&a.ID, &a.FeedID, &guid, &a.URL, &a.Title, &author, &summary,
		&contentHTML, &contentText, &translationRaw, &translationLang,
		&imageURL, &published,
		&a.FetchedAt, &read, &starred, &fullFetched,
		&kept, &keepReason, &keepConf, &keepSource, &keepFolderID,
	); err != nil {
		return model.Article{}, err
	}
	a.GUID = strPtr(guid)
	a.Author = strPtr(author)
	a.Summary = strPtr(summary)
	a.ContentHTML = strPtr(contentHTML)
	a.ContentText = strPtr(contentText)
	a.TranslationRaw = strPtr(translationRaw)
	a.TranslationLang = strPtr(translationLang)
	a.ImageURL = strPtr(imageURL)
	a.PublishedAt = strPtr(published)
	a.IsRead = read != 0
	a.IsStarred = starred != 0
	a.FullContentFetched = fullFetched != 0
	if kept != 0 {
		a.IsKept = true
		a.KeepReason = keepReason
		a.KeepConfidence = keepConf
		a.KeepSource = keepSource
		a.KeepFolderID = keepFolderID
	}
	return a, nil
}
