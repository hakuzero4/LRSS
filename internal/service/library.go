package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"lrss/internal/favicon"
	"lrss/internal/fulltext"
	"lrss/internal/htmltext"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/rss"

	"github.com/microcosm-cc/bluemonday"
)

// Library orchestrates feed fetch, persistence, and article mutations.
type Library struct {
	Feeds    FeedStore
	Articles ArticleStore
	Folders  FolderStore
	RSS      RSSFetcher

	// Fulltext fetches original article pages (surf fingerprint client).
	// Optional; nil uses fulltext.Fetch defaults.
	Fulltext FulltextFetcher

	// Sanitizer for content_html (UGC policy). Lazily created if nil.
	Sanitizer *bluemonday.Policy

	// refreshMu serializes RefreshAll and ImportOPML fetch phase with auto-refresh.
	refreshMu sync.Mutex
}

// NewLibrary constructs a Library with default HTML sanitizer.
func NewLibrary(feeds FeedStore, articles ArticleStore, folders FolderStore, rssClient RSSFetcher) *Library {
	return &Library{
		Feeds:     feeds,
		Articles:  articles,
		Folders:   folders,
		RSS:       rssClient,
		Sanitizer: bluemonday.UGCPolicy(),
	}
}

// NewLibraryFromRepos wires a repo.Repos + rss client.
func NewLibraryFromRepos(r *repo.Repos, rssClient RSSFetcher) *Library {
	return NewLibrary(r.Feeds, r.Articles, r.Folders, rssClient)
}

func (lib *Library) sanitizeHTML(raw string) string {
	if raw == "" {
		return ""
	}
	p := lib.Sanitizer
	if p == nil {
		p = bluemonday.UGCPolicy()
	}
	return p.Sanitize(raw)
}

// ListFolders returns sidebar folders.
func (lib *Library) ListFolders(ctx context.Context) ([]model.Folder, error) {
	return lib.Folders.List(ctx)
}

// ListFeeds returns all feeds with unread counts.
func (lib *Library) ListFeeds(ctx context.Context) ([]model.Feed, error) {
	return lib.Feeds.List(ctx)
}

// AddFeed validates URL, fetches once, inserts feed + articles.
func (lib *Library) AddFeed(ctx context.Context, feedURL string, folderID *string) (model.Feed, error) {
	feedURL = strings.TrimSpace(feedURL)
	if err := validateFeedURL(feedURL); err != nil {
		return model.Feed{}, err
	}

	if folderID != nil && strings.TrimSpace(*folderID) == "" {
		folderID = nil
	}

	// Serialize with refresh/purge-adjacent work so insert+upsert is not interleaved
	// with RefreshAll on the single SQLite connection.
	lib.refreshMu.Lock()
	defer lib.refreshMu.Unlock()

	// Already subscribed: re-sync articles when local store is empty (zombie feed
	// with ETag but 0 items used to stick forever on 304 refreshes).
	if existing, err := lib.Feeds.GetByURL(ctx, feedURL); err == nil {
		if folderID != nil {
			_ = lib.Feeds.SetFolder(ctx, existing.ID, folderID)
		}
		n, cerr := lib.Articles.CountByFeed(ctx, existing.ID)
		if cerr == nil && n == 0 {
			// Drop validators so refresh re-downloads the body.
			existing.ETag = nil
			existing.LastModified = nil
			if _, rerr := lib.refreshOne(ctx, existing); rerr != nil {
				return model.Feed{}, rerr
			}
		}
		out, gerr := lib.Feeds.Get(ctx, existing.ID)
		if gerr != nil {
			return existing, nil
		}
		return out, nil
	} else if err != nil && err != sql.ErrNoRows {
		return model.Feed{}, err
	}

	result, parsed, err := lib.fetchAndMap(ctx, feedURL, rss.FetchOptions{})
	if err != nil {
		return model.Feed{}, fmt.Errorf("fetch feed: %w", err)
	}
	if result.NotModified || parsed == nil {
		return model.Feed{}, fmt.Errorf("unexpected empty parse for new feed")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		title = feedURL
	}
	var siteURL *string
	if s := strings.TrimSpace(parsed.SiteURL); s != "" {
		siteURL = &s
	}
	var etag, lastMod *string
	if result.ETag != "" {
		e := result.ETag
		etag = &e
	}
	if result.LastModified != "" {
		m := result.LastModified
		lastMod = &m
	}

	feed := &model.Feed{
		FolderID:      folderID,
		Title:         title,
		SiteURL:       siteURL,
		FeedURL:       feedURL,
		ETag:          etag,
		LastModified:  lastMod,
		LastFetchedAt: &now,
		IsPaused:      false,
	}
	if err := lib.Feeds.Insert(ctx, feed); err != nil {
		return model.Feed{}, err
	}

	items := mapParsedItems(parsed.Items, lib)
	if _, err := lib.Articles.UpsertFromParsed(ctx, feed.ID, items); err != nil {
		// Leave the feed row (user can refresh) but clear validators so a retry
		// cannot 304-loop with an empty article table.
		_ = lib.Feeds.UpdateAfterFetch(ctx, feed.ID, "", nil, nil, ptrString(err.Error()))
		return model.Feed{}, err
	}

	// Best-effort favicon; never fail AddFeed for icon discovery.
	lib.ensureFavicon(ctx, feed.ID, siteURL, feedURL, nil)

	out, err := lib.Feeds.Get(ctx, feed.ID)
	if err != nil {
		return *feed, nil
	}
	return out, nil
}

