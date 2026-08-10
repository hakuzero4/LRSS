package appsvc

import (
	"context"
	"strings"

	"lrss/internal/model"
	"lrss/internal/service"
)

// FeedService is the Wails-facing feed API.
type FeedService struct {
	lib *service.Library
}

// NewFeedService wraps the library orchestrator.
func NewFeedService(lib *service.Library) *FeedService {
	return &FeedService{lib: lib}
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
	return RefreshAllResult{
		FeedsOK:       res.FeedsOK,
		FeedsErr:      res.FeedsErr,
		ArticlesAdded: res.ArticlesAdded,
	}, nil
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
