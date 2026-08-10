package opml

// Document is a parsed or exportable OPML subscription tree.
type Document struct {
	Title    string
	Outlines []Outline
}

// Outline is a folder (children, no XMLURL) or a feed (non-empty XMLURL).
type Outline struct {
	Text     string // display name (text or title)
	Title    string
	Type     string // "rss" when feed
	XMLURL   string // feed URL
	HTMLURL  string // site URL
	Children []Outline
}