func ptrString(s string) *string { return &s }

// ensureFavicon resolves and stores a favicon when missing.
// existing is the current favicon pointer (skip when already set).
func (lib *Library) ensureFavicon(ctx context.Context, feedID string, siteURL *string, feedURL string, existing *string) {
	if existing != nil && strings.TrimSpace(*existing) != "" {
		return
	}
	site := ""
	if siteURL != nil {
		site = *siteURL
	}
	icon := favicon.Resolve(ctx, site, feedURL)
	if icon == "" {
		return
	}
	_ = lib.Feeds.SetFaviconURL(ctx, feedID, icon)
}

// DeleteFeed removes a subscription (and FTS for its articles).
func (lib *Library) DeleteFeed(ctx context.Context, feedID string) error {
	return lib.Feeds.Delete(ctx, feedID)
}

// ClearAllResult summarizes wiping all subscriptions.
type ClearAllResult struct {
	FeedsDeleted   int `json:"feedsDeleted"`
	FoldersDeleted int `json:"foldersDeleted"`
}

// ClearAllSubscriptions removes every feed (and cascaded articles/embeddings/FTS)
// and every folder. Blocks concurrent refresh while running.
func (lib *Library) ClearAllSubscriptions(ctx context.Context) (ClearAllResult, error) {
	lib.refreshMu.Lock()
	defer lib.refreshMu.Unlock()

	feedsN, err := lib.Feeds.DeleteAll(ctx)
	if err != nil {
		return ClearAllResult{}, err
	}
	foldersN, err := lib.Folders.DeleteAll(ctx)
	if err != nil {
		return ClearAllResult{}, err
	}
	return ClearAllResult{
		FeedsDeleted:   feedsN,
		FoldersDeleted: foldersN,
	}, nil
}

// RefreshFeed re-fetches one feed and upserts articles. Returns number of new articles.
// Serialized with RefreshAll / TryRefreshDue via refreshMu so concurrent manual +
// auto refresh cannot interleave upserts on MaxOpenConns=1.
func (lib *Library) RefreshFeed(ctx context.Context, feedID string) (int, error) {
	lib.refreshMu.Lock()
	defer lib.refreshMu.Unlock()

	feed, err := lib.Feeds.Get(ctx, feedID)
	if err != nil {
		return 0, err
	}
	if feed.IsPaused {
		return 0, nil
	}
	return lib.refreshOne(ctx, feed)
}

// RefreshAllResult summarizes a full refresh.
type RefreshAllResult struct {
	FeedsOK       int `json:"feedsOk"`
	FeedsErr      int `json:"feedsErr"`
	ArticlesAdded int `json:"articlesAdded"`
}

