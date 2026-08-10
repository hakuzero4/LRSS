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
	Delete(ctx context.Context, feedID string) error
}

// ArticleStore is the article persistence port (satisfied by *repo.ArticleRepo).
type ArticleStore interface {
	List(ctx context.Context, collection string, opts repo.ListOpts) ([]model.Article, error)
	Get(ctx context.Context, articleID string) (model.Article, error)
	UpsertFromParsed(ctx context.Context, feedID string, items []repo.ParsedItem) (repo.UpsertResult, error)
	SetRead(ctx context.Context, articleID string, read bool) error
	SetStarred(ctx context.Context, articleID string, starred bool) error
	MarkAllRead(ctx context.Context, collection string) error
}

// FolderStore lists folders (satisfied by *repo.FolderRepo).
type FolderStore interface {
	List(ctx context.Context) ([]model.Folder, error)
}

// RSSFetcher fetches feeds (satisfied by *rss.Client).
type RSSFetcher interface {
	Fetch(ctx context.Context, feedURL string, opts rss.FetchOptions) (*rss.FetchResult, error)
}
