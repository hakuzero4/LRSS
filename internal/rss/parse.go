package rss

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"time"
	"unicode"

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
	// Always plain text for summary (RSS Description is often HTML).
	summary := htmltext.ToText(desc)
	if summary == "" && contentHTML != "" {
		summary = htmltext.ToText(contentHTML)
	}
	summary = truncateRunes(summary, 320)

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
	// Some feeds (e.g. Google Blog) put person markup in author:
	// <name>…</name><title>…</title><department>…</department>
	author = normalizeAuthor(author)

	pi := ParsedItem{
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
	// YouTube Atom (and some media RSS) store description/thumbnail only under
	// media:group / yt:videoId — not item.Content / Description.
	enrichFromMediaExtensions(item, &pi)
	return pi
}

func bestContentHTML(item *gofeed.Item) string {
	if c := strings.TrimSpace(item.Content); c != "" {
		return c
	}
	return strings.TrimSpace(item.Description)
}

// normalizeAuthor turns RSS author fields into plain display text.
// Handles Google-style person fragments that still contain XML tags.
func normalizeAuthor(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "<") {
		return raw
	}
	// Prefer structured person fields when present.
	var parts []string
	for _, tag := range []string{"name", "title", "department", "company", "email"} {
		if v := extractSimpleTag(raw, tag); v != "" {
			parts = append(parts, v)
		}
	}
	if len(parts) > 0 {
		return strings.Join(parts, " · ")
	}
	// Generic HTML/XML → plain text (may concatenate tags with spaces via ToText).
	return strings.TrimSpace(htmltext.ToText(raw))
}

// extractSimpleTag returns the first non-empty text of <tag>...</tag> (case-insensitive).
func extractSimpleTag(raw, tag string) string {
	lower := strings.ToLower(raw)
	open := "<" + strings.ToLower(tag)
	idx := strings.Index(lower, open)
	if idx < 0 {
		return ""
	}
	// Skip to end of opening tag (allow attributes).
	rest := raw[idx:]
	gt := strings.IndexByte(rest, '>')
	if gt < 0 {
		return ""
	}
	// Self-closing or empty.
	if gt > 0 && rest[gt-1] == '/' {
		return ""
	}
	inner := rest[gt+1:]
	close := "</" + tag
	// case-insensitive close search
	innerLower := strings.ToLower(inner)
	cidx := strings.Index(innerLower, strings.ToLower(close))
	if cidx < 0 {
		return ""
	}
	return strings.TrimSpace(htmltext.ToText(inner[:cidx]))
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	// Prefer break at word boundary near the cut.
	cut := max
	for i := max; i > max*3/4; i-- {
		if unicode.IsSpace(r[i-1]) {
			cut = i
			break
		}
	}
	return strings.TrimSpace(string(r[:cut])) + "…"
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
