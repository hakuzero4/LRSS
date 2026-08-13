package appsvc

import (
	"context"
	"strings"
	"time"

	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/service"
	"lrss/internal/settings"
)

// FeedService is the Wails-facing feed API.
type FeedService struct {
	lib      *service.Library
	notify   RefreshNotifier
	briefing *service.BriefingWorker
}

// RefreshNotifier is called after a successful refresh with new article count.
// Implemented by *notify.Sender (optional).
type RefreshNotifier interface {
	AfterRefresh(ctx context.Context, articlesAdded int)
}

// NewFeedService wraps the library orchestrator.
func NewFeedService(lib *service.Library) *FeedService {
	return &FeedService{lib: lib}
}

// SetNotifier injects desktop notification hooks for refresh results.
//
//wails:ignore
func (s *FeedService) SetNotifier(n RefreshNotifier) {
	s.notify = n
}

// SetBriefingWorker lets JobActivity include smart-briefing progress.
//
//wails:ignore
func (s *FeedService) SetBriefingWorker(w *service.BriefingWorker) {
	if s == nil {
		return
	}
	s.briefing = w
}

// JobActivity is a live snapshot of feed refresh + briefing work.
func (s *FeedService) JobActivity() service.JobActivity {
	var a service.JobActivity
	if s == nil || s.lib == nil {
		return a
	}
	id, title, pending, queuedIDs := s.lib.RefreshSnapshot()
	a.FeedID = id
	a.FeedTitle = title
	a.Pending = pending
	a.Refreshing = id != "" || pending > 0
	if len(queuedIDs) > 0 {
		titles := make([]string, 0, len(queuedIDs))
		ctx := context.Background()
		for _, qid := range queuedIDs {
			f, err := s.lib.Feeds.Get(ctx, qid)
			if err != nil {
				continue
			}
			t := strings.TrimSpace(f.Title)
			if t == "" {
				t = f.FeedURL
			}
			if t != "" {
				titles = append(titles, t)
			}
		}
		a.QueuedTitles = titles
	}
	if s.briefing != nil {
		a.BriefingState, a.BriefingPending, a.BriefingArticles = s.briefing.Snapshot()
	}
	return a
}

func (s *FeedService) afterRefresh(added int) {
	if s == nil || s.notify == nil || added <= 0 {
		return
	}
	s.notify.AfterRefresh(context.Background(), added)
}

// ListFolders returns all folders.
func (s *FeedService) ListFolders() ([]model.Folder, error) {
	return s.lib.ListFolders(context.Background())
}

// ListFeeds returns all feeds with unread counts.
func (s *FeedService) ListFeeds() ([]model.Feed, error) {
	return s.lib.ListFeeds(context.Background())
}

// AddFeed subscribes to a feed URL. Empty folderId means no folder.
func (s *FeedService) AddFeed(feedURL string, folderId string) (model.Feed, error) {
	var folder *string
	if strings.TrimSpace(folderId) != "" {
		folderId = strings.TrimSpace(folderId)
		folder = &folderId
	}
	return s.lib.AddFeed(context.Background(), feedURL, folder)
}

// DeleteFeed removes a subscription.
func (s *FeedService) DeleteFeed(id string) error {
	return s.lib.DeleteFeed(context.Background(), id)
}

// ClearAllResult is returned by ClearAllSubscriptions.
type ClearAllResult struct {
	FeedsDeleted   int `json:"feedsDeleted"`
	FoldersDeleted int `json:"foldersDeleted"`
}

// ClearAllSubscriptions removes every feed, article, and folder. Irreversible.
func (s *FeedService) ClearAllSubscriptions() (ClearAllResult, error) {
	res, err := s.lib.ClearAllSubscriptions(context.Background())
	if err != nil {
		return ClearAllResult{}, err
	}
	return ClearAllResult{
		FeedsDeleted:   res.FeedsDeleted,
		FoldersDeleted: res.FoldersDeleted,
	}, nil
}

// RefreshResult is returned by RefreshFeed.
type RefreshResult struct {
	Added int `json:"added"`
}

// RefreshFeed re-fetches one feed.
// Bounded so a hung remote feed cannot leave the UI spinner spinning forever.
func (s *FeedService) RefreshFeed(id string) (RefreshResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	n, err := s.lib.RefreshFeed(ctx, id)
	if err != nil {
		return RefreshResult{}, err
	}
	s.afterRefresh(n)
	return RefreshResult{Added: n}, nil
}

