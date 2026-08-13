package model

// BriefingCite is one source article under a briefing point.
type BriefingCite struct {
	ArticleID string `json:"articleId"`
	Title     string `json:"title"`
	FeedTitle string `json:"feedTitle"`
}

// BriefingBullet is one insight. Cites holds all sources (do not duplicate the point).
type BriefingBullet struct {
	Point     string         `json:"point"`
	ArticleID string         `json:"articleId"`
	Title     string         `json:"title"`
	FeedTitle string         `json:"feedTitle"`
	Cites     []BriefingCite `json:"cites,omitempty"`
}

// BriefingTheme groups related bullets under a heading.
type BriefingTheme struct {
	Title   string           `json:"title"`
	Bullets []BriefingBullet `json:"bullets"`
}

// BriefingPayload is the generated briefing body stored as payload_json.
type BriefingPayload struct {
	Overview  string           `json:"overview"`
	Themes    []BriefingTheme  `json:"themes"`
	Watch     []BriefingBullet `json:"watch"`
	SourceIDs []string         `json:"sourceIds,omitempty"`
}

// Briefing is one stored AI briefing (智能汇报).
// Status is pending | ready | error.
type Briefing struct {
	ID           string          `json:"id"`
	CreatedAt    string          `json:"createdAt"`
	Status       string          `json:"status"`
	Locale       string          `json:"locale"`
	Model        string          `json:"model,omitempty"`
	Overview     string          `json:"overview"`
	Error        string          `json:"error,omitempty"`
	ArticleCount int             `json:"articleCount"`
	OmittedCount int             `json:"omittedCount"`
	IsRead       bool            `json:"isRead"`
	IsStarred    bool            `json:"isStarred"`
	Payload      BriefingPayload `json:"payload"`
}
