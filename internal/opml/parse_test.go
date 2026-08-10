package opml

import (
	"strings"
	"testing"
)

func TestParse_EmptyInput(t *testing.T) {
	cases := []struct {
		name string
		data []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"whitespace", []byte("   \n\t  ")},
		{"bom_only", []byte{0xEF, 0xBB, 0xBF}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.data)
			if err == nil {
				t.Fatal("expected error for empty input")
			}
		})
	}
}

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse([]byte(`not xml at all`))
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParse_NestedFolderAndFeeds(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head>
    <title>My Subscriptions</title>
  </head>
  <body>
    <outline text="News" title="News">
      <outline type="rss" text="Example" title="Example"
               xmlUrl="https://example.com/feed.xml"
               htmlUrl="https://example.com"/>
      <outline text="Tech">
        <outline type="rss" text="Go Blog" xmlUrl="https://go.dev/blog/feed.atom"
                 htmlUrl="https://go.dev/blog"/>
      </outline>
    </outline>
    <outline type="rss" text="Bare" xmlUrl="https://bare.example/rss"/>
  </body>
</opml>`)

	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Title != "My Subscriptions" {
		t.Errorf("title = %q, want My Subscriptions", doc.Title)
	}
	if len(doc.Outlines) != 2 {
		t.Fatalf("top outlines = %d, want 2", len(doc.Outlines))
	}

	news := doc.Outlines[0]
	if news.Text != "News" || news.XMLURL != "" {
		t.Errorf("News folder: text=%q xmlUrl=%q", news.Text, news.XMLURL)
	}
	if len(news.Children) != 2 {
		t.Fatalf("News children = %d, want 2", len(news.Children))
	}
	ex := news.Children[0]
	if ex.XMLURL != "https://example.com/feed.xml" || ex.HTMLURL != "https://example.com" {
		t.Errorf("Example feed: %+v", ex)
	}
	if ex.Type != "rss" || ex.Text != "Example" {
		t.Errorf("Example type/text: %q / %q", ex.Type, ex.Text)
	}
	tech := news.Children[1]
	if tech.Text != "Tech" || len(tech.Children) != 1 {
		t.Fatalf("Tech folder: text=%q children=%d", tech.Text, len(tech.Children))
	}
	if tech.Children[0].XMLURL != "https://go.dev/blog/feed.atom" {
		t.Errorf("nested feed url = %q", tech.Children[0].XMLURL)
	}

	bare := doc.Outlines[1]
	if bare.XMLURL != "https://bare.example/rss" || bare.Text != "Bare" {
		t.Errorf("bare feed: %+v", bare)
	}
}

func TestParse_FlatFeedsOnly(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<opml version="1.0">
  <head><title>Flat</title></head>
  <body>
    <outline type="rss" text="A" xmlUrl="https://a.example/feed"/>
    <outline type="rss" text="B" title="Bee" xmlUrl="https://b.example/rss.xml"
             htmlUrl="https://b.example"/>
  </body>
</opml>`)

	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Outlines) != 2 {
		t.Fatalf("outlines = %d, want 2", len(doc.Outlines))
	}
	if doc.Outlines[0].XMLURL != "https://a.example/feed" {
		t.Errorf("feed A url = %q", doc.Outlines[0].XMLURL)
	}
	if doc.Outlines[1].Title != "Bee" || doc.Outlines[1].HTMLURL != "https://b.example" {
		t.Errorf("feed B: %+v", doc.Outlines[1])
	}
}

