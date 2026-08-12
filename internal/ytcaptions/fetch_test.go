package ytcaptions

import (
	"strings"
	"testing"
)

func TestParseSrv3AndFormat(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="utf-8" ?><timedtext format="3">
<body>
<p t="160" d="2960">Pi is a great AI agent,</p>
<p t="3120" d="1760">you can do all kinds of stuff with it.</p>
</body></timedtext>`)
	cues := parseSrv3Cues(raw)
	if len(cues) != 2 {
		t.Fatalf("cues = %+v", cues)
	}
	if cues[0].StartMs != 160 || !strings.Contains(cues[0].Text, "Pi is a great AI agent") {
		t.Fatalf("cue0 = %+v", cues[0])
	}
	if cues[1].StartMs != 3120 {
		t.Fatalf("cue1 start = %d", cues[1].StartMs)
	}
	text := parseSrv3(raw)
	if !strings.Contains(text, "all kinds of stuff") {
		t.Fatalf("text missing second line: %q", text)
	}
	html := FormatHTML(Result{Language: "en", Cues: cues})
	if !HasCaptionsSection(html) {
		t.Fatalf("missing section: %s", html)
	}
	if !HasTimedCaptions(html) {
		t.Fatalf("expected timed captions: %s", html)
	}
	if !strings.Contains(html, "yt-caption-time") || !strings.Contains(html, "0:00") {
		// 160ms → 0:00 (nearby cues may merge into one row)
		t.Fatalf("missing time labels: %s", html)
	}
	if !strings.Contains(html, "Pi is a great AI agent") {
		t.Fatalf("missing caption text: %s", html)
	}
	// Wider gap → separate timeline rows
	wide := FormatHTML(Result{Language: "en", Cues: []Cue{
		{StartMs: 0, Text: "First"},
		{StartMs: 12000, Text: "Second"},
	}})
	if !strings.Contains(wide, "0:00") || !strings.Contains(wide, "0:12") {
		t.Fatalf("expected two timestamps: %s", wide)
	}
	if strings.Contains(html, "<script>") {
		t.Fatal("unexpected script")
	}
	joined := AppendHTML(`<div class="yt-embed">x</div>`, Result{Language: "en", Cues: cues})
	if !strings.Contains(joined, "yt-embed") || !HasCaptionsSection(joined) {
		t.Fatalf("append failed: %s", joined)
	}
	// Idempotent append
	again := AppendHTML(joined, Result{Language: "en", Text: "other"})
	if strings.Count(again, `data-yt-captions="1"`) != 1 {
		t.Fatalf("double captions: %s", again)
	}
}

func TestFormatTimestamp(t *testing.T) {
	if got := formatTimestamp(0); got != "0:00" {
		t.Fatalf("0 → %q", got)
	}
	if got := formatTimestamp(65000); got != "1:05" {
		t.Fatalf("65s → %q", got)
	}
	if got := formatTimestamp(3661000); got != "1:01:01" {
		t.Fatalf("1h1m1s → %q", got)
	}
}

func TestTracksFromYtDlp(t *testing.T) {
	info := ytDlpInfo{
		Subtitles: map[string][]ytDlpTrackEntry{
			"en": {{URL: "https://example/en.srv3", Ext: "srv3"}},
		},
		AutomaticCaptions: map[string][]ytDlpTrackEntry{
			"zh-Hans": {{URL: "https://example/zh.json3", Ext: "json3"}},
		},
	}
	tracks := tracksFromYtDlp(info)
	if len(tracks) != 2 {
		t.Fatalf("tracks = %+v", tracks)
	}
	got := pickTrack(tracks, []string{"en"})
	if got.Kind != "" || !strings.Contains(got.BaseURL, "en.srv3") {
		t.Fatalf("prefer manual en: %+v", got)
	}
}

func TestExtractSessionRegex(t *testing.T) {
	html := `ytcfg.set({"INNERTUBE_API_KEY":"AIzaSyDemoKey123","INNERTUBE_CLIENT_VERSION":"2.20260320.08.00","VISITOR_DATA":"CgtVisitor123"});`
	if m := reAPIKey.FindStringSubmatch(html); len(m) != 2 || m[1] != "AIzaSyDemoKey123" {
		t.Fatalf("api key: %v", m)
	}
	if m := reClientVer.FindStringSubmatch(html); len(m) != 2 || !strings.HasPrefix(m[1], "2.2026") {
		t.Fatalf("client ver: %v", m)
	}
	if m := reVisitor.FindStringSubmatch(html); len(m) != 2 || m[1] != "CgtVisitor123" {
		t.Fatalf("visitor: %v", m)
	}
}

func TestMergeCues(t *testing.T) {
	in := []Cue{
		{StartMs: 0, Text: "Hello"},
		{StartMs: 400, Text: "world"},
		{StartMs: 8000, Text: "Next"},
	}
	out := mergeCues(in, 5000, 90)
	if len(out) != 2 {
		t.Fatalf("merged = %+v", out)
	}
	if out[0].Text != "Hello world" || out[0].StartMs != 0 {
		t.Fatalf("first = %+v", out[0])
	}
	if out[1].Text != "Next" || out[1].StartMs != 8000 {
		t.Fatalf("second = %+v", out[1])
	}
}

func TestPickTrackPrefersManualThenLang(t *testing.T) {
	tracks := []captionTrack{
		{BaseURL: "a", LanguageCode: "de", Kind: "asr"},
		{BaseURL: "b", LanguageCode: "en", Kind: "asr"},
		{BaseURL: "c", LanguageCode: "en", Kind: ""},
		{BaseURL: "d", LanguageCode: "zh-Hans", Kind: "asr"},
	}
	got := pickTrack(tracks, []string{"en", "zh-Hans"})
	if got.BaseURL != "c" {
		t.Fatalf("want manual en, got %+v", got)
	}
	got = pickTrack(tracks, []string{"zh-Hans", "en"})
	// manual en still scores higher than asr zh due to +1000
	if got.BaseURL != "c" {
		t.Fatalf("manual still preferred: %+v", got)
	}
}

func TestVideoIDOK(t *testing.T) {
	if !videoIDOK("o8-EgQhqdU0") {
		t.Fatal("valid id rejected")
	}
	if videoIDOK("bad id") {
		t.Fatal("invalid accepted")
	}
}

func TestStripLegacyCaptions(t *testing.T) {
	in := `<div class="yt-embed">embed</div><h3>Captions (en), auto</h3><p>hello transcript</p>`
	out := StripLegacyCaptions(in)
	if strings.Contains(out, "Captions") || strings.Contains(out, "transcript") {
		t.Fatalf("legacy not stripped: %q", out)
	}
	if !strings.Contains(out, "yt-embed") {
		t.Fatalf("embed lost: %q", out)
	}
	good := FormatHTML(Result{Language: "en", Text: "hi"})
	if StripLegacyCaptions(good) != good {
		t.Fatal("should keep strong markers")
	}
}