// RefreshAll refreshes non-paused feeds sequentially.
// Concurrent callers block on refreshMu so only one full refresh runs at a time.
func (lib *Library) RefreshAll(ctx context.Context) (RefreshAllResult, error) {
	lib.refreshMu.Lock()
	defer lib.refreshMu.Unlock()

	feeds, err := lib.Feeds.ListActive(ctx)
	if err != nil {
		return RefreshAllResult{}, err
	}
	var res RefreshAllResult
	for _, f := range feeds {
		n, err := lib.refreshOne(ctx, f)
		if err != nil {
			res.FeedsErr++
			continue
		}
		res.FeedsOK++
		res.ArticlesAdded += n
	}
	return res, nil
}

// TryRefreshAll is like RefreshAll but returns immediately if a refresh is already running.
// ok is false when the lock could not be acquired.
func (lib *Library) TryRefreshAll(ctx context.Context) (res RefreshAllResult, ok bool, err error) {
	if !lib.refreshMu.TryLock() {
		return RefreshAllResult{}, false, nil
	}
	defer lib.refreshMu.Unlock()

	feeds, err := lib.Feeds.ListActive(ctx)
	if err != nil {
		return RefreshAllResult{}, true, err
	}
	for _, f := range feeds {
		n, rerr := lib.refreshOne(ctx, f)
		if rerr != nil {
			res.FeedsErr++
			continue
		}
		res.FeedsOK++
		res.ArticlesAdded += n
	}
	return res, true, nil
}

// EffectiveRefreshMinutes returns the interval used for auto-refresh for one feed.
// Per-feed value wins when > 0; otherwise defaultMinutes (clamped to [5, 180]).
func EffectiveRefreshMinutes(feed model.Feed, defaultMinutes int) int {
	if feed.RefreshIntervalMinutes > 0 {
		return repo.NormalizeRefreshInterval(feed.RefreshIntervalMinutes)
	}
	if defaultMinutes < 5 {
		return 5
	}
	if defaultMinutes > 180 {
		return 180
	}
	return defaultMinutes
}

// feedRefreshDue reports whether feed should be fetched now for auto-refresh.
func feedRefreshDue(feed model.Feed, defaultMinutes int, now time.Time) bool {
	interval := EffectiveRefreshMinutes(feed, defaultMinutes)
	if feed.LastFetchedAt == nil || strings.TrimSpace(*feed.LastFetchedAt) == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(*feed.LastFetchedAt))
	if err != nil {
		// Tolerate fractional seconds / space separator from older rows.
		t, err = time.Parse(time.RFC3339Nano, strings.TrimSpace(*feed.LastFetchedAt))
		if err != nil {
			return true
		}
	}
	return !now.Before(t.Add(time.Duration(interval) * time.Minute))
}

// TryRefreshDue refreshes only non-paused feeds whose last fetch is older than
// their effective interval. defaultMinutes is the global LibraryConfig interval.
// ok is false when another refresh holds the lock.
func (lib *Library) TryRefreshDue(ctx context.Context, defaultMinutes int) (res RefreshAllResult, ok bool, err error) {
	if !lib.refreshMu.TryLock() {
		return RefreshAllResult{}, false, nil
	}
	defer lib.refreshMu.Unlock()

	feeds, err := lib.Feeds.ListActive(ctx)
	if err != nil {
		return RefreshAllResult{}, true, err
	}
	now := time.Now().UTC()
	for _, f := range feeds {
		if !feedRefreshDue(f, defaultMinutes, now) {
			continue
		}
		n, rerr := lib.refreshOne(ctx, f)
		if rerr != nil {
			res.FeedsErr++
			continue
		}
		res.FeedsOK++
		res.ArticlesAdded += n
	}
	return res, true, nil
}

// RenameFeed sets a custom display title (locked against feed-document overwrites).
func (lib *Library) RenameFeed(ctx context.Context, feedID, title string) error {
	return lib.Feeds.SetTitle(ctx, feedID, title)
}

