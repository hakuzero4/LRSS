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
}

// List returns articles for a collection filter with optional opts.
// collection: unread | today | starred | all | feed:<id> | folder:<id>
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
	q := strings.TrimSpace(opts.Query)
	if q != "" {
		like := "%" + q + "%"
		where = append(where, `(title LIKE ? OR IFNULL(summary,'') LIKE ? OR IFNULL(content_text,'') LIKE ?)`)
		args = append(args, like, like, like)
	}

	sqlStr := `
		SELECT id, feed_id, guid, url, title, author, summary,
		       content_html, content_text, image_url, published_at,
		       fetched_at, is_read, is_starred
		FROM articles`
	if len(where) > 0 {
		sqlStr += ` WHERE ` + strings.Join(where, ` AND `)
	}
	sqlStr += ` ORDER BY COALESCE(published_at, fetched_at) DESC, id DESC LIMIT ? OFFSET ?`
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
		       content_html, content_text, image_url, published_at,
		       fetched_at, is_read, is_starred
		FROM articles WHERE id = ?`, articleID)
	a, err := scanArticle(row)
	if err == sql.ErrNoRows {
		return model.Article{}, fmt.Errorf("article not found: %s", articleID)
	}
	if err != nil {
		return model.Article{}, err
	}
	return a, nil
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
func (r *ArticleRepo) MarkAllRead(ctx context.Context, collection string) error {
	where, args, err := collectionWhere(collection)
	if err != nil {
		return err
	}
	where = append(where, `is_read = 0`)
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
	var guid, author, summary, contentHTML, contentText, imageURL, published sql.NullString
	var read, starred int
	if err := row.Scan(
		&a.ID, &a.FeedID, &guid, &a.URL, &a.Title, &author, &summary,
		&contentHTML, &contentText, &imageURL, &published,
		&a.FetchedAt, &read, &starred,
	); err != nil {
		return model.Article{}, err
	}
	a.GUID = strPtr(guid)
	a.Author = strPtr(author)
	a.Summary = strPtr(summary)
	a.ContentHTML = strPtr(contentHTML)
	a.ContentText = strPtr(contentText)
	a.ImageURL = strPtr(imageURL)
	a.PublishedAt = strPtr(published)
	a.IsRead = read != 0
	a.IsStarred = starred != 0
	return a, nil
}
