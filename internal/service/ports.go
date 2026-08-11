package service

import (
	"context"

	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/rss"
)

// FeedStore is the feed persistence port (satisfied by *repo.FeedRepo).
type FeedStore interface {
	List(ctx context.Context) ([]model.Feed, error)
	ListActive(ctx context.Context) ([]model.Feed, error)
	Get(ctx context.Context, feedID string) (model.Feed, error)
	GetByURL(ctx context.Context, feedURL string) (model.Feed, error)
	Insert(ctx context.Context, f *model.Feed) error
	UpdateAfterFetch(ctx context.Context, feedID string, title string, etag, lastModified, lastError *string) error
	SetFolder(ctx context.Context, feedID string, folderID *string) error
	SetPaused(ctx context.Context, feedID string, paused bool) error
	SetTitle(ctx context.Context, feedID, title string) error
	SetRefreshInterval(ctx context.Context, feedID string, minutes int) error
	SetNsfw(ctx context.Context, feedID string, nsfw bool) error
	SetSiteURL(ctx context.Context, feedID, siteURL string) error
	SetFaviconURL(ctx context.Context, feedID, faviconURL string) error
	Delete(ctx context.Context, feedID string) error
	DeleteAll(ctx context.Context) (int, error)
}

// ArticleStore is the article persistence port (satisfied by *repo.ArticleRepo).
type ArticleStore interface {
	List(ctx context.Context, collection string, opts repo.ListOpts) ([]model.Article, error)
	Get(ctx context.Context, articleID string) (model.Article, error)
	CountByFeed(ctx context.Context, feedID string) (int, error)
	UpsertFromParsed(ctx context.Context, feedID string, items []repo.ParsedItem) (repo.UpsertResult, error)
	UpdateContent(ctx context.Context, articleID, contentHTML, contentText string) error
	SetRead(ctx context.Context, articleID string, read bool) error
	SetStarred(ctx context.Context, articleID string, starred bool) error
	MarkAllRead(ctx context.Context, collection string, excludeNsfw bool) error
	PurgeOlderThan(ctx context.Context, days int) (int, error)
	CountSmart(ctx context.Context, excludeNsfw bool) (repo.SmartCounts, error)
}

// FulltextFetcher downloads a page and extracts readable HTML (tests can stub).
type FulltextFetcher interface {
	Fetch(ctx context.Context, pageURL string) (html string, text string, err error)
}

// FolderStore is the folder persistence port (satisfied by *repo.FolderRepo).
type FolderStore interface {
	List(ctx context.Context) ([]model.Folder, error)
	Create(ctx context.Context, name string, parentID *string) (model.Folder, error)
	Get(ctx context.Context, folderID string) (model.Folder, error)
	Rename(ctx context.Context, folderID, name string) error
	SetNsfw(ctx context.Context, folderID string, nsfw bool) error
	Delete(ctx context.Context, folderID string) error
	DeleteAll(ctx context.Context) (int, error)
}

// RSSFetcher fetches feeds (satisfied by *rss.Client).
type RSSFetcher interface {
	Fetch(ctx context.Context, feedURL string, opts rss.FetchOptions) (*rss.FetchResult, error)
}
