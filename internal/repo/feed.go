package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"lrss/internal/id"
	"lrss/internal/model"
	"lrss/internal/search"
)

// FeedRepo persists feeds.
type FeedRepo struct {
	DB *sql.DB
}

// NewFeedRepo constructs a feed repository.
func NewFeedRepo(db *sql.DB) *FeedRepo {
	return &FeedRepo{DB: db}
}

const feedSelect = `
	SELECT f.id, f.folder_id, f.title, f.site_url, f.feed_url, f.favicon_url,
	       f.etag, f.last_modified, f.last_fetched_at, f.last_error, f.is_paused,
	       f.created_at, f.updated_at,
	       (SELECT COUNT(*) FROM articles a WHERE a.feed_id = f.id AND a.is_read = 0) AS unread
	FROM feeds f`

// List returns all feeds with UnreadCount.
func (r *FeedRepo) List(ctx context.Context) ([]model.Feed, error) {
	rows, err := r.DB.QueryContext(ctx, feedSelect+`
		ORDER BY f.title COLLATE NOCASE ASC`)
	if err != nil {
		return nil, fmt.Errorf("list feeds: %w", err)
	}
	defer rows.Close()

	var out []model.Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.Feed{}
	}
	return out, nil
}

// ListActive returns non-paused feeds (for refresh-all).
func (r *FeedRepo) ListActive(ctx context.Context) ([]model.Feed, error) {
	rows, err := r.DB.QueryContext(ctx, feedSelect+`
		WHERE f.is_paused = 0
		ORDER BY f.title COLLATE NOCASE ASC`)
	if err != nil {
		return nil, fmt.Errorf("list active feeds: %w", err)
	}
	defer rows.Close()

	var out []model.Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []model.Feed{}
	}
	return out, nil
}

// Get loads one feed by id (with unread count).
func (r *FeedRepo) Get(ctx context.Context, feedID string) (model.Feed, error) {
	row := r.DB.QueryRowContext(ctx, feedSelect+` WHERE f.id = ?`, feedID)
	f, err := scanFeed(row)
	if err == sql.ErrNoRows {
		return model.Feed{}, fmt.Errorf("feed not found: %s", feedID)
	}
	if err != nil {
		return model.Feed{}, err
	}
	return f, nil
}

// GetByURL loads a feed by feed_url.
func (r *FeedRepo) GetByURL(ctx context.Context, feedURL string) (model.Feed, error) {
	row := r.DB.QueryRowContext(ctx, feedSelect+` WHERE f.feed_url = ?`, feedURL)
	f, err := scanFeed(row)
	if err == sql.ErrNoRows {
		return model.Feed{}, sql.ErrNoRows
	}
	if err != nil {
		return model.Feed{}, err
	}
	return f, nil
}

// Insert creates a feed row. Empty ID/timestamps are filled in on the value returned via pointer.
func (r *FeedRepo) Insert(ctx context.Context, f *model.Feed) error {
	if f == nil {
		return fmt.Errorf("insert feed: nil")
	}
	if strings.TrimSpace(f.FeedURL) == "" {
		return fmt.Errorf("insert feed: feed_url required")
	}
	if f.ID == "" {
		f.ID = id.New()
	}
	now := nowUTC()
	if f.CreatedAt == "" {
		f.CreatedAt = now
	}
	if f.UpdatedAt == "" {
		f.UpdatedAt = now
	}
	if f.Title == "" {
		f.Title = f.FeedURL
	}
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO feeds (
			id, folder_id, title, site_url, feed_url, favicon_url,
			etag, last_modified, last_fetched_at, last_error, is_paused,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, nullStr(f.FolderID), f.Title, nullStr(f.SiteURL), f.FeedURL, nullStr(f.FaviconURL),
		nullStr(f.ETag), nullStr(f.LastModified), nullStr(f.LastFetchedAt), nullStr(f.LastError),
		boolToInt(f.IsPaused), f.CreatedAt, f.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert feed: %w", err)
	}
	return nil
}

// UpdateAfterFetch writes etag, last_modified, last_fetched_at, last_error, and optional title.
func (r *FeedRepo) UpdateAfterFetch(ctx context.Context, feedID string, title string, etag, lastModified, lastError *string) error {
	now := nowUTC()
	if title != "" {
		_, err := r.DB.ExecContext(ctx, `
			UPDATE feeds SET
				title = ?,
				etag = ?,
				last_modified = ?,
				last_fetched_at = ?,
				last_error = ?,
				updated_at = ?
			WHERE id = ?`,
			title, nullStr(etag), nullStr(lastModified), now, nullStr(lastError), now, feedID,
		)
		if err != nil {
			return fmt.Errorf("update feed after fetch: %w", err)
		}
		return nil
	}
	_, err := r.DB.ExecContext(ctx, `
		UPDATE feeds SET
			etag = ?,
			last_modified = ?,
			last_fetched_at = ?,
			last_error = ?,
			updated_at = ?
		WHERE id = ?`,
		nullStr(etag), nullStr(lastModified), now, nullStr(lastError), now, feedID,
	)
	if err != nil {
		return fmt.Errorf("update feed after fetch: %w", err)
	}
	return nil
}

// SetPaused sets is_paused on a feed.
func (r *FeedRepo) SetPaused(ctx context.Context, feedID string, paused bool) error {
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE feeds SET is_paused = ?, updated_at = ? WHERE id = ?`,
		boolToInt(paused), now, feedID)
	if err != nil {
		return fmt.Errorf("set paused: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feed not found: %s", feedID)
	}
	return nil
}

// Delete removes a feed. Articles cascade via FK; FTS rows are deleted first
// (articles_fts is application-synced, not FK-linked).
func (r *FeedRepo) Delete(ctx context.Context, feedID string) error {
	// Collect IDs fully before any further Exec (MaxOpenConns=1).
	rows, err := r.DB.QueryContext(ctx, `SELECT id FROM articles WHERE feed_id = ?`, feedID)
	if err != nil {
		return fmt.Errorf("delete feed list articles: %w", err)
	}
	var articleIDs []string
	for rows.Next() {
		var aid string
		if err := rows.Scan(&aid); err != nil {
			_ = rows.Close()
			return err
		}
		articleIDs = append(articleIDs, aid)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, aid := range articleIDs {
		if err := search.DeleteFTS(ctx, r.DB, aid); err != nil {
			return fmt.Errorf("delete fts %s: %w", aid, err)
		}
	}

	res, err := r.DB.ExecContext(ctx, `DELETE FROM feeds WHERE id = ?`, feedID)
	if err != nil {
		return fmt.Errorf("delete feed: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feed not found: %s", feedID)
	}
	return nil
}

func scanFeed(row scannable) (model.Feed, error) {
	var f model.Feed
	var folder, site, fav, etag, lastMod, lastFetch, lastErr sql.NullString
	var paused int
	if err := row.Scan(
		&f.ID, &folder, &f.Title, &site, &f.FeedURL, &fav,
		&etag, &lastMod, &lastFetch, &lastErr, &paused,
		&f.CreatedAt, &f.UpdatedAt, &f.UnreadCount,
	); err != nil {
		return model.Feed{}, err
	}
	f.FolderID = strPtr(folder)
	f.SiteURL = strPtr(site)
	f.FaviconURL = strPtr(fav)
	f.ETag = strPtr(etag)
	f.LastModified = strPtr(lastMod)
	f.LastFetchedAt = strPtr(lastFetch)
	f.LastError = strPtr(lastErr)
	f.IsPaused = paused != 0
	return f, nil
}
