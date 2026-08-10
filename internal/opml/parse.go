package opml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/url"
	"strings"
)

const maxOutlineDepth = 16

// Parse reads OPML 1.0/2.0 XML and returns a Document tree.
// Invalid individual feed outlines are skipped; empty input and malformed XML fail.
func Parse(data []byte) (*Document, error) {
	data = bytes.TrimSpace(data)
	// Strip UTF-8 BOM if present.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if len(data) == 0 {
		return nil, fmt.Errorf("opml: empty input")
	}

	var raw rawOPML
	dec := xml.NewDecoder(bytes.NewReader(data))
	// OPML often omits namespaces; keep default strictness for structure.
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("opml: parse: %w", err)
	}

	doc := &Document{
		Title:    strings.TrimSpace(raw.Head.Title),
		Outlines: make([]Outline, 0, len(raw.Body.Outlines)),
	}
	for i := range raw.Body.Outlines {
		out, err := convertOutline(&raw.Body.Outlines[i], 1)
		if err != nil {
			return nil, err
		}
		if out != nil {
			doc.Outlines = append(doc.Outlines, *out)
		}
	}
	return doc, nil
}

type rawOPML struct {
	XMLName xml.Name    `xml:"opml"`
	Head    rawHead     `xml:"head"`
	Body    rawBody     `xml:"body"`
}

type rawHead struct {
	Title string `xml:"title"`
}

type rawBody struct {
	Outlines []rawOutline `xml:"outline"`
}

// rawOutline unmarshals outline elements with case-insensitive attribute names.
type rawOutline struct {
	Text     string
	Title    string
	Type     string
	XMLURL   string
	HTMLURL  string
	Children []rawOutline
}

func (o *rawOutline) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch strings.ToLower(attr.Name.Local) {
		case "text":
			o.Text = attr.Value
		case "title":
			o.Title = attr.Value
		case "type":
			o.Type = attr.Value
		case "xmlurl":
			o.XMLURL = attr.Value
		case "htmlurl":
			o.HTMLURL = attr.Value
		}
	}

	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if strings.EqualFold(t.Name.Local, "outline") {
				var child rawOutline
				if err := d.DecodeElement(&child, &t); err != nil {
					return err
				}
				o.Children = append(o.Children, child)
			} else {
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// convertOutline maps a raw outline into a feed, folder, or nil (ignored).
// depth is 1 for top-level body children.
func convertOutline(raw *rawOutline, depth int) (*Outline, error) {
	if depth > maxOutlineDepth {
		return nil, fmt.Errorf("opml: outline depth exceeds maximum of %d", maxOutlineDepth)
	}

	xmlURL := strings.TrimSpace(raw.XMLURL)
	htmlURL := strings.TrimSpace(raw.HTMLURL)
	text := strings.TrimSpace(raw.Text)
	title := strings.TrimSpace(raw.Title)
	typ := strings.TrimSpace(raw.Type)

	display := text
	if display == "" {
		display = title
	}

	// Feed: non-empty xmlUrl.
	if xmlURL != "" {
		if !isHTTPURL(xmlURL) {
			// Skip invalid feed URLs; do not fail the whole document.
			return nil, nil
		}
		if htmlURL != "" && !isHTTPURL(htmlURL) {
			htmlURL = ""
		}
		if typ == "" {
			typ = "rss"
		}
		return &Outline{
			Text:    display,
			Title:   title,
			Type:    typ,
			XMLURL:  xmlURL,
			HTMLURL: htmlURL,
		}, nil
	}

	// Folder or empty: convert children; ignore leaf without xmlUrl.
	if len(raw.Children) == 0 {
		return nil, nil
	}

	children := make([]Outline, 0, len(raw.Children))
	for i := range raw.Children {
		child, err := convertOutline(&raw.Children[i], depth+1)
		if err != nil {
			return nil, err
		}
		if child != nil {
			children = append(children, *child)
		}
	}
	if len(children) == 0 {
		// Folder with only invalid/empty children → ignore.
		return nil, nil
	}
	if display == "" {
		display = "Folder"
	}
	return &Outline{
		Text:     display,
		Title:    title,
		Children: children,
	}, nil
}

func isHTTPURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return (scheme == "http" || scheme == "https") && u.Host != ""
}
