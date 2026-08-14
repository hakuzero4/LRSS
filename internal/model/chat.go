package model

// ChatSession is one reading-assistant conversation.
// Stage 1: one session per article_id.
type ChatSession struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	ArticleID    string `json:"articleId"`
	CollectionID string `json:"collectionId,omitempty"`
	Locale       string `json:"locale,omitempty"`
}

// ChatCitation maps a model [n] to a real article.
type ChatCitation struct {
	N         int    `json:"n"`
	ArticleID string `json:"articleId"`
	Title     string `json:"title"`
	FeedTitle string `json:"feedTitle,omitempty"`
}

// ChatMessage is one user or assistant turn.
type ChatMessage struct {
	ID        string         `json:"id"`
	SessionID string         `json:"sessionId"`
	Role      string         `json:"role"` // user | assistant
	Content   string         `json:"content"`
	Citations []ChatCitation `json:"citations,omitempty"`
	CreatedAt string         `json:"createdAt"`
}