// SetFeedRefreshInterval sets per-feed auto-refresh minutes (0 = global default).
func (lib *Library) SetFeedRefreshInterval(ctx context.Context, feedID string, minutes int) error {
	return lib.Feeds.SetRefreshInterval(ctx, feedID, minutes)
}

func (lib *Library) refreshOne(ctx context.Context, feed model.Feed) (int, error) {
	opts := rss.FetchOptions{}
	if feed.ETag != nil {
		opts.ETag = *feed.ETag
	}
	if feed.LastModified != nil {
		opts.LastModified = *feed.LastModified
	}

	// Local empty + remote validators: conditional GET would 304 forever and never
	// repopulate articles. Skip If-None-Match / If-Modified-Since in that case.
	localCount, _ := lib.Articles.CountByFeed(ctx, feed.ID)
	if localCount == 0 {
		opts.ETag = ""
		opts.LastModified = ""
	}

	result, parsed, err := lib.fetchAndMap(ctx, feed.FeedURL, opts)
	if err != nil {
		msg := err.Error()
		_ = lib.Feeds.UpdateAfterFetch(ctx, feed.ID, "", feed.ETag, feed.LastModified, &msg)
		return 0, err
	}

	var etag, lastMod *string
	if result.ETag != "" {
		e := result.ETag
		etag = &e
	} else {
		etag = feed.ETag
	}
	if result.LastModified != "" {
		m := result.LastModified
		lastMod = &m
	} else {
		lastMod = feed.LastModified
	}

	if result.NotModified {
		if err := lib.Feeds.UpdateAfterFetch(ctx, feed.ID, "", etag, lastMod, nil); err != nil {
			return 0, err
		}
		lib.ensureFavicon(ctx, feed.ID, feed.SiteURL, feed.FeedURL, feed.FaviconURL)
		return 0, nil
	}

	// Only adopt remote title when the user has not renamed this feed.
	title := ""
	var siteURL *string
	if parsed != nil {
		if !feed.TitleUserSet {
			title = strings.TrimSpace(parsed.Title)
		}
		if s := strings.TrimSpace(parsed.SiteURL); s != "" {
			siteURL = &s
			// Persist site URL when discovered (also helps favicon).
			if feed.SiteURL == nil || strings.TrimSpace(*feed.SiteURL) == "" {
				_ = lib.Feeds.SetSiteURL(ctx, feed.ID, s)
				feed.SiteURL = siteURL
			}
		}
	}
	if siteURL == nil {
		siteURL = feed.SiteURL
	}
	if err := lib.Feeds.UpdateAfterFetch(ctx, feed.ID, title, etag, lastMod, nil); err != nil {
		return 0, err
	}

	if parsed == nil {
		lib.ensureFavicon(ctx, feed.ID, siteURL, feed.FeedURL, feed.FaviconURL)
		return 0, nil
	}
	items := mapParsedItems(parsed.Items, lib)
	up, err := lib.Articles.UpsertFromParsed(ctx, feed.ID, items)
	if err != nil {
		msg := err.Error()
		// Clear validators so the next refresh re-downloads instead of 304-looping.
		_ = lib.Feeds.UpdateAfterFetch(ctx, feed.ID, title, nil, nil, &msg)
		return 0, err
	}
	lib.ensureFavicon(ctx, feed.ID, siteURL, feed.FeedURL, feed.FaviconURL)
	return up.Inserted, nil
}

// ListArticles returns a page of articles for a collection.
// excludeNsfw hides articles from is_nsfw feeds (office mode).
func (lib *Library) ListArticles(ctx context.Context, collection string, limit, offset int, excludeNsfw bool) ([]model.Article, error) {
	list, err := lib.Articles.List(ctx, collection, repo.ListOpts{
		Limit: limit, Offset: offset, ExcludeNsfw: excludeNsfw,
	})
	if err != nil {
		return nil, err
	}
	for i := range list {
		lib.normalizeArticleForUI(&list[i], false)
	}
	return list, nil
}

