package repo

import (
	"context"
	"database/sql"

	"lrss/internal/vector"
)

// EmbeddingEnabledFunc reports whether new articles should be queued for embedding.
// When nil, embedding enqueue is skipped.
type EmbeddingEnabledFunc func(ctx context.Context) bool

// Repos groups folder/feed/article repositories over one DB.
type Repos struct {
	Folders  *FolderRepo
	Feeds    *FeedRepo
	Articles *ArticleRepo
}

// Option configures Repos construction.
type Option func(*reposConfig)

type reposConfig struct {
	embeddingEnabled EmbeddingEnabledFunc
	vec              *vector.Index
}

// WithEmbeddingEnabled sets the callback used after article insert.
func WithEmbeddingEnabled(fn EmbeddingEnabledFunc) Option {
	return func(c *reposConfig) {
		c.embeddingEnabled = fn
	}
}

// WithVectorIndex injects a vector.Index (defaults to vector.NewIndex(db)).
func WithVectorIndex(idx *vector.Index) Option {
	return func(c *reposConfig) {
		c.vec = idx
	}
}

// New constructs folder/feed/article repositories sharing db.
func New(db *sql.DB, opts ...Option) *Repos {
	cfg := reposConfig{}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.vec == nil {
		cfg.vec = vector.NewIndex(db)
	}
	return &Repos{
		Folders: NewFolderRepo(db),
		Feeds:   NewFeedRepo(db),
		Articles: NewArticleRepo(db, ArticleRepoOpts{
			EmbeddingEnabled: cfg.embeddingEnabled,
			Vector:           cfg.vec,
		}),
	}
}
