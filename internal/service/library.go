package service

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"lrss/internal/favicon"
	"lrss/internal/fulltext"
	"lrss/internal/htmltext"
	"lrss/internal/llm"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/ytcaptions"

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

	// refreshMu serializes a single refreshOne / AddFeed / purge unit so upserts
	// do not interleave on MaxOpenConns=1.
	refreshMu sync.Mutex
	// refreshBatchMu covers multi-feed passes (RefreshAll / TryRefreshDue).
	// Held for the whole pass, but refreshMu is released between feeds so a
	// manual RefreshFeed is not stuck behind hundreds of HTTP round-trips.
	refreshBatchMu sync.Mutex

	// forceMu guards the manual "refresh all" queue (same pacing as auto-refresh).
	forceMu  sync.Mutex
	forceIDs []string
	forceSet map[string]struct{}

	// fulltextMu guards the auto full-content queue (separate from feed refresh).
	fulltextMu  sync.Mutex
	fulltextIDs []string
	fulltextSet map[string]struct{}

	// FullContentEnabled reports Settings → Feeds "fetch full content".
	// Nil means never auto-queue full-content work.
	FullContentEnabled func(ctx context.Context) bool

	// OnArticlesInserted is called with newly inserted article IDs after a
	// successful upsert (refresh / add feed). Optional. Must not call LLM
	// while holding refresh locks — enqueue only.
	OnArticlesInserted func(ctx context.Context, ids []string)

	actMu          sync.Mutex
	actFeedID      string
	actFeedTitle   string
	actInserted    int
}

// AutoFulltextMaxPerTick caps original-page fetches per background drain.
// Independent of AutoRefreshMaxFeedsPerTick so feed refresh is not slowed.
const AutoFulltextMaxPerTick = 8

// fulltextQueueCap bounds memory if many partials arrive while the drain is slow.
const fulltextQueueCap = 2000

// youtubeEmbedSrcRe allows only YouTube / youtube-nocookie embed iframe srcs.
var youtubeEmbedSrcRe = regexp.MustCompile(`(?i)^https://(www\.)?(youtube\.com|youtube-nocookie\.com)/embed/[A-Za-z0-9_-]{6,20}([?#].*)?$`)

// newArticleSanitizer is UGC plus privacy-friendly YouTube embeds + caption blocks.
func newArticleSanitizer() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.AllowElements("iframe", "section")
	p.AllowAttrs("width", "height", "allow", "allowfullscreen", "frameborder",
		"loading", "title", "class", "referrerpolicy", "sandbox").OnElements("iframe")
	p.AllowAttrs("src").Matching(youtubeEmbedSrcRe).OnElements("iframe")
	// yt-embed / yt-desc / yt-captions wrappers, titles, and unavailable notice
	p.AllowAttrs("class").OnElements("div", "section", "h2", "h3", "p", "span")
	p.AllowAttrs("id").Matching(regexp.MustCompile(`^lrss-yt-captions$`)).OnElements("section", "div")
	p.AllowAttrs("data-yt-captions", "data-yt-captions-miss", "data-yt-captions-timed", "hidden").OnElements("section", "div")
	return p
}

// NewLibrary constructs a Library with default HTML sanitizer.
func NewLibrary(feeds FeedStore, articles ArticleStore, folders FolderStore, rssClient RSSFetcher) *Library {
	return &Library{
		Feeds:     feeds,
		Articles:  articles,
		Folders:   folders,
		RSS:       rssClient,
		Sanitizer: newArticleSanitizer(),
	}
}

// NewLibraryFromRepos wires a repo.Repos + rss client.
func NewLibraryFromRepos(r *repo.Repos, rssClient RSSFetcher) *Library {
	return NewLibrary(r.Feeds, r.Articles, r.Folders, rssClient)
}

func (lib *Library) emitInserted(ctx context.Context, ids []string) {
	if lib == nil || len(ids) == 0 {
		return
	}
	lib.actMu.Lock()
	lib.actInserted += len(ids)
	lib.actMu.Unlock()
	if lib.OnArticlesInserted != nil {
		lib.OnArticlesInserted(ctx, ids)
	}
}

// InsertedTotal is the process-lifetime count of newly stored articles.
// The UI polls this via JobActivity to refresh unread badges after background ticks.
func (lib *Library) InsertedTotal() int {
	if lib == nil {
		return 0
	}
	lib.actMu.Lock()
	n := lib.actInserted
	lib.actMu.Unlock()
	return n
}