// SmartCounts returns sidebar smart-list totals (full DB counts, not list page size).
// excludeNsfw omits NSFW-feed articles when true.
func (lib *Library) SmartCounts(ctx context.Context, excludeNsfw bool) (repo.SmartCounts, error) {
	return lib.Articles.CountSmart(ctx, excludeNsfw)
}

// GetArticle returns one article with sanitized ContentHTML for UI.
func (lib *Library) GetArticle(ctx context.Context, articleID string) (model.Article, error) {
	a, err := lib.Articles.Get(ctx, articleID)
	if err != nil {
		return model.Article{}, err
	}
	lib.normalizeArticleForUI(&a, true)
	return a, nil
}

// normalizeArticleForUI strips HTML from summary (legacy rows) and sanitizes body.
// full=true also prepares content for the reader.
func (lib *Library) normalizeArticleForUI(a *model.Article, full bool) {
	if a == nil {
		return
	}
	if a.Summary != nil && *a.Summary != "" {
		plain := htmltext.ToText(*a.Summary)
		// Drop summary when it is only a dump of the full HTML body lead-in.
		if plain == "" {
			a.Summary = nil
		} else {
			a.Summary = &plain
		}
	}
	if full && a.ContentHTML != nil && *a.ContentHTML != "" {
		san := lib.sanitizeHTML(*a.ContentHTML)
		a.ContentHTML = &san
	}
	// Hide redundant summary when body text starts with the same plain text.
	if a.Summary != nil && a.ContentText != nil {
		s, c := *a.Summary, *a.ContentText
		if s != "" && c != "" && (s == c || strings.HasPrefix(c, s)) {
			a.Summary = nil
		}
	}
}

// FetchFullContent downloads the article's original URL (via fingerprint HTTP),
// extracts readable HTML, persists it, and returns the updated article for UI.
func (lib *Library) FetchFullContent(ctx context.Context, articleID string) (model.Article, error) {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return model.Article{}, fmt.Errorf("article id is required")
	}
	a, err := lib.Articles.Get(ctx, articleID)
	if err != nil {
		return model.Article{}, err
	}
	pageURL := strings.TrimSpace(a.URL)
	if pageURL == "" {
		return model.Article{}, fmt.Errorf("article has no url")
	}
	if err := validateArticleURL(pageURL); err != nil {
		return model.Article{}, err
	}

	var html, text string
	if lib.Fulltext != nil {
		html, text, err = lib.Fulltext.Fetch(ctx, pageURL)
	} else {
		res, ferr := fulltext.Fetch(ctx, pageURL, fulltext.Options{})
		html, text, err = res.HTML, res.Text, ferr
	}
	if err != nil {
		return model.Article{}, err
	}
	html = lib.sanitizeHTML(html)
	if text == "" {
		text = htmltext.ToText(html)
	}
	if strings.TrimSpace(html) == "" {
		return model.Article{}, fmt.Errorf("no readable content")
	}

	if err := lib.Articles.UpdateContent(ctx, articleID, html, text); err != nil {
		return model.Article{}, err
	}
	return lib.GetArticle(ctx, articleID)
}

func validateArticleURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return fmt.Errorf("invalid article url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("article url must be http(s)")
	}
	if u.Host == "" {
		return fmt.Errorf("article url host is required")
	}
	return nil
}

// UpdateArticleSummary persists a new summary (e.g. AI-generated deck) and refreshes FTS.
func (lib *Library) UpdateArticleSummary(ctx context.Context, articleID, summary string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("article id is required")
	}
	return lib.Articles.UpdateSummary(ctx, articleID, summary)
}

// UpdateArticleContent replaces body HTML/text (e.g. after full-text fetch) and refreshes FTS.
// Not used for AI translate — translate keeps the original and stores bilingual text separately.
func (lib *Library) UpdateArticleContent(ctx context.Context, articleID, contentHTML, contentText string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("article id is required")
	}
	// Sanitize HTML for UI storage.
	if contentHTML != "" {
		contentHTML = lib.sanitizeHTML(contentHTML)
	}
	if contentText == "" && contentHTML != "" {
		contentText = htmltext.ToText(contentHTML)
	}
	return lib.Articles.UpdateContent(ctx, articleID, contentHTML, contentText)
}

