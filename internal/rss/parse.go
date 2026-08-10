package rss

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"lrss/internal/htmltext"
)

// ToParsedFeed maps a gofeed.Feed into our DTO with normalization rules applied.
func ToParsedFeed(feed *gofeed.Feed, feedURL string) *ParsedFeed {
	if feed == nil {
		return &ParsedFeed{FeedURL: feedURL}
	}

	siteURL := strings.TrimSpace(feed.Link)
	baseForResolve := siteURL
	if baseForResolve == "" {
		baseForResolve = feedURL
	}

	out := &ParsedFeed{
		Title:   strings.TrimSpace(feed.Title),
		SiteURL: siteURL,
		FeedURL: feedURL,
		Items:   make([]ParsedItem, 0, len(feed.Items)),
	}

	for _, item := range feed.Items {
		if item == nil {
			continue
		}
		out.Items = append(out.Items, mapItem(item, baseForResolve, feedURL))
	}
	return out
}

func mapItem(item *gofeed.Item, baseURL, feedURL string) ParsedItem {
	title := strings.TrimSpace(item.Title)
	link := strings.TrimSpace(item.Link)
	absURL := resolveURL(baseURL, link)
	if absURL == "" {
		absURL = resolveURL(feedURL, link)
	}

	guid := strings.TrimSpace(item.GUID)
	if guid == "" {
		guid = absURL
	}
	if guid == "" {
		guid = stableHash(title, item)
	}

	contentHTML := bestContentHTML(item)
	desc := strings.TrimSpace(item.Description)
	summary := desc
	if summary == "" && contentHTML != "" {
		summary = htmltext.ToText(contentHTML)
	}

	textSrc := contentHTML
	if textSrc == "" {
		textSrc = desc
	}
	contentText := htmltext.ToText(textSrc)

	imageURL := imageFromEnclosures(item)
	if imageURL == "" {
		imageURL = firstImgSrc(contentHTML)
	}
	if imageURL == "" {
		imageURL = firstImgSrc(desc)
	}
	if imageURL != "" {
		imageURL = resolveURL(baseURL, imageURL)
	}

	var publishedAt *time.Time
	if item.PublishedParsed != nil {
		t := item.PublishedParsed.UTC()
		publishedAt = &t
	} else if item.UpdatedParsed != nil {
		t := item.UpdatedParsed.UTC()
		publishedAt = &t
	}

	author := ""
	if item.Author != nil {
		author = strings.TrimSpace(item.Author.Name)
		if author == "" {
			author = strings.TrimSpace(item.Author.Email)
		}
	}
	if author == "" && len(item.Authors) > 0 && item.Authors[0] != nil {
		author = strings.TrimSpace(item.Authors[0].Name)
	}

	return ParsedItem{
		GUID:        guid,
		URL:         absURL,
		Title:       title,
		Author:      author,
		Summary:     summary,
		ContentHTML: contentHTML,
		ContentText: contentText,
		ImageURL:    imageURL,
		PublishedAt: publishedAt,
	}
}

func bestContentHTML(item *gofeed.Item) string {
	if c := strings.TrimSpace(item.Content); c != "" {
		return c
	}
	return strings.TrimSpace(item.Description)
}

func stableHash(title string, item *gofeed.Item) string {
	pub := ""
	if item.PublishedParsed != nil {
		pub = item.PublishedParsed.UTC().Format(time.RFC3339)
	} else if item.Published != "" {
		pub = item.Published
	} else if item.UpdatedParsed != nil {
		pub = item.UpdatedParsed.UTC().Format(time.RFC3339)
	} else if item.Updated != "" {
		pub = item.Updated
	}
	sum := sha256.Sum256([]byte(title + "|" + pub))
	return hex.EncodeToString(sum[:])
}

func resolveURL(base, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if u.IsAbs() {
		return u.String()
	}
	base = strings.TrimSpace(base)
	if base == "" {
		return ref
	}
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	return b.ResolveReference(u).String()
}

func imageFromEnclosures(item *gofeed.Item) string {
	for _, enc := range item.Enclosures {
		if enc == nil {
			continue
		}
		t := strings.ToLower(strings.TrimSpace(enc.Type))
		if strings.HasPrefix(t, "image/") {
			if u := strings.TrimSpace(enc.URL); u != "" {
				return u
			}
		}
	}
	for _, enc := range item.Enclosures {
		if enc == nil {
			continue
		}
		u := strings.TrimSpace(enc.URL)
		if u == "" {
			continue
		}
		lower := strings.ToLower(u)
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"} {
			if strings.Contains(lower, ext) {
				return u
			}
		}
	}
	return ""
}

// firstImgSrc finds the first <img src="..."> in HTML (case-insensitive, basic).
func firstImgSrc(htmlStr string) string {
	if htmlStr == "" {
		return ""
	}
	lower := strings.ToLower(htmlStr)
	idx := strings.Index(lower, "<img")
	if idx < 0 {
		return ""
	}
	rest := htmlStr[idx:]
	restLower := strings.ToLower(rest)
	srcIdx := strings.Index(restLower, "src=")
	if srcIdx < 0 {
		return ""
	}
	rest = rest[srcIdx+4:]
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return ""
	}
	quote := rest[0]
	if quote == '"' || quote == '\'' {
		rest = rest[1:]
		end := strings.IndexByte(rest, quote)
		if end < 0 {
			return ""
		}
		return strings.TrimSpace(rest[:end])
	}
	end := strings.IndexAny(rest, " \t\n\r>")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}