func TestParse_MissingXMLURLFolderWithChildren(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<opml version="2.0">
  <body>
    <outline text="Group">
      <outline text="ChildFeed" xmlUrl="https://child.example/feed"/>
    </outline>
    <outline text="EmptyLeaf"/>
    <outline text="EmptyFolder">
      <outline text="AlsoEmpty"/>
    </outline>
  </body>
</opml>`)

	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// EmptyLeaf and EmptyFolder (only empty children) ignored.
	if len(doc.Outlines) != 1 {
		t.Fatalf("outlines = %d, want 1 (folder with feed)", len(doc.Outlines))
	}
	if doc.Outlines[0].Text != "Group" || len(doc.Outlines[0].Children) != 1 {
		t.Fatalf("group: %+v", doc.Outlines[0])
	}
	if doc.Outlines[0].Children[0].XMLURL != "https://child.example/feed" {
		t.Errorf("child feed: %+v", doc.Outlines[0].Children[0])
	}
}

func TestParse_AttributeCaseVariants(t *testing.T) {
	cases := []struct {
		name string
		attr string
		want string
	}{
		{"xmlUrl", `xmlUrl="https://ex.example/f"`, "https://ex.example/f"},
		{"XMLURL", `XMLURL="https://ex.example/f"`, "https://ex.example/f"},
		{"xmlurl", `xmlurl="https://ex.example/f"`, "https://ex.example/f"},
		{"XmlUrl", `XmlUrl="https://ex.example/f"`, "https://ex.example/f"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(`<?xml version="1.0"?>
<opml version="2.0"><body>
  <outline text="X" ` + tc.attr + ` HTMLURL="https://ex.example"/>
</body></opml>`)
			doc, err := Parse(raw)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(doc.Outlines) != 1 {
				t.Fatalf("outlines = %d, want 1", len(doc.Outlines))
			}
			if doc.Outlines[0].XMLURL != tc.want {
				t.Errorf("XMLURL = %q, want %q", doc.Outlines[0].XMLURL, tc.want)
			}
			if doc.Outlines[0].HTMLURL != "https://ex.example" {
				t.Errorf("HTMLURL = %q", doc.Outlines[0].HTMLURL)
			}
		})
	}
}

func TestParse_SkipInvalidFeedURLs(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<opml version="2.0"><body>
  <outline text="Bad" xmlUrl="ftp://files.example/feed"/>
  <outline text="NoHost" xmlUrl="https://"/>
  <outline text="Relative" xmlUrl="/local/feed"/>
  <outline text="Good" xmlUrl=" https://good.example/feed "/>
  <outline text="Spaced" xmlUrl="  "/>
</body></opml>`)

	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(doc.Outlines) != 1 {
		t.Fatalf("outlines = %d, want 1 (only good http feed)", len(doc.Outlines))
	}
	if doc.Outlines[0].XMLURL != "https://good.example/feed" {
		t.Errorf("trimmed url = %q", doc.Outlines[0].XMLURL)
	}
}

func TestParse_TextFallsBackToTitle(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<opml version="2.0"><body>
  <outline title="Only Title" xmlUrl="https://t.example/feed"/>
</body></opml>`)
	doc, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.Outlines[0].Text != "Only Title" {
		t.Errorf("Text = %q, want Only Title", doc.Outlines[0].Text)
	}
}

func TestParse_DepthCap(t *testing.T) {
	// Build nesting deeper than maxOutlineDepth.
	var b strings.Builder
	b.WriteString(`<?xml version="1.0"?><opml version="2.0"><body>`)
	for i := 0; i < maxOutlineDepth+2; i++ {
		b.WriteString(`<outline text="L">`)
	}
	b.WriteString(`<outline text="Deep" xmlUrl="https://deep.example/feed"/>`)
	for i := 0; i < maxOutlineDepth+2; i++ {
		b.WriteString(`</outline>`)
	}
	b.WriteString(`</body></opml>`)

	_, err := Parse([]byte(b.String()))
	if err == nil {
		t.Fatal("expected depth error")
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Errorf("error should mention depth: %v", err)
	}
}

func TestParse_BOM(t *testing.T) {
	body := `<?xml version="1.0"?><opml version="2.0"><body>
  <outline text="A" xmlUrl="https://a.example/f"/>
</body></opml>`
	data := append([]byte{0xEF, 0xBB, 0xBF}, body...)
	doc, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse with BOM: %v", err)
	}
	if len(doc.Outlines) != 1 {
		t.Fatalf("outlines = %d", len(doc.Outlines))
	}
}
