package appsvc

import (
	"context"
	"strings"

	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/service"
)

// FeedService is the Wails-facing feed API.
type FeedService struct {
	lib    *service.Library
	notify RefreshNotifier
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
func (s *FeedService) RefreshFeed(id string) (RefreshResult, error) {
	n, err := s.lib.RefreshFeed(context.Background(), id)
	if err != nil {
		return RefreshResult{}, err
	}
	s.afterRefresh(n)
	return RefreshResult{Added: n}, nil
}

// RefreshAllResult is returned by RefreshAll.
type RefreshAllResult struct {
	FeedsOK       int `json:"feedsOk"`
	FeedsErr      int `json:"feedsErr"`
	ArticlesAdded int `json:"articlesAdded"`
}

// RefreshAll refreshes all non-paused feeds.
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

// RenameFeed sets a custom display title (will not be overwritten by refresh).
func (s *FeedService) RenameFeed(id, title string) error {
	return s.lib.RenameFeed(context.Background(), id, title)
}

// SetFeedRefreshInterval sets per-feed auto-refresh minutes.
// 0 means follow the global default; otherwise clamped to [5, 180].
func (s *FeedService) SetFeedRefreshInterval(id string, minutes int) error {
	return s.lib.SetFeedRefreshInterval(context.Background(), id, minutes)
}

// OPMLImportResult is the Wails-facing import summary (mirrors service.OPMLImportResult).
type OPMLImportResult struct {
	FoldersCreated int      `json:"foldersCreated"`
	FeedsAdded     int      `json:"feedsAdded"`
	FeedsSkipped   int      `json:"feedsSkipped"`
	FeedsFailed    int      `json:"feedsFailed"`
	Errors         []string `json:"errors"`
	AddedFeedIDs   []string `json:"addedFeedIds"`
}

// ImportOPML imports an OPML document.
// Prefer fetch=false from the UI so the call returns after writing subscriptions;
// then refresh AddedFeedIDs with RefreshFeed for progress. fetch=true blocks until
// every new feed is fetched (slow for large OPML files).
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
	lib *service.Library
}

// NewArticleService wraps the library orchestrator.
func NewArticleService(lib *service.Library) *ArticleService {
	return &ArticleService{lib: lib}
}

// List returns articles for a collection (unread|today|starred|all|feed:ID|folder:ID).
func (s *ArticleService) List(collection string, limit, offset int) ([]model.Article, error) {
	return s.lib.ListArticles(context.Background(), collection, limit, offset)
}

// SmartCounts returns full library totals for sidebar badges (not capped by list limit).
func (s *ArticleService) SmartCounts() (repo.SmartCounts, error) {
	return s.lib.SmartCounts(context.Background())
}

// Get returns one article with sanitized HTML.
func (s *ArticleService) Get(id string) (model.Article, error) {
	return s.lib.GetArticle(context.Background(), id)
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
	return s.lib.MarkAllRead(context.Background(), collection)
}
