package model

// Folder groups feeds in the sidebar.
type Folder struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	ParentID  *string `json:"parentId,omitempty"`
	SortOrder int     `json:"sortOrder"`
	// IsNsfw marks a sensitive folder. When UIPrefs.nsfwMode is false, UI hides
	// this folder and its feeds; article lists/search also exclude them.
	IsNsfw    bool   `json:"isNsfw"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// Feed is an RSS/Atom subscription.
type Feed struct {
	ID            string  `json:"id"`
	FolderID      *string `json:"folderId,omitempty"`
	Title         string  `json:"title"`
	SiteURL       *string `json:"siteUrl,omitempty"`
	FeedURL       string  `json:"feedUrl"`
	FaviconURL    *string `json:"faviconUrl,omitempty"`
	ETag          *string `json:"etag,omitempty"`
	LastModified  *string `json:"lastModified,omitempty"`
	LastFetchedAt *string `json:"lastFetchedAt,omitempty"`
	LastError     *string `json:"lastError,omitempty"`
	IsPaused      bool    `json:"isPaused"`
	// RefreshIntervalMinutes is per-feed auto-refresh interval.
	// 0 means use the global LibraryConfig default.
	RefreshIntervalMinutes int `json:"refreshIntervalMinutes"`
	// KeepArticlesDays is per-feed retention for non-starred articles.
	// 0 means use the global UIPrefs keepArticlesDays; otherwise [7, 365].
	KeepArticlesDays int `json:"keepArticlesDays"`
	// TitleUserSet is true when the user renamed the feed; refresh must not overwrite title.
	TitleUserSet bool `json:"titleUserSet"`
	// IsNsfw marks a sensitive feed. When UIPrefs.nsfwMode is false, UI hides this feed.
	IsNsfw      bool `json:"isNsfw"`
	UnreadCount int  `json:"unreadCount"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// Article is a single feed item.
type Article struct {
	ID          string  `json:"id"`
	FeedID      string  `json:"feedId"`
	GUID        *string `json:"guid,omitempty"`
	URL         string  `json:"url"`
	Title       string  `json:"title"`
	Author      *string `json:"author,omitempty"`
	Summary     *string `json:"summary,omitempty"`
	ContentHTML *string `json:"contentHtml,omitempty"`
	ContentText *string `json:"contentText,omitempty"`
	// TranslationRaw is bilingual marker text (<<o>>/<<t>>) kept next to the original body.
	// Original content_html is never replaced by translate.
	TranslationRaw  *string `json:"translationRaw,omitempty"`
	TranslationLang *string `json:"translationLang,omitempty"`
	ImageURL        *string `json:"imageUrl,omitempty"`
	PublishedAt     *string `json:"publishedAt,omitempty"`
	FetchedAt       string  `json:"fetchedAt"`
	IsRead          bool    `json:"isRead"`
	IsStarred       bool    `json:"isStarred"`
	// FullContentFetched is true after a successful full-page fetch replaced the body.
	// Used to skip auto “partial body” detection on subsequent opens.
	FullContentFetched bool `json:"fullContentFetched"`
}

// EmbeddingStatus for article_embeddings.status.
type EmbeddingStatus string

const (
	EmbeddingPending EmbeddingStatus = "pending"
	EmbeddingReady   EmbeddingStatus = "ready"
	EmbeddingError   EmbeddingStatus = "error"
	EmbeddingSkipped EmbeddingStatus = "skipped"
)

// ArticleEmbedding stores a FLOAT32 vector blob for an article.
type ArticleEmbedding struct {
	ArticleID   string          `json:"articleId"`
	Model       string          `json:"model"`
	Dimensions  int             `json:"dimensions"`
	Embedding   []byte          `json:"-"`
	ContentHash string          `json:"contentHash"`
	Status      EmbeddingStatus `json:"status"`
	Error       *string         `json:"error,omitempty"`
	CreatedAt   string          `json:"createdAt"`
	UpdatedAt   string          `json:"updatedAt"`
}

// Job tracks background work (embed, fetch, opml_import).
type Job struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Payload   *string `json:"payload,omitempty"`
	Status    string  `json:"status"`
	Progress  float64 `json:"progress"`
	Error     *string `json:"error,omitempty"`
	CreatedAt string  `json:"createdAt"`
	UpdatedAt string  `json:"updatedAt"`
}
