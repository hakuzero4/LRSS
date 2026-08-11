package rss

import (
	"strings"
	"testing"

	"github.com/mmcdole/gofeed"
)

const sampleYouTubeAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015"
      xmlns:media="http://search.yahoo.com/mrss/"
      xmlns="http://www.w3.org/2005/Atom">
  <title>Example Channel</title>
  <link rel="alternate" href="https://www.youtube.com/channel/UCTEST"/>
  <entry>
    <id>yt:video:AbCdEf12345</id>
    <yt:videoId>AbCdEf12345</yt:videoId>
    <yt:channelId>UCTEST</yt:channelId>
    <title>Sample Video</title>
    <link rel="alternate" href="https://www.youtube.com/watch?v=AbCdEf12345"/>
    <published>2024-01-02T15:04:05+00:00</published>
    <media:group>
      <media:title>Sample Video</media:title>
      <media:content url="https://www.youtube.com/v/AbCdEf12345?version=3" type="application/x-shockwave-flash" width="640" height="390"/>
      <media:thumbnail url="https://i.ytimg.com/vi/AbCdEf12345/hqdefault.jpg" width="480" height="360"/>
      <media:description>Line one.

Line three with &lt;script&gt;no&lt;/script&gt;.</media:description>
    </media:group>
  </entry>
</feed>`

func TestToParsedFeed_YouTubeAtom_MediaGroup(t *testing.T) {
	fp := gofeed.NewParser()
	feed, err := fp.ParseString(sampleYouTubeAtom)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	got := ToParsedFeed(feed, "https://www.youtube.com/feeds/videos.xml?channel_id=UCTEST")
	if len(got.Items) != 1 {
		t.Fatalf("items = %d", len(got.Items))
	}
	it := got.Items[0]
	if it.URL != "https://www.youtube.com/watch?v=AbCdEf12345" {
		t.Errorf("url = %q", it.URL)
	}
	if !strings.Contains(it.ContentHTML, "youtube-nocookie.com/embed/AbCdEf12345") {
		t.Fatalf("missing embed in ContentHTML: %q", it.ContentHTML)
	}
	if !strings.Contains(it.ContentHTML, "Line one") {
		t.Fatalf("missing description: %q", it.ContentHTML)
	}
	if strings.Contains(it.ContentHTML, "<script>") {
		t.Fatalf("description not escaped: %q", it.ContentHTML)
	}
	if it.ImageURL != "https://i.ytimg.com/vi/AbCdEf12345/hqdefault.jpg" {
		t.Errorf("image = %q", it.ImageURL)
	}
	if it.Summary == "" {
		t.Error("expected summary from description")
	}
}

func TestYouTubeIDFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=o8-EgQhqdU0":     "o8-EgQhqdU0",
		"https://youtu.be/o8-EgQhqdU0":                    "o8-EgQhqdU0",
		"https://www.youtube.com/embed/o8-EgQhqdU0":      "o8-EgQhqdU0",
		"https://www.youtube.com/shorts/o8-EgQhqdU0":     "o8-EgQhqdU0",
		"https://example.com/x":                          "",
	}
	for in, want := range cases {
		if got := youtubeIDFromURL(in); got != want {
			t.Errorf("youtubeIDFromURL(%q) = %q want %q", in, got, want)
		}
	}
}
