package opml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

const defaultExportTitle = "LRSS Subscriptions"

// Export serializes doc as OPML 2.0 UTF-8 XML.
func Export(doc *Document) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("opml: nil document")
	}

	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = defaultExportTitle
	}

	out := exportOPML{
		Version: "2.0",
		Head: exportHead{
			Title:       title,
			DateCreated: time.Now().UTC().Format(time.RFC1123Z),
		},
		Body: exportBody{
			Outlines: make([]exportOutline, 0, len(doc.Outlines)),
		},
	}
	for i := range doc.Outlines {
		out.Body.Outlines = append(out.Body.Outlines, toExportOutline(&doc.Outlines[i]))
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(&out); err != nil {
		return nil, fmt.Errorf("opml: export: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("opml: export: %w", err)
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

type exportOPML struct {
	XMLName xml.Name    `xml:"opml"`
	Version string      `xml:"version,attr"`
	Head    exportHead  `xml:"head"`
	Body    exportBody  `xml:"body"`
}

type exportHead struct {
	Title       string `xml:"title"`
	DateCreated string `xml:"dateCreated,omitempty"`
}

type exportBody struct {
	Outlines []exportOutline `xml:"outline"`
}

type exportOutline struct {
	Text     string          `xml:"text,attr"`
	Title    string          `xml:"title,attr,omitempty"`
	Type     string          `xml:"type,attr,omitempty"`
	XMLURL   string          `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string          `xml:"htmlUrl,attr,omitempty"`
	Children []exportOutline `xml:"outline,omitempty"`
}

func toExportOutline(o *Outline) exportOutline {
	text := strings.TrimSpace(o.Text)
	title := strings.TrimSpace(o.Title)
	if text == "" {
		text = title
	}
	if text == "" && o.XMLURL != "" {
		text = o.XMLURL
	}
	if text == "" {
		text = "Untitled"
	}

	eo := exportOutline{
		Text:    text,
		Title:   title,
		Type:    strings.TrimSpace(o.Type),
		XMLURL:  strings.TrimSpace(o.XMLURL),
		HTMLURL: strings.TrimSpace(o.HTMLURL),
	}

	// Feed outline: ensure type=rss when xmlUrl is set.
	if eo.XMLURL != "" {
		if eo.Type == "" {
			eo.Type = "rss"
		}
		if eo.Title == "" {
			eo.Title = text
		}
		return eo
	}

	// Folder: nest children; no type/xmlUrl/htmlUrl.
	eo.Type = ""
	eo.XMLURL = ""
	eo.HTMLURL = ""
	if eo.Title == "" {
		eo.Title = text
	}
	if len(o.Children) > 0 {
		eo.Children = make([]exportOutline, 0, len(o.Children))
		for i := range o.Children {
			eo.Children = append(eo.Children, toExportOutline(&o.Children[i]))
		}
	}
	return eo
}