func (lib *Library) sanitizeHTML(raw string) string {
	if raw == "" {
		return ""
	}
	p := lib.Sanitizer
	if p == nil {
		p = newArticleSanitizer()
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
	up, err := lib.Articles.UpsertFromParsed(ctx, feed.ID, items)
	if err != nil {
		// Leave the feed row (user can refresh) but clear validators so a retry
		// cannot 304-loop with an empty article table.
		_ = lib.Feeds.UpdateAfterFetch(ctx, feed.ID, "", nil, nil, ptrString(err.Error()))
		return model.Feed{}, err
	}
	lib.maybeQueueFullContent(ctx, up.InsertedIDs)
	lib.emitInserted(ctx, up.InsertedIDs)

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
	lib.refreshBatchMu.Lock()
	defer lib.refreshBatchMu.Unlock()
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
// Only holds the per-feed lock for this one source so a long RefreshAll / auto-refresh
// pass does not leave the UI spinner waiting behind every other feed.
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
	lib.beginRefreshFeed(feed.ID, feed.Title)
	defer lib.endRefreshFeed()
	return lib.refreshOne(ctx, feed)
}

// RefreshAllResult summarizes a refresh batch (auto-due and/or manual force queue).
type RefreshAllResult struct {
	FeedsOK       int `json:"feedsOk"`
	FeedsErr      int `json:"feedsErr"`
	ArticlesAdded int `json:"articlesAdded"`
	// FeedsPending is how many force-queued feeds remain after this batch
	// (manual Refresh All is paced like auto-refresh).
	FeedsPending int `json:"feedsPending"`
}

// AutoRefreshMaxFeedsPerTick is the max number of feeds one background tick
// (or one RefreshAll API call) may fetch. Large libraries are drained across
// minutes instead of one multi-minute spike.
const AutoRefreshMaxFeedsPerTick = 20

// ForceQueueLen returns how many feeds are waiting from manual Refresh All.
func (lib *Library) ForceQueueLen() int {
	lib.forceMu.Lock()
	defer lib.forceMu.Unlock()
	return len(lib.forceIDs)
}

func (lib *Library) enqueueForceIDs(ids []string) {
	if len(ids) == 0 {
		return
	}
	lib.forceMu.Lock()
	defer lib.forceMu.Unlock()
	if lib.forceSet == nil {
		lib.forceSet = make(map[string]struct{}, len(ids))
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := lib.forceSet[id]; ok {
			continue
		}
		lib.forceSet[id] = struct{}{}
		lib.forceIDs = append(lib.forceIDs, id)
	}
}

func (lib *Library) popForceIDs(n int) []string {
	lib.forceMu.Lock()
	defer lib.forceMu.Unlock()
	if n <= 0 || len(lib.forceIDs) == 0 {
		return nil
	}
	if n > len(lib.forceIDs) {
		n = len(lib.forceIDs)
	}
	out := append([]string(nil), lib.forceIDs[:n]...)
	lib.forceIDs = lib.forceIDs[n:]
	for _, id := range out {
		delete(lib.forceSet, id)
	}
	if len(lib.forceIDs) == 0 {
		lib.forceSet = nil
	}
	return out
}

// FulltextQueueLen returns pending auto full-content article ids.
func (lib *Library) FulltextQueueLen() int {
	lib.fulltextMu.Lock()
	defer lib.fulltextMu.Unlock()
	return len(lib.fulltextIDs)
}

func (lib *Library) enqueueFulltextIDs(ids []string) {
	if len(ids) == 0 {
		return
	}
	lib.fulltextMu.Lock()
	defer lib.fulltextMu.Unlock()
	if lib.fulltextSet == nil {
		lib.fulltextSet = make(map[string]struct{}, len(ids))
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := lib.fulltextSet[id]; ok {
			continue
		}
		if len(lib.fulltextIDs) >= fulltextQueueCap {
			break
		}
		lib.fulltextSet[id] = struct{}{}
		lib.fulltextIDs = append(lib.fulltextIDs, id)
	}
}

func (lib *Library) popFulltextIDs(n int) []string {
	lib.fulltextMu.Lock()
	defer lib.fulltextMu.Unlock()
	if n <= 0 || len(lib.fulltextIDs) == 0 {
		return nil
	}
	if n > len(lib.fulltextIDs) {
		n = len(lib.fulltextIDs)
	}
	out := append([]string(nil), lib.fulltextIDs[:n]...)
	lib.fulltextIDs = lib.fulltextIDs[n:]
	for _, id := range out {
		delete(lib.fulltextSet, id)
	}
	if len(lib.fulltextIDs) == 0 {
		lib.fulltextSet = nil
	}
	return out
}

// QueueNewArticlesForFullContent enqueues newly inserted articles that look
// partial when Settings → Feeds "fetch full content" is on. Cheap local
// heuristic only — does not fetch pages (separate paced drain).
func (lib *Library) QueueNewArticlesForFullContent(ctx context.Context, insertedIDs []string) {
	lib.maybeQueueFullContent(ctx, insertedIDs)
}

// maybeQueueFullContent enqueues newly inserted articles that look partial
// when Settings → Feeds "fetch full content" is on. Cheap local heuristic only.
// Does not fetch pages here (separate paced drain).
func (lib *Library) maybeQueueFullContent(ctx context.Context, insertedIDs []string) {
	if len(insertedIDs) == 0 {
		return
	}
	if lib.FullContentEnabled == nil || !lib.FullContentEnabled(ctx) {
		return
	}
	var toQueue []string
	for _, id := range insertedIDs {
		if ctx.Err() != nil {
			break
		}
		a, err := lib.Articles.Get(ctx, id)
		if err != nil {
			continue
		}
		if a.FullContentFetched {
			continue
		}
		pageURL := strings.TrimSpace(a.URL)
		if pageURL == "" {
			continue
		}
		// YouTube: refresh path already attaches captions; page readability is useless.
		if rss.YouTubeVideoID(pageURL) != "" {
			continue
		}
		body := ""
		if a.ContentText != nil {
			body = *a.ContentText
		}
		if body == "" && a.ContentHTML != nil {
			body = htmltext.ToText(*a.ContentHTML)
		}
		summary := ""
		if a.Summary != nil {
			summary = *a.Summary
		}
		if !llm.NeedsFullContentFetch(a.Title, summary, body, pageURL) {
			continue
		}
		toQueue = append(toQueue, id)
	}
	lib.enqueueFulltextIDs(toQueue)
}

// TryDrainFulltext fetches original pages for up to maxN queued partial articles.
// Uses refreshMu per article only (never refreshBatchMu) so feed refresh queues
// are not blocked. maxN <= 0 uses AutoFulltextMaxPerTick.
func (lib *Library) TryDrainFulltext(ctx context.Context, maxN int) (ok, failed, pending int) {
	if maxN <= 0 {
		maxN = AutoFulltextMaxPerTick
	}
	for i := 0; i < maxN; i++ {
		if ctx.Err() != nil {
			break
		}
		ids := lib.popFulltextIDs(1)
		if len(ids) == 0 {
			break
		}
		id := ids[0]
		lib.refreshMu.Lock()
		_, err := lib.FetchFullContent(ctx, id)
		lib.refreshMu.Unlock()
		if err != nil {
			failed++
			continue
		}
		ok++
	}
	pending = lib.FulltextQueueLen()
	return ok, failed, pending
}

// EnqueueRefreshAll queues every active feed for a paced force-refresh
// (oldest last_fetched first). Dedupes ids already in the queue.
func (lib *Library) EnqueueRefreshAll(ctx context.Context) (int, error) {
	feeds, err := lib.Feeds.ListActive(ctx)
	if err != nil {
		return 0, err
	}
	sort.SliceStable(feeds, func(i, j int) bool {
		return lastFetchedLess(feeds[i], feeds[j])
	})
	ids := make([]string, 0, len(feeds))
	for _, f := range feeds {
		ids = append(ids, f.ID)
	}
	before := lib.ForceQueueLen()
	lib.enqueueForceIDs(ids)
	return lib.ForceQueueLen() - before, nil
}

// RefreshAll enqueues all active feeds and runs one paced batch (same cap as
// auto-refresh). Remaining work is drained by the background refresh loop so
// hundreds of OPML imports are not fetched in a single blocking spike.
func (lib *Library) RefreshAll(ctx context.Context) (RefreshAllResult, error) {
	if _, err := lib.EnqueueRefreshAll(ctx); err != nil {
		return RefreshAllResult{}, err
	}
	res, ok, err := lib.TryRefreshWork(ctx, 30, false)
	if err != nil {
		return res, err
	}
	if !ok {
		// Another multi-feed pass is running; queue is still populated.
		res.FeedsPending = lib.ForceQueueLen()
		return res, nil
	}
	return res, nil
}

// TryRefreshAll enqueues a full refresh and drains one batch if the batch lock
// is free. ok is false when the batch lock could not be acquired (queue still set).
func (lib *Library) TryRefreshAll(ctx context.Context) (res RefreshAllResult, ok bool, err error) {
	if _, err := lib.EnqueueRefreshAll(ctx); err != nil {
		return RefreshAllResult{}, true, err
	}
	return lib.TryRefreshWork(ctx, 30, false)
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

// refreshPhaseMinutes is a stable offset in [0, interval) from the feed id.
// Spreads bulk-due feeds (same last_fetched_at after OPML import) across the
// interval window using wall-clock minutes.
func refreshPhaseMinutes(feedID string, interval int) int {
	if interval <= 1 {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(feedID))
	return int(h.Sum32() % uint32(interval))
}

func parseFeedLastFetched(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			return time.Time{}, false
		}
	}
	return t, true
}

