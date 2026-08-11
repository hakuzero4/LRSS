package rss

import (
	"html"
	"net/url"
	"regexp"
	"strings"

	"github.com/mmcdole/gofeed"
	gfext "github.com/mmcdole/gofeed/extensions"
)

// youtubeVideoIDRe matches common YouTube watch / short / embed IDs.
var youtubeVideoIDRe = regexp.MustCompile(`(?i)(?:youtube(?:-nocookie)?\.com/(?:watch\?(?:[^#]*&)?v=|embed/|shorts/|v/)|youtu\.be/)([A-Za-z0-9_-]{6,})`)

// youtubeIDFromItem prefers yt:videoId extension, then link / media URL.
func youtubeIDFromItem(item *gofeed.Item, linkURL string) string {
	if item != nil {
		if id := extensionText(item, "yt", "videoId"); id != "" {
			return id
		}
		if id := youtubeIDFromURL(mediaGroupChildAttr(item, "content", "url")); id != "" {
			return id
		}
	}
	return youtubeIDFromURL(linkURL)
}

// youtubeIDFromURL extracts a video id from a YouTube URL (watch, embed, shorts, youtu.be).
func youtubeIDFromURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if m := youtubeVideoIDRe.FindStringSubmatch(raw); len(m) == 2 {
		return m[1]
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Host)
	if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtube-nocookie.com") {
		if v := strings.TrimSpace(u.Query().Get("v")); youtubeIDOK(v) {
			return v
		}
	}
	return ""
}

func youtubeIDOK(id string) bool {
	if len(id) < 6 || len(id) > 20 {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func extensionText(item *gofeed.Item, ns, name string) string {
	if item == nil || item.Extensions == nil {
		return ""
	}
	m, ok := item.Extensions[ns]
	if !ok {
		return ""
	}
	list := m[name]
	if len(list) == 0 {
		return ""
	}
	return strings.TrimSpace(list[0].Value)
}

func mediaGroupExtensions(item *gofeed.Item) []gfext.Extension {
	if item == nil || item.Extensions == nil {
		return nil
	}
	media, ok := item.Extensions["media"]
	if !ok {
		return nil
	}
	return media["group"]
}

func mediaGroupChild(item *gofeed.Item, child string) *gfext.Extension {
	for _, g := range mediaGroupExtensions(item) {
		list := g.Children[child]
		if len(list) == 0 {
			for k, v := range g.Children {
				if strings.EqualFold(k, child) && len(v) > 0 {
					list = v
					break
				}
			}
		}
		if len(list) > 0 {
			c := list[0]
			return &c
		}
	}
	return nil
}

func mediaDescription(item *gofeed.Item) string {
	if c := mediaGroupChild(item, "description"); c != nil {
		return strings.TrimSpace(c.Value)
	}
	return ""
}

func mediaThumbnailURL(item *gofeed.Item) string {
	return mediaGroupChildAttr(item, "thumbnail", "url")
}

func mediaGroupChildAttr(item *gofeed.Item, child, attr string) string {
	c := mediaGroupChild(item, child)
	if c == nil {
		return ""
	}
	for ak, av := range c.Attrs {
		if strings.EqualFold(ak, attr) {
			return strings.TrimSpace(av)
		}
	}
	return ""
}

// buildYouTubeContentHTML builds a privacy-friendly embed + optional description.
// Description is escaped plain text (YouTube media:description is not trusted HTML).
func buildYouTubeContentHTML(videoID, description string) string {
	if !youtubeIDOK(videoID) {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="yt-embed">`)
	b.WriteString(`<iframe src="https://www.youtube-nocookie.com/embed/`)
	b.WriteString(html.EscapeString(videoID))
	b.WriteString(`" title="YouTube video" loading="lazy" allowfullscreen `)
	b.WriteString(`allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" `)
	b.WriteString(`referrerpolicy="strict-origin-when-cross-origin"></iframe>`)
	b.WriteString(`</div>`)
	desc := strings.TrimSpace(description)
	if desc != "" {
		b.WriteString(`<div class="yt-desc">`)
		parts := strings.Split(desc, "\n")
		b.WriteString("<p>")
		for i, line := range parts {
			if i > 0 {
				if strings.TrimSpace(line) == "" {
					b.WriteString("</p><p>")
					continue
				}
				b.WriteString("<br>")
			}
			b.WriteString(html.EscapeString(line))
		}
		b.WriteString("</p>")
		b.WriteString(`</div>`)
	}
	return b.String()
}

// enrichFromMediaExtensions fills empty body / image from YouTube Atom media:group + yt:videoId.
func enrichFromMediaExtensions(item *gofeed.Item, pi *ParsedItem) {
	if item == nil || pi == nil {
		return
	}
	videoID := youtubeIDFromItem(item, pi.URL)
	desc := mediaDescription(item)
	if desc == "" {
		desc = strings.TrimSpace(item.Description)
	}
	thumb := mediaThumbnailURL(item)

	if pi.ImageURL == "" && thumb != "" {
		pi.ImageURL = thumb
	}

	if strings.TrimSpace(pi.ContentHTML) != "" {
		if videoID != "" && !strings.Contains(pi.ContentHTML, "youtube.com/embed/") &&
			!strings.Contains(pi.ContentHTML, "youtube-nocookie.com/embed/") {
			pi.ContentHTML = buildYouTubeContentHTML(videoID, "") + pi.ContentHTML
		}
		return
	}

	if videoID != "" {
		pi.ContentHTML = buildYouTubeContentHTML(videoID, desc)
		if pi.Summary == "" && desc != "" {
			pi.Summary = truncateRunes(strings.Join(strings.Fields(desc), " "), 320)
		}
		if pi.ContentText == "" && desc != "" {
			pi.ContentText = desc
		}
		return
	}

	if desc != "" {
		pi.ContentHTML = "<p>" + html.EscapeString(desc) + "</p>"
		if pi.ContentText == "" {
			pi.ContentText = desc
		}
		if pi.Summary == "" {
			pi.Summary = truncateRunes(strings.Join(strings.Fields(desc), " "), 320)
		}
	}
}
