package rss

import "time"

// ParsedFeed is a fetch/parse DTO (not a DB model).
type ParsedFeed struct {
	Title   string
	SiteURL string
	FeedURL string
	Items   []ParsedItem
}

// ParsedItem is a single feed entry DTO (not a DB model).
type ParsedItem struct {
	GUID        string
	URL         string
	Title       string
	Author      string
	Summary     string
	ContentHTML string
	ContentText string
	ImageURL    string
	PublishedAt *time.Time // nil if missing
}