// FeedRefreshDue reports whether a feed is eligible for background auto-refresh
// at now. Age must meet the effective interval; then a stable id-based phase
// staggers sources that became due together. Feeds more than 2× interval overdue
// skip the phase gate so a long offline period can catch up (still capped per tick).
func FeedRefreshDue(feed model.Feed, defaultMinutes int, now time.Time) bool {
	interval := EffectiveRefreshMinutes(feed, defaultMinutes)
	now = now.UTC()

	var age time.Duration
	if feed.LastFetchedAt == nil {
		age = time.Duration(interval*3) * time.Minute // treat as very overdue
	} else if t, ok := parseFeedLastFetched(*feed.LastFetchedAt); !ok {
		age = time.Duration(interval*3) * time.Minute
	} else {
		age = now.Sub(t)
		if age < time.Duration(interval)*time.Minute {
			return false
		}
	}

	phase := refreshPhaseMinutes(feed.ID, interval)
	slot := int(now.Unix()/60) % interval
	if slot == phase {
		return true
	}
	// Catch-up after long downtime: allow without waiting for phase match.
	// max-per-tick still spreads the work across minutes.
	if age >= time.Duration(interval*2)*time.Minute {
		return true
	}
	return false
}

func lastFetchedLess(a, b model.Feed) bool {
	// Never-fetched first, then oldest last_fetched_at, then id for stability.
	at, aOK := time.Time{}, false
	bt, bOK := time.Time{}, false
	if a.LastFetchedAt != nil {
		at, aOK = parseFeedLastFetched(*a.LastFetchedAt)
	}
	if b.LastFetchedAt != nil {
		bt, bOK = parseFeedLastFetched(*b.LastFetchedAt)
	}
	if aOK != bOK {
		return !aOK // a never-fetched → true
	}
	if aOK && bOK && !at.Equal(bt) {
		return at.Before(bt)
	}
	return a.ID < b.ID
}

