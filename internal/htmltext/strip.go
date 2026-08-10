package htmltext

import (
	"html"
	"strings"
	"unicode"

	xhtml "golang.org/x/net/html"
)

// blockTags force a space boundary when the element ends.
var blockTags = map[string]bool{
	"p": true, "div": true, "br": true, "hr": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"li": true, "tr": true, "blockquote": true, "pre": true,
	"section": true, "article": true, "header": true, "footer": true,
	"ul": true, "ol": true, "table": true, "td": true, "th": true,
}

// skipTags and their descendants contribute no text.
var skipTags = map[string]bool{
	"script": true, "style": true, "noscript": true,
}

// ToText strips HTML tags, decodes entities, and collapses whitespace.
func ToText(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}

	doc, err := xhtml.Parse(strings.NewReader(raw))
	if err != nil {
		return collapseSpace(html.UnescapeString(stripTagsFallback(raw)))
	}

	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		switch n.Type {
		case xhtml.TextNode:
			b.WriteString(n.Data)
		case xhtml.ElementNode:
			tag := strings.ToLower(n.Data)
			if skipTags[tag] {
				return
			}
			if tag == "br" {
				b.WriteByte(' ')
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if blockTags[tag] {
				b.WriteByte(' ')
			}
			return
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}
	}
	walk(doc)

	return collapseSpace(html.UnescapeString(b.String()))
}

func collapseSpace(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevSpace := true // trim leading
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(b.String())
}

func stripTagsFallback(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}
