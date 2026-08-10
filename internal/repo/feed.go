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
	       f.refresh_interval_minutes, f.title_user_set,
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
	if f.RefreshIntervalMinutes < 0 {
		f.RefreshIntervalMinutes = 0
	}
	_, err := r.DB.ExecContext(ctx, `
		INSERT INTO feeds (
			id, folder_id, title, site_url, feed_url, favicon_url,
			etag, last_modified, last_fetched_at, last_error, is_paused,
			refresh_interval_minutes, title_user_set,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID, nullStr(f.FolderID), f.Title, nullStr(f.SiteURL), f.FeedURL, nullStr(f.FaviconURL),
		nullStr(f.ETag), nullStr(f.LastModified), nullStr(f.LastFetchedAt), nullStr(f.LastError),
		boolToInt(f.IsPaused), f.RefreshIntervalMinutes, boolToInt(f.TitleUserSet),
		f.CreatedAt, f.UpdatedAt,
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

// SetFolder assigns a feed to a folder. nil or empty folderID means unfiled.
// Non-empty folder IDs are verified to exist before update.
func (r *FeedRepo) SetFolder(ctx context.Context, feedID string, folderID *string) error {
	if folderID != nil {
		id := strings.TrimSpace(*folderID)
		if id == "" {
			folderID = nil
		} else {
			folderID = &id
			var n int
			err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM folders WHERE id = ?`, id).Scan(&n)
			if err != nil {
				return fmt.Errorf("set folder: %w", err)
			}
			if n == 0 {
				return fmt.Errorf("folder not found: %s", id)
			}
		}
	}
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE feeds SET folder_id = ?, updated_at = ? WHERE id = ?`,
		nullStr(folderID), now, feedID)
	if err != nil {
		return fmt.Errorf("set folder: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feed not found: %s", feedID)
	}
	return nil
}

// SetSiteURL updates site_url when discovered from the feed document.
func (r *FeedRepo) SetSiteURL(ctx context.Context, feedID, siteURL string) error {
	siteURL = strings.TrimSpace(siteURL)
	var site any
	if siteURL != "" {
		site = siteURL
	}
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE feeds SET site_url = ?, updated_at = ? WHERE id = ?`,
		site, now, feedID)
	if err != nil {
		return fmt.Errorf("set site_url: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feed not found: %s", feedID)
	}
	return nil
}

// SetFaviconURL stores the feed's favicon absolute URL (empty clears).
func (r *FeedRepo) SetFaviconURL(ctx context.Context, feedID, faviconURL string) error {
	faviconURL = strings.TrimSpace(faviconURL)
	var fav any
	if faviconURL != "" {
		fav = faviconURL
	}
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE feeds SET favicon_url = ?, updated_at = ? WHERE id = ?`,
		fav, now, feedID)
	if err != nil {
		return fmt.Errorf("set favicon: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feed not found: %s", feedID)
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

// SetTitle renames a feed and locks the title so refresh will not overwrite it.
func (r *FeedRepo) SetTitle(ctx context.Context, feedID, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("title required")
	}
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE feeds SET title = ?, title_user_set = 1, updated_at = ? WHERE id = ?`,
		title, now, feedID)
	if err != nil {
		return fmt.Errorf("set title: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feed not found: %s", feedID)
	}
	return nil
}

// NormalizeRefreshInterval clamps minutes: 0 keeps global default; else [5, 180].
func NormalizeRefreshInterval(minutes int) int {
	if minutes <= 0 {
		return 0
	}
	if minutes < 5 {
		return 5
	}
	if minutes > 180 {
		return 180
	}
	return minutes
}

// SetRefreshInterval sets per-feed auto-refresh minutes (0 = use global default).
func (r *FeedRepo) SetRefreshInterval(ctx context.Context, feedID string, minutes int) error {
	minutes = NormalizeRefreshInterval(minutes)
	now := nowUTC()
	res, err := r.DB.ExecContext(ctx, `
		UPDATE feeds SET refresh_interval_minutes = ?, updated_at = ? WHERE id = ?`,
		minutes, now, feedID)
	if err != nil {
		return fmt.Errorf("set refresh interval: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("feed not found: %s", feedID)
	}
	return nil
}

// DeleteAll removes every feed (articles cascade via FK). Clears articles_fts first
// because FTS is application-synced, not FK-linked. Returns the number of feeds removed.
func (r *FeedRepo) DeleteAll(ctx context.Context) (int, error) {
	var n int
	if err := r.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM feeds`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count feeds: %w", err)
	}
	if _, err := r.DB.ExecContext(ctx, `DELETE FROM articles_fts`); err != nil {
		return 0, fmt.Errorf("clear articles_fts: %w", err)
	}
	if _, err := r.DB.ExecContext(ctx, `DELETE FROM feeds`); err != nil {
		return 0, fmt.Errorf("delete all feeds: %w", err)
	}
	return n, nil
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
	var paused, titleUserSet int
	if err := row.Scan(
		&f.ID, &folder, &f.Title, &site, &f.FeedURL, &fav,
		&etag, &lastMod, &lastFetch, &lastErr, &paused,
		&f.RefreshIntervalMinutes, &titleUserSet,
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
	f.TitleUserSet = titleUserSet != 0
	return f, nil
}