// SelectFeedsDueForRefresh returns up to maxN feeds that are due, oldest first.
// maxN <= 0 means no cap (used only in tests).
func SelectFeedsDueForRefresh(feeds []model.Feed, defaultMinutes int, now time.Time, maxN int) []model.Feed {
	var due []model.Feed
	for _, f := range feeds {
		if f.IsPaused {
			continue
		}
		if FeedRefreshDue(f, defaultMinutes, now) {
			due = append(due, f)
		}
	}
	sort.SliceStable(due, func(i, j int) bool {
		return lastFetchedLess(due[i], due[j])
	})
	if maxN > 0 && len(due) > maxN {
		due = due[:maxN]
	}
	return due
}

// TryRefreshDue drains the manual force queue first, then interval-due feeds,
// up to AutoRefreshMaxFeedsPerTick. defaultMinutes is the global LibraryConfig
// interval. ok is false when another multi-feed pass holds refreshBatchMu.
func (lib *Library) TryRefreshDue(ctx context.Context, defaultMinutes int) (res RefreshAllResult, ok bool, err error) {
	return lib.TryRefreshWork(ctx, defaultMinutes, true)
}

// TryRefreshWork runs one paced refresh batch.
// includeDue: also refresh interval-due feeds after the force queue.
// When includeDue is false, only the manual force queue is drained (Refresh All).
func (lib *Library) TryRefreshWork(ctx context.Context, defaultMinutes int, includeDue bool) (res RefreshAllResult, ok bool, err error) {
	if !lib.refreshBatchMu.TryLock() {
		return RefreshAllResult{}, false, nil
	}
	defer lib.refreshBatchMu.Unlock()

	res, err = lib.drainRefreshBatchLocked(ctx, defaultMinutes, AutoRefreshMaxFeedsPerTick, includeDue)
	res.FeedsPending = lib.ForceQueueLen()
	return res, true, err
}

