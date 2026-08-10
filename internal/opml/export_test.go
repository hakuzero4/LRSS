package opml

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
)

func TestExport_NilDocument(t *testing.T) {
	_, err := Export(nil)
	if err == nil {
		t.Fatal("expected error for nil document")
	}
}

func TestExport_Structure(t *testing.T) {
	doc := &Document{
		Title: "Test Subs",
		Outlines: []Outline{
			{
				Text:  "News",
				Title: "News",
				Children: []Outline{
					{
						Text:    "Example",
						Title:   "Example",
						Type:    "rss",
						XMLURL:  "https://example.com/feed.xml",
						HTMLURL: "https://example.com",
					},
				},
			},
			{
				Text:   "Bare",
				Type:   "rss",
				XMLURL: "https://bare.example/rss",
			},
		},
	}

	out, err := Export(doc)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	s := string(out)
	if !strings.HasPrefix(s, xml.Header) && !strings.Contains(s, `<?xml version="1.0"`) {
		t.Errorf("missing XML header: %s", s[:min(80, len(s))])
	}
	if !strings.Contains(s, `version="2.0"`) {
		t.Error("missing opml version 2.0")
	}
	if !strings.Contains(s, "<title>Test Subs</title>") {
		t.Error("missing head title")
	}
	if !strings.Contains(s, `xmlUrl="https://example.com/feed.xml"`) {
		t.Error("missing nested xmlUrl")
	}
	if !strings.Contains(s, `htmlUrl="https://example.com"`) {
		t.Error("missing htmlUrl")
	}
	if !strings.Contains(s, `type="rss"`) {
		t.Error("missing type=rss")
	}
	if !strings.Contains(s, `xmlUrl="https://bare.example/rss"`) {
		t.Error("missing bare feed")
	}
	// Folder should not carry xmlUrl on the News outline open tag alone —
	// ensure News appears as text without being a feed-only line.
	if !strings.Contains(s, `text="News"`) {
		t.Error("missing News folder text")
	}
}

func TestExport_DefaultTitle(t *testing.T) {
	doc := &Document{
		Outlines: []Outline{
			{Text: "A", XMLURL: "https://a.example/f"},
		},
	}
	out, err := Export(doc)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.Contains(string(out), "<title>"+defaultExportTitle+"</title>") {
		t.Errorf("expected default title, got:\n%s", out)
	}
}

func TestExport_EscapesXML(t *testing.T) {
	doc := &Document{
		Title: `A & B <C>`,
		Outlines: []Outline{
			{
				Text:   `Feed "quoted" & more`,
				XMLURL: "https://ex.example/feed?a=1&b=2",
			},
		},
	}
	out, err := Export(doc)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	s := string(out)
	// Must not contain raw unescaped ampersands in text content/attrs incorrectly.
	// encoding/xml escapes & to &amp; in attributes and char data.
	if strings.Contains(s, `A & B`) {
		t.Error("title not escaped")
	}
	if !strings.Contains(s, `&amp;`) {
		t.Error("expected &amp; escaping")
	}
	// Re-parse should succeed.
	if _, err := Parse(out); err != nil {
		t.Fatalf("re-parse exported XML: %v", err)
	}
}

func TestExport_FolderAndFeedDefaults(t *testing.T) {
	doc := &Document{
		Outlines: []Outline{
			{
				Text: "Folder",
				Children: []Outline{
					{XMLURL: "https://x.example/feed"}, // no text/type
				},
			},
		},
	}
	out, err := Export(doc)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, `type="rss"`) {
		t.Error("feed should default type=rss")
	}
	if !strings.Contains(s, `xmlUrl="https://x.example/feed"`) {
		t.Error("missing feed url")
	}
}

func TestRoundTrip_PreservesFeedURLs(t *testing.T) {
	cases := []struct {
		name string
		xml  string
		want []string // expected feed URL set
	}{
		{
			name: "nested",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>RT</title></head>
  <body>
    <outline text="News" title="News">
      <outline type="rss" text="Example" title="Example"
               xmlUrl="https://example.com/feed.xml"
               htmlUrl="https://example.com"/>
      <outline text="Tech">
        <outline type="rss" text="Go" xmlUrl="https://go.dev/blog/feed.atom"/>
      </outline>
    </outline>
    <outline type="rss" text="Bare" xmlUrl="https://bare.example/rss"/>
  </body>
</opml>`,
			want: []string{
				"https://example.com/feed.xml",
				"https://go.dev/blog/feed.atom",
				"https://bare.example/rss",
			},
		},
		{
			name: "flat",
			xml: `<?xml version="1.0"?>
<opml version="1.0"><body>
  <outline text="A" xmlUrl="https://a.example/1"/>
  <outline text="B" xmlUrl="https://b.example/2"/>
</body></opml>`,
			want: []string{"https://a.example/1", "https://b.example/2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, err := Parse([]byte(tc.xml))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			exported, err := Export(doc)
			if err != nil {
				t.Fatalf("Export: %v", err)
			}
			again, err := Parse(exported)
			if err != nil {
				t.Fatalf("Parse(Export): %v", err)
			}
			got := collectFeedURLs(again.Outlines)
			if len(got) != len(tc.want) {
				t.Fatalf("feed count = %d, want %d; got %v", len(got), len(tc.want), got)
			}
			wantSet := map[string]struct{}{}
			for _, u := range tc.want {
				wantSet[u] = struct{}{}
			}
			for _, u := range got {
				if _, ok := wantSet[u]; !ok {
					t.Errorf("unexpected feed URL %q", u)
				}
			}
			// Second export should keep the same URL set.
			exported2, err := Export(again)
			if err != nil {
				t.Fatalf("second Export: %v", err)
			}
			third, err := Parse(exported2)
			if err != nil {
				t.Fatalf("third Parse: %v", err)
			}
			got2 := collectFeedURLs(third.Outlines)
			if len(got2) != len(got) {
				t.Errorf("second round-trip lost feeds: %v vs %v", got2, got)
			}
			// Ensure export is valid-looking OPML.
			if !bytes.Contains(exported, []byte(`version="2.0"`)) {
				t.Error("exported version not 2.0")
			}
		})
	}
}

func collectFeedURLs(outlines []Outline) []string {
	var urls []string
	var walk func([]Outline)
	walk = func(list []Outline) {
		for i := range list {
			o := &list[i]
			if o.XMLURL != "" {
				urls = append(urls, o.XMLURL)
			}
			walk(o.Children)
		}
	}
	walk(outlines)
	return urls
}
