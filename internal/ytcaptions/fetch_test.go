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
	text := parseSrv3(raw)
	if !strings.Contains(text, "Pi is a great AI agent") {
		t.Fatalf("text = %q", text)
	}
	if !strings.Contains(text, "all kinds of stuff") {
		t.Fatalf("text missing second line: %q", text)
	}
	html := FormatHTML(Result{Language: "en", Text: text})
	if !HasCaptionsSection(html) {
		t.Fatalf("missing section: %s", html)
	}
	if strings.Contains(html, "<script>") {
		t.Fatal("unexpected script")
	}
	joined := AppendHTML(`<div class="yt-embed">x</div>`, Result{Language: "en", Text: text})
	if !strings.Contains(joined, "yt-embed") || !HasCaptionsSection(joined) {
		t.Fatalf("append failed: %s", joined)
	}
	// Idempotent append
	again := AppendHTML(joined, Result{Language: "en", Text: "other"})
	if strings.Count(again, `data-yt-captions="1"`) != 1 {
		t.Fatalf("double captions: %s", again)
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