// drainRefreshBatchLocked requires refreshBatchMu. Force-queue first, then due.
func (lib *Library) drainRefreshBatchLocked(ctx context.Context, defaultMinutes, maxN int, includeDue bool) (RefreshAllResult, error) {
	var res RefreshAllResult
	if maxN <= 0 {
		maxN = AutoRefreshMaxFeedsPerTick
	}
	budget := maxN
	refreshed := make(map[string]struct{}, maxN)

	for budget > 0 {
		if ctx.Err() != nil {
			break
		}
		ids := lib.popForceIDs(1)
		if len(ids) == 0 {
			break
		}
		id := ids[0]
		feed, gerr := lib.Feeds.Get(ctx, id)
		if gerr != nil || feed.IsPaused {
			// Drop missing/paused from the queue; do not burn the whole budget on junk.
			continue
		}
		lib.beginRefreshFeed(feed.ID, feed.Title)
		lib.refreshMu.Lock()
		n, rerr := lib.refreshOne(ctx, feed)
		lib.refreshMu.Unlock()
		lib.endRefreshFeed()
		refreshed[id] = struct{}{}
		budget--
		if rerr != nil {
			res.FeedsErr++
			continue
		}
		res.FeedsOK++
		res.ArticlesAdded += n
	}

	if !includeDue || budget <= 0 || ctx.Err() != nil {
		return res, nil
	}

	feeds, err := lib.Feeds.ListActive(ctx)
	if err != nil {
		return res, err
	}
	now := time.Now().UTC()
	due := SelectFeedsDueForRefresh(feeds, defaultMinutes, now, budget)
	for _, f := range due {
		if ctx.Err() != nil {
			break
		}
		if _, already := refreshed[f.ID]; already {
			continue
		}
		lib.beginRefreshFeed(f.ID, f.Title)
		lib.refreshMu.Lock()
		n, rerr := lib.refreshOne(ctx, f)
		lib.refreshMu.Unlock()
		lib.endRefreshFeed()
		if rerr != nil {
			res.FeedsErr++
			continue
		}
		res.FeedsOK++
		res.ArticlesAdded += n
	}
	return res, nil
}

// RenameFeed sets a custom display title (locked against feed-document overwrites).
func (lib *Library) RenameFeed(ctx context.Context, feedID, title string) error {
	return lib.Feeds.SetTitle(ctx, feedID, title)
}

// SetFeedURL changes a subscription's feed URL. Rejects invalid URLs and
// conflicts with another feed. Clears ETag / Last-Modified / last_error so the
// next refresh revalidates against the new endpoint. Article history is kept.
func (lib *Library) SetFeedURL(ctx context.Context, feedID, feedURL string) error {
	feedURL = strings.TrimSpace(feedURL)
	if err := validateFeedURL(feedURL); err != nil {
		return err
	}
	cur, err := lib.Feeds.Get(ctx, feedID)
	if err != nil {
		return err
	}
	if cur.FeedURL == feedURL {
		return nil
	}
	other, err := lib.Feeds.GetByURL(ctx, feedURL)
	if err == nil && other.ID != feedID {
		return fmt.Errorf("feed url already subscribed")
	}
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return lib.Feeds.SetFeedURL(ctx, feedID, feedURL)
}

// SetFeedRefreshInterval sets per-feed auto-refresh minutes (0 = global default).
func (lib *Library) SetFeedRefreshInterval(ctx context.Context, feedID string, minutes int) error {
	return lib.Feeds.SetRefreshInterval(ctx, feedID, minutes)
}