// RefreshAllResult is returned by RefreshAll (one paced batch; rest may be pending).
type RefreshAllResult struct {
	FeedsOK       int `json:"feedsOk"`
	FeedsErr      int `json:"feedsErr"`
	ArticlesAdded int `json:"articlesAdded"`
	// FeedsPending is force-queue remaining after this call (background continues).
	FeedsPending int `json:"feedsPending"`
}

// RefreshAll queues every active feed and refreshes one paced batch (same cap
// as auto-refresh). Remaining feeds drain in the background loop.
func (s *FeedService) RefreshAll() (RefreshAllResult, error) {
	res, err := s.lib.RefreshAll(context.Background())
	if err != nil {
		return RefreshAllResult{}, err
	}
	s.afterRefresh(res.ArticlesAdded)
	return RefreshAllResult{
		FeedsOK:       res.FeedsOK,
		FeedsErr:      res.FeedsErr,
		ArticlesAdded: res.ArticlesAdded,
		FeedsPending:  res.FeedsPending,
	}, nil
}

// CreateFolder creates a folder. Empty parentId means root.
func (s *FeedService) CreateFolder(name, parentId string) (model.Folder, error) {
	var parent *string
	if strings.TrimSpace(parentId) != "" {
		parentId = strings.TrimSpace(parentId)
		parent = &parentId
	}
	return s.lib.CreateFolder(context.Background(), name, parent)
}

// RenameFolder renames a folder.
func (s *FeedService) RenameFolder(id, name string) error {
	return s.lib.RenameFolder(context.Background(), id, name)
}

// DeleteFolder removes a folder (feeds become unfiled).
func (s *FeedService) DeleteFolder(id string) error {
	return s.lib.DeleteFolder(context.Background(), id)
}

// MoveFeed assigns a feed to a folder. Empty folderId means unfiled.
func (s *FeedService) MoveFeed(feedId, folderId string) error {
	var folder *string
	if strings.TrimSpace(folderId) != "" {
		folderId = strings.TrimSpace(folderId)
		folder = &folderId
	}
	return s.lib.MoveFeed(context.Background(), feedId, folder)
}

// SetFeedPaused pauses or unpauses a feed.
func (s *FeedService) SetFeedPaused(id string, paused bool) error {
	return s.lib.SetPaused(context.Background(), id, paused)
}

// SetFeedNsfw marks or unmarks a feed as sensitive (NSFW).
func (s *FeedService) SetFeedNsfw(id string, nsfw bool) error {
	return s.lib.SetFeedNsfw(context.Background(), id, nsfw)
}

// SetFolderNsfw marks or unmarks a folder as sensitive (NSFW).
// In office mode the folder and its feeds are hidden from the sidebar and lists.
func (s *FeedService) SetFolderNsfw(id string, nsfw bool) error {
	return s.lib.SetFolderNsfw(context.Background(), id, nsfw)
}

// SetFolderDisplayMode sets the article list layout for a folder: "list" or "cards".
func (s *FeedService) SetFolderDisplayMode(id, mode string) error {
	return s.lib.SetFolderDisplayMode(context.Background(), id, mode)
}

// RenameFeed sets a custom display title (will not be overwritten by refresh).
func (s *FeedService) RenameFeed(id, title string) error {
	return s.lib.RenameFeed(context.Background(), id, title)
}

// SetFeedURL updates the subscription feed URL (http/https). Keeps articles;
// rejects URLs already used by another feed.
func (s *FeedService) SetFeedURL(id, feedURL string) error {
	return s.lib.SetFeedURL(context.Background(), id, feedURL)
}

// SetFeedRefreshInterval sets per-feed auto-refresh minutes.
// 0 means follow the global default; otherwise clamped to [5, 180].
func (s *FeedService) SetFeedRefreshInterval(id string, minutes int) error {
	return s.lib.SetFeedRefreshInterval(context.Background(), id, minutes)
}

// SetFeedKeepArticlesDays sets per-feed article retention days.
// 0 = use global UIPrefs keepArticlesDays; otherwise clamped to [7, 365].
func (s *FeedService) SetFeedKeepArticlesDays(id string, days int) error {
	return s.lib.SetFeedKeepArticlesDays(context.Background(), id, days)
}

// OPMLImportResult is the Wails-facing import summary (mirrors service.OPMLImportResult).
type OPMLImportResult struct {
	FoldersCreated int      `json:"foldersCreated"`
	FeedsAdded     int      `json:"feedsAdded"`
	FeedsUpdated   int      `json:"feedsUpdated"`
	FeedsSkipped   int      `json:"feedsSkipped"`
	FeedsFailed    int      `json:"feedsFailed"`
	Errors         []string `json:"errors"`
	AddedFeedIDs   []string `json:"addedFeedIds"`
}

