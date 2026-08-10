package rss

import (
	"strings"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example Feed</title>
    <link>https://example.com/</link>
    <description>A sample feed</description>
    <item>
      <title>First Post</title>
      <link>https://example.com/posts/1</link>
      <guid isPermaLink="true">https://example.com/posts/1</guid>
      <pubDate>Mon, 02 Jan 2006 15:04:05 GMT</pubDate>
      <author>alice@example.com (Alice)</author>
      <description>&lt;p&gt;Hello &lt;b&gt;world&lt;/b&gt;&lt;/p&gt;</description>
      <enclosure url="https://example.com/img/cover.png" type="image/png" length="1234"/>
    </item>
    <item>
      <title>Relative Link</title>
      <link>/posts/2</link>
      <description>&lt;p&gt;No guid here&lt;/p&gt;&lt;img src="/media/pic.jpg"/&gt;</description>
      <pubDate>Tue, 03 Jan 2006 15:04:05 GMT</pubDate>
    </item>
    <item>
      <title>Hash Fallback</title>
      <description>Only title and date</description>
      <pubDate>Wed, 04 Jan 2006 15:04:05 GMT</pubDate>
    </item>
  </channel>
</rss>`

func TestNormalizeAuthor_GooglePersonMarkup(t *testing.T) {
	raw := `<name>Brenda Flynn</name><title>Partnerships Lead</title><department>Kaggle</department><company/>`
	got := normalizeAuthor(raw)
	want := "Brenda Flynn · Partnerships Lead · Kaggle"
	if got != want {
		t.Fatalf("normalizeAuthor = %q want %q", got, want)
	}
	if normalizeAuthor("Alice") != "Alice" {
		t.Fatalf("plain author changed")
	}
	if normalizeAuthor("") != "" {
		t.Fatalf("empty")
	}
}

func TestToParsedFeed_RSS20(t *testing.T) {
	parser := gofeed.NewParser()
	feed, err := parser.ParseString(sampleRSS)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	const feedURL = "https://example.com/feed.xml"
	got := ToParsedFeed(feed, feedURL)

	if got.Title != "Example Feed" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.SiteURL != "https://example.com/" {
		t.Errorf("SiteURL = %q", got.SiteURL)
	}
	if got.FeedURL != feedURL {
		t.Errorf("FeedURL = %q", got.FeedURL)
	}
	if len(got.Items) != 3 {
		t.Fatalf("Items len = %d, want 3", len(got.Items))
	}

	i0 := got.Items[0]
	if i0.GUID != "https://example.com/posts/1" {
		t.Errorf("item0 GUID = %q", i0.GUID)
	}
	if i0.URL != "https://example.com/posts/1" {
		t.Errorf("item0 URL = %q", i0.URL)
	}
	if i0.Title != "First Post" {
		t.Errorf("item0 Title = %q", i0.Title)
	}
	if i0.ImageURL != "https://example.com/img/cover.png" {
		t.Errorf("item0 ImageURL = %q", i0.ImageURL)
	}
	if i0.PublishedAt == nil {
		t.Error("item0 PublishedAt is nil")
	} else {
		want := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
		if !i0.PublishedAt.Equal(want) {
			t.Errorf("item0 PublishedAt = %v, want %v", *i0.PublishedAt, want)
		}
	}
	if !strings.Contains(i0.ContentText, "Hello") || !strings.Contains(i0.ContentText, "world") {
		t.Errorf("item0 ContentText = %q", i0.ContentText)
	}
	if strings.Contains(i0.ContentText, "<b>") {
		t.Errorf("item0 ContentText still has tags: %q", i0.ContentText)
	}

	i1 := got.Items[1]
	if i1.URL != "https://example.com/posts/2" {
		t.Errorf("item1 URL = %q, want absolute", i1.URL)
	}
	if i1.GUID != "https://example.com/posts/2" {
		t.Errorf("item1 GUID = %q", i1.GUID)
	}
	if i1.ImageURL != "https://example.com/media/pic.jpg" {
		t.Errorf("item1 ImageURL = %q", i1.ImageURL)
	}

	i2 := got.Items[2]
	if i2.GUID == "" {
		t.Error("item2 GUID empty, expected hash")
	}
	if len(i2.GUID) != 64 {
		t.Errorf("item2 GUID = %q (len %d), want sha256 hex", i2.GUID, len(i2.GUID))
	}
	again := ToParsedFeed(feed, feedURL)
	if again.Items[2].GUID != i2.GUID {
		t.Errorf("hash not stable: %q vs %q", i2.GUID, again.Items[2].GUID)
	}
}

func TestToParsedFeed_Nil(t *testing.T) {
	got := ToParsedFeed(nil, "https://x.com/feed")
	if got.FeedURL != "https://x.com/feed" {
		t.Fatalf("FeedURL = %q", got.FeedURL)
	}
	if len(got.Items) != 0 {
		t.Fatalf("Items = %v", got.Items)
	}
}

func TestResolveURL(t *testing.T) {
	cases := []struct {
		base, ref, want string
	}{
		{"https://ex.com/a/", "/b", "https://ex.com/b"},
		{"https://ex.com/a/", "c", "https://ex.com/a/c"},
		{"https://ex.com/", "https://other.com/x", "https://other.com/x"},
		{"", "https://abs.com", "https://abs.com"},
		{"https://ex.com", "", ""},
	}
	for _, tc := range cases {
		got := resolveURL(tc.base, tc.ref)
		if got != tc.want {
			t.Errorf("resolveURL(%q, %q) = %q, want %q", tc.base, tc.ref, got, tc.want)
		}
	}
}

func TestFirstImgSrc(t *testing.T) {
	html := `<p>hi</p><img class="x" src="https://cdn.ex/a.png" alt="a"/>`
	if got := firstImgSrc(html); got != "https://cdn.ex/a.png" {
		t.Fatalf("got %q", got)
	}
	if got := firstImgSrc("<p>no image</p>"); got != "" {
		t.Fatalf("got %q", got)
	}
}