// SetFeedKeepArticlesDays sets per-feed retention days (0 = global UIPrefs default).
func (lib *Library) SetFeedKeepArticlesDays(ctx context.Context, feedID string, days int) error {
	return lib.Feeds.SetKeepArticlesDays(ctx, feedID, days)
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
	// Best-effort YouTube transcripts under the embed (does not fail the refresh).
	lib.attachYouTubeCaptions(ctx, items)
	up, err := lib.Articles.UpsertFromParsed(ctx, feed.ID, items)
	if err != nil {
		msg := err.Error()
		// Clear validators so the next refresh re-downloads instead of 304-looping.
		_ = lib.Feeds.UpdateAfterFetch(ctx, feed.ID, title, nil, nil, &msg)
		return 0, err
	}
	// Queue partial new articles for paced full-content fetch (does not block this feed).
	lib.maybeQueueFullContent(ctx, up.InsertedIDs)
	lib.emitInserted(ctx, up.InsertedIDs)
	// Existing YouTube rows use ON CONFLICT DO NOTHING — fill captions for those
	// that still lack a captions section (one pass after upsert).
	lib.backfillYouTubeCaptionsForFeed(ctx, feed.ID, items)
	lib.ensureFavicon(ctx, feed.ID, siteURL, feed.FeedURL, feed.FaviconURL)
	return up.Inserted, nil
}

// attachYouTubeCaptions fetches transcripts for YouTube items and appends HTML.
func (lib *Library) attachYouTubeCaptions(ctx context.Context, items []repo.ParsedItem) {
	for i := range items {
		lib.enrichParsedYouTubeItem(ctx, &items[i])
	}
}

func (lib *Library) enrichParsedYouTubeItem(ctx context.Context, item *repo.ParsedItem) {
	if item == nil {
		return
	}
	vid := rss.YouTubeVideoID(item.URL)
	if vid == "" {
		return
	}
	htmlBody := ""
	if item.ContentHTML != nil {
		htmlBody = *item.ContentHTML
	}
	if !needsYouTubeCaptionFetch(htmlBody) {
		return
	}
	hadPlain := ytcaptions.HasCaptionsSection(htmlBody) && !ytcaptions.HasTimedCaptions(htmlBody)
	base := htmlBody
	if hadPlain {
		// Upgrade path: strip untimed block so we can rewrite with cues.
		base = stripYouTubeCaptionsSection(htmlBody)
	}
	if strings.TrimSpace(base) == "" {
		base = rss.YouTubeEmbedHTML(vid, "")
	}
	res, err := ytcaptions.Fetch(ctx, vid, ytcaptions.Options{Timeout: 12 * time.Second})
	if err != nil {
		// Never destroy readable plain captions when re-fetch fails.
		if hadPlain {
			return
		}
		// No prior captions: mark miss so GetArticle won't hammer YouTube.
		san := lib.sanitizeHTML(markYouTubeCaptionsMiss(base))
		item.ContentHTML = &san
		return
	}
	merged := lib.sanitizeHTML(ytcaptions.AppendHTML(base, res))
	item.ContentHTML = &merged
	if res.Text != "" {
		ct := res.Text
		if item.ContentText != nil && strings.TrimSpace(*item.ContentText) != "" {
			ct = strings.TrimSpace(*item.ContentText) + "\n\n" + res.Text
		}
		item.ContentText = &ct
	}
}

func needsYouTubeCaptionFetch(contentHTML string) bool {
	if strings.Contains(contentHTML, `data-yt-captions-miss="1"`) {
		return false
	}
	// Already have timeline captions — skip.
	if ytcaptions.HasTimedCaptions(contentHTML) {
		return false
	}
	// Plain (untimed) block or no captions — fetch/upgrade.
	return true
}

// stripYouTubeCaptionsSection removes a previous captions <section> so it can be re-appended.
func stripYouTubeCaptionsSection(contentHTML string) string {
	if contentHTML == "" {
		return contentHTML
	}
	// Prefer marker-based cut (same as FetchFullContent path).
	if i := strings.Index(contentHTML, `data-yt-captions="1"`); i > 0 {
		cut := strings.LastIndex(contentHTML[:i], "<section")
		if cut >= 0 {
			return strings.TrimSpace(contentHTML[:cut])
		}
	}
	if i := strings.Index(contentHTML, `id="lrss-yt-captions"`); i > 0 {
		cut := strings.LastIndex(contentHTML[:i], "<section")
		if cut >= 0 {
			return strings.TrimSpace(contentHTML[:cut])
		}
	}
	return contentHTML
}