// SaveArticleTranslation stores bilingual <<o>>/<<t>> text next to the original body.
func (lib *Library) SaveArticleTranslation(ctx context.Context, articleID, raw, lang string) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("article id is required")
	}
	return lib.Articles.UpdateTranslation(ctx, articleID, raw, lang)
}

// ClearArticleTranslation drops stored bilingual text; original body is unchanged.
func (lib *Library) ClearArticleTranslation(ctx context.Context, articleID string) error {
	return lib.Articles.ClearTranslation(ctx, articleID)
}

// SetRead marks an article read/unread.
func (lib *Library) SetRead(ctx context.Context, articleID string, read bool) error {
	return lib.Articles.SetRead(ctx, articleID, read)
}

// SetStarred marks an article starred/unstarred.
func (lib *Library) SetStarred(ctx context.Context, articleID string, starred bool) error {
	return lib.Articles.SetStarred(ctx, articleID, starred)
}

// MarkAllRead marks all articles in a collection as read.
// excludeNsfw skips NSFW-feed articles (office mode).
func (lib *Library) MarkAllRead(ctx context.Context, collection string, excludeNsfw bool) error {
	return lib.Articles.MarkAllRead(ctx, collection, excludeNsfw)
}

// SetFeedNsfw marks or unmarks a feed as sensitive.
func (lib *Library) SetFeedNsfw(ctx context.Context, feedID string, nsfw bool) error {
	if strings.TrimSpace(feedID) == "" {
		return fmt.Errorf("feed id is required")
	}
	return lib.Feeds.SetNsfw(ctx, feedID, nsfw)
}

// PurgeOldArticles deletes non-starred articles older than days days.
// days is clamped by the repo layer to [7, 365]. Returns deleted count.
// Holds refreshMu so purge cannot race with AddFeed / RefreshFeed upserts.
func (lib *Library) PurgeOldArticles(ctx context.Context, days int) (int, error) {
	lib.refreshMu.Lock()
	defer lib.refreshMu.Unlock()
	return lib.Articles.PurgeOlderThan(ctx, days)
}

func (lib *Library) fetchAndMap(ctx context.Context, feedURL string, opts rss.FetchOptions) (*rss.FetchResult, *rss.ParsedFeed, error) {
	// Prefer FetchAndMap when client supports it.
	if c, ok := lib.RSS.(*rss.Client); ok {
		return c.FetchAndMap(ctx, feedURL, opts)
	}
	res, err := lib.RSS.Fetch(ctx, feedURL, opts)
	if err != nil {
		return res, nil, err
	}
	if res.NotModified || res.Parsed == nil {
		return res, nil, nil
	}
	return res, rss.ToParsedFeed(res.Parsed, feedURL), nil
}

func mapParsedItems(items []rss.ParsedItem, lib *Library) []repo.ParsedItem {
	out := make([]repo.ParsedItem, 0, len(items))
	for _, it := range items {
		html := it.ContentHTML
		if html != "" {
			html = lib.sanitizeHTML(html)
		}
		summary := it.Summary
		if summary != "" && strings.Contains(summary, "<") {
			summary = lib.sanitizeHTML(summary)
		}
		item := repo.ParsedItem{
			GUID:  it.GUID,
			URL:   it.URL,
			Title: it.Title,
		}
		if it.Author != "" {
			a := it.Author
			item.Author = &a
		}
		if summary != "" {
			item.Summary = &summary
		}
		if html != "" {
			item.ContentHTML = &html
		}
		if it.ContentText != "" {
			t := it.ContentText
			item.ContentText = &t
		}
		if it.ImageURL != "" {
			img := it.ImageURL
			item.ImageURL = &img
		}
		if it.PublishedAt != nil {
			s := it.PublishedAt.UTC().Format(time.RFC3339)
			item.PublishedAt = &s
		}
		out = append(out, item)
	}
	return out
}

func validateFeedURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("feed url is required")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid feed url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("feed url must be http(s)")
	}
	if u.Host == "" {
		return fmt.Errorf("feed url host is required")
	}
	return nil
}