// ImportOPML imports an OPML document.
// Prefer fetch=false from the UI so the call returns after writing subscriptions;
// then refresh AddedFeedIDs with RefreshFeed for progress. fetch=true blocks until
// every new feed is fetched (slow for large OPML files).
// Existing feed URLs are merged (folder / unlocked title / empty site URL), not ignored.
func (s *FeedService) ImportOPML(xml string, fetch bool) (OPMLImportResult, error) {
	res, err := s.lib.ImportOPML(context.Background(), xml, fetch)
	if err != nil {
		return OPMLImportResult{}, err
	}
	ids := res.AddedFeedIDs
	if ids == nil {
		ids = []string{}
	}
	return OPMLImportResult{
		FoldersCreated: res.FoldersCreated,
		FeedsAdded:     res.FeedsAdded,
		FeedsUpdated:   res.FeedsUpdated,
		FeedsSkipped:   res.FeedsSkipped,
		FeedsFailed:    res.FeedsFailed,
		Errors:         res.Errors,
		AddedFeedIDs:   ids,
	}, nil
}

// ExportOPML returns the subscription tree as OPML 2.0 XML text.
func (s *FeedService) ExportOPML() (string, error) {
	return s.lib.ExportOPML(context.Background())
}

// ArticleService is the Wails-facing article API.
type ArticleService struct {
	lib   *service.Library
	store *settings.Store
}

// NewArticleService wraps the library orchestrator.
// store may be nil (tests); nsfwMode defaults to show-all when missing.
func NewArticleService(lib *service.Library, store *settings.Store) *ArticleService {
	return &ArticleService{lib: lib, store: store}
}

// excludeNsfw is true when UIPrefs.nsfwMode is false (office mode).
func (s *ArticleService) excludeNsfw() bool {
	if s == nil || s.store == nil {
		return false
	}
	prefs, err := s.store.LoadUIPrefs(context.Background())
	if err != nil {
		return false
	}
	return !prefs.NsfwMode
}

// List returns articles for a collection (unread|today|starred|all|recent|feed:ID|folder:ID).
// Office-mode NSFW filtering applies only to smart lists. Explicit feed:/folder:
// collections always return their articles so a just-subscribed sensitive feed is
// still readable after add (sidebar may still hide it).
func (s *ArticleService) List(collection string, limit, offset int) ([]model.Article, error) {
	exclude := s.excludeNsfw() && isSmartCollection(collection)
	return s.lib.ListArticles(context.Background(), collection, limit, offset, exclude)
}

func isSmartCollection(collection string) bool {
	switch strings.TrimSpace(collection) {
	case "", "unread", "today", "starred", "all", "recent":
		return true
	default:
		return false
	}
}

// SmartCounts returns full library totals for sidebar badges (not capped by list limit).
func (s *ArticleService) SmartCounts() (repo.SmartCounts, error) {
	return s.lib.SmartCounts(context.Background(), s.excludeNsfw())
}

// Get returns one article with sanitized HTML.
func (s *ArticleService) Get(id string) (model.Article, error) {
	return s.lib.GetArticle(context.Background(), id)
}

// FetchFullContent downloads the original article page (fingerprint HTTP / surf),
// extracts full HTML body, saves it, and returns the updated article.
// Use when the feed only ships a partial summary in XML.
// Bounded so a hung remote page cannot block other ArticleService calls forever.
func (s *ArticleService) FetchFullContent(id string) (model.Article, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	return s.lib.FetchFullContent(ctx, id)
}

// RecordOpened records that the reader opened an article (recent-read list).
// keep comes from UIPrefs.RecentReadLimit (default 50 if store is missing or load fails).
func (s *ArticleService) RecordOpened(id string) error {
	keep := 50
	if s != nil && s.store != nil {
		if prefs, err := s.store.LoadUIPrefs(context.Background()); err == nil {
			keep = prefs.RecentReadLimit
		}
	}
	return s.lib.RecordOpened(context.Background(), id, keep)
}

// SetRead marks read state.
func (s *ArticleService) SetRead(id string, read bool) error {
	return s.lib.SetRead(context.Background(), id, read)
}

// SetStarred marks starred state.
func (s *ArticleService) SetStarred(id string, starred bool) error {
	return s.lib.SetStarred(context.Background(), id, starred)
}

// MarkAllRead marks a collection as read.
func (s *ArticleService) MarkAllRead(collection string) error {
	return s.lib.MarkAllRead(context.Background(), collection, s.excludeNsfw())
}