// Visible notice when captions cannot be fetched (bot check / no tracks / network).
const youtubeCaptionsMissHTML = `<section class="yt-captions-unavailable" data-yt-captions-miss="1">` +
	`<p class="yt-captions-miss-msg">未能获取字幕（视频可能没有字幕，或 YouTube 暂时拦截了自动抓取）。` +
	`可点击工具栏「请求全文」重试；若本机访问 YouTube 需代理，请确认代理对 LRSS 生效。</p>` +
	`</section>`

func markYouTubeCaptionsMiss(contentHTML string) string {
	if strings.Contains(contentHTML, `data-yt-captions-miss="1"`) {
		return contentHTML
	}
	if strings.TrimSpace(contentHTML) == "" {
		return youtubeCaptionsMissHTML
	}
	return contentHTML + "\n" + youtubeCaptionsMissHTML
}

func stripYouTubeCaptionsMiss(contentHTML string) string {
	// Remove both old hidden marker and the visible unavailable section.
	contentHTML = strings.ReplaceAll(contentHTML,
		`<div class="yt-captions-miss" data-yt-captions-miss="1" hidden></div>`, "")
	contentHTML = strings.ReplaceAll(contentHTML,
		`<div class="yt-captions-miss" data-yt-captions-miss="1" hidden=""></div>`, "")
	// Strip unavailable section if present.
	if i := strings.Index(contentHTML, `data-yt-captions-miss="1"`); i >= 0 {
		start := strings.LastIndex(contentHTML[:i], "<section")
		if start < 0 {
			start = strings.LastIndex(contentHTML[:i], "<div")
		}
		if start >= 0 {
			rest := contentHTML[i:]
			endRel := strings.Index(rest, "</section>")
			tagEnd := "</section>"
			if endRel < 0 {
				endRel = strings.Index(rest, "</div>")
				tagEnd = "</div>"
			}
			if endRel >= 0 {
				end := i + endRel + len(tagEnd)
				contentHTML = strings.TrimSpace(contentHTML[:start] + contentHTML[end:])
			}
		}
	}
	return contentHTML
}

// backfillYouTubeCaptionsForFeed updates stored articles that skipped insert (already
// present) but still lack captions, using URLs from the current refresh batch.
func (lib *Library) backfillYouTubeCaptionsForFeed(ctx context.Context, feedID string, items []repo.ParsedItem) {
	list, err := lib.Articles.List(ctx, "feed:"+feedID, repo.ListOpts{Limit: 50, Offset: 0})
	if err != nil {
		return
	}
	byURL := make(map[string]struct{}, len(items))
	for _, it := range items {
		if u := strings.TrimSpace(it.URL); u != "" {
			byURL[u] = struct{}{}
		}
	}
	for i := range list {
		a := &list[i]
		if _, ok := byURL[strings.TrimSpace(a.URL)]; !ok {
			continue
		}
		// Feed refresh may retry captions even after a prior miss.
		if a.ContentHTML != nil {
			stripped := stripYouTubeCaptionsMiss(*a.ContentHTML)
			a.ContentHTML = &stripped
		}
		a.FullContentFetched = false
		lib.ensureYouTubeCaptions(ctx, a)
	}
}

// ensureYouTubeCaptions fetches and persists captions when the article is a YouTube
// video and the body has no captions yet, or only plain (untimed) captions to upgrade.
// Skips miss-marked articles. On upgrade failure, keeps existing plain captions.
func (lib *Library) ensureYouTubeCaptions(ctx context.Context, a *model.Article) {
	if a == nil {
		return
	}
	vid := rss.YouTubeVideoID(a.URL)
	if vid == "" {
		return
	}
	htmlBody := ""
	if a.ContentHTML != nil {
		htmlBody = *a.ContentHTML
	}
	// Already timeline — nothing to do.
	if ytcaptions.HasTimedCaptions(htmlBody) {
		return
	}
	// Hard miss marker only — do not skip merely because full_content_fetched
	// (older builds stripped caption markers while still setting that flag).
	if strings.Contains(htmlBody, `data-yt-captions-miss="1"`) {
		return
	}

	hadPlain := ytcaptions.HasCaptionsSection(htmlBody)
	// Work on a copy; only persist on success (or first-time miss).
	work := htmlBody
	if hadPlain {
		// Drop untimed section so AppendHTML can rewrite with cues.
		work = stripYouTubeCaptionsSection(work)
	}
	work = ytcaptions.StripLegacyCaptions(work)
	if strings.TrimSpace(work) == "" {
		work = rss.YouTubeEmbedHTML(vid, "")
	}
	res, err := ytcaptions.Fetch(ctx, vid, ytcaptions.Options{Timeout: 12 * time.Second})
	if err != nil {
		// Had readable plain captions: keep them. Do NOT overwrite with miss UI.
		if hadPlain {
			return
		}
		san := lib.sanitizeHTML(markYouTubeCaptionsMiss(work))
		_ = lib.Articles.UpdateContent(ctx, a.ID, san, htmltext.ToText(san))
		a.ContentHTML = &san
		a.FullContentFetched = true
		return
	}
	merged := lib.sanitizeHTML(ytcaptions.AppendHTML(work, res))
	text := htmltext.ToText(merged)
	if err := lib.Articles.UpdateContent(ctx, a.ID, merged, text); err != nil {
		return
	}
	a.ContentHTML = &merged
	a.ContentText = &text
	a.FullContentFetched = true
}

// ListArticles returns a page of articles for a collection.
// excludeNsfw hides articles from is_nsfw feeds (office mode).
func (lib *Library) ListArticles(ctx context.Context, collection string, limit, offset int, excludeNsfw bool) ([]model.Article, error) {
	list, err := lib.Articles.List(ctx, collection, repo.ListOpts{
		Limit: limit, Offset: offset, ExcludeNsfw: excludeNsfw, Lite: true,
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
// For YouTube videos missing captions, best-effort fetch once and persist.
func (lib *Library) GetArticle(ctx context.Context, articleID string) (model.Article, error) {
	a, err := lib.Articles.Get(ctx, articleID)
	if err != nil {
		return model.Article{}, err
	}
	lib.ensureYouTubeCaptions(ctx, &a)
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
// YouTube watch URLs fetch captions instead of the watch page HTML.
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

	// YouTube: embed + captions (page readability is useless for watch pages).
	if vid := rss.YouTubeVideoID(pageURL); vid != "" {
		htmlBody := ""
		if a.ContentHTML != nil {
			htmlBody = *a.ContentHTML
		}
		htmlBody = stripYouTubeCaptionsMiss(htmlBody)
		if strings.TrimSpace(htmlBody) == "" || !strings.Contains(htmlBody, "youtube") {
			htmlBody = rss.YouTubeEmbedHTML(vid, "")
		}
		// Drop previous captions block so a manual re-fetch can refresh text.
		if ytcaptions.HasCaptionsSection(htmlBody) {
			if i := strings.Index(htmlBody, `data-yt-captions="1"`); i > 0 {
				cut := strings.LastIndex(htmlBody[:i], "<section")
				if cut >= 0 {
					htmlBody = strings.TrimSpace(htmlBody[:cut])
				}
			}
		}
		res, ferr := ytcaptions.Fetch(ctx, vid, ytcaptions.Options{Timeout: 18 * time.Second})
		if ferr != nil {
			return model.Article{}, fmt.Errorf("youtube captions: %w", ferr)
		}
		htmlBody = lib.sanitizeHTML(ytcaptions.AppendHTML(htmlBody, res))
		text := htmltext.ToText(htmlBody)
		if err := lib.Articles.UpdateContent(ctx, articleID, htmlBody, text); err != nil {
			return model.Article{}, err
		}
		// Avoid ensureYouTubeCaptions no-op path double work; load + sanitize only.
		out, gerr := lib.Articles.Get(ctx, articleID)
		if gerr != nil {
			return model.Article{}, gerr
		}
		lib.normalizeArticleForUI(&out, true)
		return out, nil
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
	// Same public-host policy as fulltext.Fetch (SSRF-style egress).
	if err := fulltext.ValidateFetchURL(raw); err != nil {
		return err
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

// RecordOpened notes that the reader opened an article and prunes the recent list.
// keep is the max recent-read rows (0 or negative → 50; repo also clamps).
func (lib *Library) RecordOpened(ctx context.Context, articleID string, keep int) error {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return fmt.Errorf("article id is required")
	}
	if keep <= 0 {
		keep = 50
	}
	return lib.Articles.RecordOpened(ctx, articleID, keep)
}

// PruneOpened drops recently-read rows beyond keep (Settings → General slider).
func (lib *Library) PruneOpened(ctx context.Context, keep int) error {
	if keep <= 0 {
		keep = 50
	}
	return lib.Articles.PruneOpened(ctx, keep)
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
