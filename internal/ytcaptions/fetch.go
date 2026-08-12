// Package ytcaptions fetches YouTube caption/transcript text for a video id
// (manual tracks preferred, then auto-generated) without a Data API key.
package ytcaptions

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	defaultTimeout = 18 * time.Second
	maxBody        = 3 << 20
)

// Cue is one timed caption line (start offset in the video).
type Cue struct {
	// StartMs is the cue start time in milliseconds from video start.
	StartMs int
	// Text is the caption line (plain text, unescaped).
	Text string
}

// Result is caption data for display under the video embed.
type Result struct {
	Language string // BCP-47-ish code from the track (e.g. en, zh-Hans)
	Kind     string // "asr" for auto-generated, else ""
	// Text is plain joined transcript (legacy / search / fallback).
	Text string
	// Cues are timed lines when available; FormatHTML prefers these for a timeline UI.
	Cues []Cue
}

// fillTextFromCues sets Text from Cues when Text is empty.
func (r *Result) fillTextFromCues() {
	if strings.TrimSpace(r.Text) != "" || len(r.Cues) == 0 {
		return
	}
	var b strings.Builder
	for _, c := range r.Cues {
		t := strings.TrimSpace(c.Text)
		if t == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t)
	}
	r.Text = b.String()
}

// Options controls Fetch.
type Options struct {
	// Prefer languages in order (e.g. zh-Hans, zh, en). Empty uses defaults.
	Prefer []string
	Timeout time.Duration
	HTTP    *http.Client
}

var defaultPrefer = []string{
	"zh-Hans", "zh-CN", "zh", "zh-Hant", "zh-TW",
	"en", "en-US", "en-GB",
	"ja", "ko", "de", "fr", "es",
}

// Fetch downloads captions for videoID.
// Order (aligned with baoyu-youtube-transcript style tools):
//  1. InnerTube: watch-page session + multi-client player + timedtext
//  2. github.com/kkdai/youtube/v2
//  3. Optional local yt-dlp (-J) if installed
func Fetch(ctx context.Context, videoID string, opts Options) (Result, error) {
	videoID = strings.TrimSpace(videoID)
	if videoID == "" || !videoIDOK(videoID) {
		return Result{}, fmt.Errorf("ytcaptions: invalid video id")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	prefer := opts.Prefer
	if len(prefer) == 0 {
		prefer = defaultPrefer
	}

	httpClient := opts.HTTP
	if httpClient == nil {
		// Plain net/http; surf fingerprinting is flaky against YouTube on some PCs.
		httpClient = &http.Client{Timeout: timeout}
	}

	var lastErr error

	// --- 1) InnerTube with watch session + ANDROID/WEB/iOS rotation ---
	if res, err := fetchViaInnertubeSession(cctx, videoID, prefer, httpClient); err == nil {
		normalizeResult(&res)
		if res.Text != "" || len(res.Cues) > 0 {
			return res, nil
		}
		lastErr = fmt.Errorf("ytcaptions: empty innertube result")
	} else {
		lastErr = err
	}

	// --- 2) kkdai pure-Go client ---
	if res, err := fetchViaKKDAI(cctx, videoID, prefer, httpClient); err == nil {
		normalizeResult(&res)
		if res.Text != "" || len(res.Cues) > 0 {
			return res, nil
		}
		lastErr = fmt.Errorf("ytcaptions: empty kkdai result")
	} else {
		lastErr = err
	}

	// --- 3) optional yt-dlp ---
	if res, err := fetchViaYtDlp(cctx, videoID, prefer, httpClient); err == nil {
		normalizeResult(&res)
		if res.Text != "" || len(res.Cues) > 0 {
			return res, nil
		}
		lastErr = fmt.Errorf("ytcaptions: empty yt-dlp result")
	} else if !strings.Contains(err.Error(), "yt-dlp not found") {
		lastErr = err
	}

	if lastErr != nil {
		return Result{}, lastErr
	}
	return Result{}, fmt.Errorf("ytcaptions: all caption backends failed")
}

func normalizeResult(res *Result) {
	if res == nil {
		return
	}
	for i := range res.Cues {
		res.Cues[i].Text = cleanCaptionText(res.Cues[i].Text)
	}
	// Drop empty cues
	out := res.Cues[:0]
	for _, c := range res.Cues {
		if c.Text != "" {
			out = append(out, c)
		}
	}
	res.Cues = out
	if strings.TrimSpace(res.Text) != "" {
		res.Text = cleanCaptionText(res.Text)
	}
	res.fillTextFromCues()
}

type captionTrack struct {
	BaseURL      string
	LanguageCode string
	Kind         string // "asr" or empty
	Name         string
}

func playability(v any) (status, reason string) {
	m, ok := v.(map[string]any)
	if !ok {
		return "", ""
	}
	// Direct or nested playabilityStatus
	var walk func(any)
	walk = func(x any) {
		if status != "" {
			return
		}
		switch t := x.(type) {
		case map[string]any:
			if ps, ok := t["playabilityStatus"].(map[string]any); ok {
				status, _ = ps["status"].(string)
				reason, _ = ps["reason"].(string)
				return
			}
			for _, c := range t {
				walk(c)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	walk(m)
	return status, reason
}

func extractTracks(v any) []captionTrack {
	switch t := v.(type) {
	case map[string]any:
		if arr, ok := t["captionTracks"].([]any); ok {
			out := make([]captionTrack, 0, len(arr))
			for _, el := range arr {
				m, ok := el.(map[string]any)
				if !ok {
					continue
				}
				base, _ := m["baseUrl"].(string)
				if base == "" {
					continue
				}
				code, _ := m["languageCode"].(string)
				kind, _ := m["kind"].(string)
				name := ""
				if n, ok := m["name"].(map[string]any); ok {
					name, _ = n["simpleText"].(string)
				}
				out = append(out, captionTrack{
					BaseURL:      base,
					LanguageCode: code,
					Kind:         kind,
					Name:         name,
				})
			}
			return out
		}
		for _, c := range t {
			if out := extractTracks(c); len(out) > 0 {
				return out
			}
		}
	case []any:
		for _, c := range t {
			if out := extractTracks(c); len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func pickTrack(tracks []captionTrack, prefer []string) captionTrack {
	// Prefer non-ASR matching preferred languages, then any preferred (incl. ASR), then first non-ASR, then first.
	score := func(t captionTrack) int {
		s := 0
		if t.Kind != "asr" {
			s += 1000
		}
		code := strings.ToLower(t.LanguageCode)
		for i, p := range prefer {
			pl := strings.ToLower(p)
			if code == pl || strings.HasPrefix(code, pl+"-") || strings.HasPrefix(pl, code+"-") {
				s += 500 - i
				break
			}
		}
		return s
	}
	best := tracks[0]
	bestS := score(best)
	for _, t := range tracks[1:] {
		if sc := score(t); sc > bestS {
			best, bestS = t, sc
		}
	}
	return best
}

func downloadTrack(ctx context.Context, client *http.Client, baseURL string) ([]Cue, error) {
	return downloadTrackWithUA(ctx, client, baseURL, webUA)
}

type srv3TimedText struct {
	Body struct {
		P []struct {
			T    string `xml:"t,attr"` // start ms
			D    string `xml:"d,attr"` // duration ms (unused)
			Text string `xml:",chardata"`
			// Nested <s> segments in some variants
			S []struct {
				Text string `xml:",chardata"`
			} `xml:"s"`
		} `xml:"p"`
	} `xml:"body"`
}

func parseSrv3Cues(raw []byte) []Cue {
	var doc srv3TimedText
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return parsePTagCues(string(raw))
	}
	var cues []Cue
	for _, p := range doc.Body.P {
		var b strings.Builder
		b.WriteString(p.Text)
		for _, s := range p.S {
			b.WriteString(s.Text)
		}
		line := strings.TrimSpace(html.UnescapeString(b.String()))
		if line == "" {
			continue
		}
		start := 0
		if p.T != "" {
			if n, err := parseIntLoose(p.T); err == nil {
				start = n
			}
		}
		cues = append(cues, Cue{StartMs: start, Text: line})
	}
	return cues
}

// parseSrv3 returns plain joined text (tests / legacy).
func parseSrv3(raw []byte) string {
	cues := parseSrv3Cues(raw)
	var parts []string
	for _, c := range cues {
		parts = append(parts, c.Text)
	}
	return strings.Join(parts, " ")
}

var pTagRe = regexp.MustCompile(`(?is)<p\b([^>]*)>(.*?)</p>`)
var pAttrTRe = regexp.MustCompile(`(?i)\bt\s*=\s*["']?(\d+)`)

func parsePTagCues(raw string) []Cue {
	matches := pTagRe.FindAllStringSubmatch(raw, -1)
	var cues []Cue
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		line := strings.TrimSpace(html.UnescapeString(stripTags(m[2])))
		if line == "" {
			continue
		}
		start := 0
		if am := pAttrTRe.FindStringSubmatch(m[1]); len(am) == 2 {
			if n, err := parseIntLoose(am[1]); err == nil {
				start = n
			}
		}
		cues = append(cues, Cue{StartMs: start, Text: line})
	}
	return cues
}

func parseJSON3Cues(raw []byte) []Cue {
	var payload struct {
		Events []struct {
			TStartMs int `json:"tStartMs"`
			Segs     []struct {
				Utf8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	var cues []Cue
	for _, e := range payload.Events {
		var b strings.Builder
		for _, s := range e.Segs {
			b.WriteString(s.Utf8)
		}
		line := strings.TrimSpace(html.UnescapeString(b.String()))
		if line == "" || line == "\n" {
			continue
		}
		cues = append(cues, Cue{StartMs: e.TStartMs, Text: line})
	}
	return cues
}

func parseIntLoose(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	digits := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		digits++
	}
	if digits == 0 {
		return 0, fmt.Errorf("no digits")
	}
	return n, nil
}

var spaceRe = regexp.MustCompile(`\s+`)

func cleanCaptionText(s string) string {
	s = html.UnescapeString(s)
	s = stripTags(s)
	s = spaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func videoIDOK(id string) bool {
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

// FormatHTML wraps transcript as a section for article body HTML.
// When Cues are present, each line is shown with a timeline timestamp (m:ss).
func FormatHTML(res Result) string {
	res.fillTextFromCues()
	if strings.TrimSpace(res.Text) == "" && len(res.Cues) == 0 {
		return ""
	}
	label := "字幕 / Captions"
	if res.Language != "" {
		label = "字幕 / Captions (" + res.Language + ")"
		if res.Kind == "asr" {
			label += ", auto"
		}
	} else if res.Kind == "asr" {
		label = "字幕 / Captions (auto)"
	}
	var b strings.Builder
	// id survives aggressive sanitizers better than data-* alone; keep both.
	// data-yt-captions-timed marks timeline layout so we can upgrade old plain blocks.
	timed := len(res.Cues) > 0
	b.WriteString(`<section id="lrss-yt-captions" class="yt-captions`)
	if timed {
		b.WriteString(` yt-captions-timed"`)
		b.WriteString(` data-yt-captions="1" data-yt-captions-timed="1"`)
	} else {
		b.WriteString(`" data-yt-captions="1"`)
	}
	b.WriteString(`>`)
	b.WriteString(`<h3 class="yt-captions-title">`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`</h3>`)
	if timed {
		b.WriteString(formatCueLines(res.Cues))
	} else {
		escaped := html.EscapeString(res.Text)
		b.WriteString(wrapCaptionParagraphs(escaped))
	}
	b.WriteString(`</section>`)
	return b.String()
}

// formatCueLines renders timed cues as a vertical timeline list.
func formatCueLines(cues []Cue) string {
	// Merge very short ASR fragments into readable rows (~5s or ~90 chars).
	merged := mergeCues(cues, 5000, 90)
	var b strings.Builder
	b.WriteString(`<div class="yt-caption-list">`)
	for _, c := range merged {
		b.WriteString(`<div class="yt-caption-line">`)
		b.WriteString(`<span class="yt-caption-time">`)
		b.WriteString(html.EscapeString(formatTimestamp(c.StartMs)))
		b.WriteString(`</span>`)
		b.WriteString(`<span class="yt-caption-text">`)
		b.WriteString(html.EscapeString(c.Text))
		b.WriteString(`</span>`)
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// mergeCues groups consecutive short cues for readable timeline rows.
// maxGapMs: start a new row when the next cue jumps more than this after the group start.
// maxChars: soft wrap length for a single row.
func mergeCues(cues []Cue, maxGapMs, maxChars int) []Cue {
	if len(cues) == 0 {
		return nil
	}
	if maxGapMs <= 0 {
		maxGapMs = 5000
	}
	if maxChars <= 0 {
		maxChars = 90
	}
	var out []Cue
	cur := Cue{StartMs: cues[0].StartMs, Text: strings.TrimSpace(cues[0].Text)}
	for i := 1; i < len(cues); i++ {
		n := cues[i]
		nt := strings.TrimSpace(n.Text)
		if nt == "" {
			continue
		}
		gap := n.StartMs - cur.StartMs
		nextLen := len(cur.Text) + 1 + len(nt)
		if cur.Text == "" {
			cur = Cue{StartMs: n.StartMs, Text: nt}
			continue
		}
		if gap > maxGapMs || nextLen > maxChars {
			if cur.Text != "" {
				out = append(out, cur)
			}
			cur = Cue{StartMs: n.StartMs, Text: nt}
			continue
		}
		cur.Text = cur.Text + " " + nt
	}
	if cur.Text != "" {
		out = append(out, cur)
	}
	return out
}

// formatTimestamp renders milliseconds as m:ss or h:mm:ss.
func formatTimestamp(ms int) string {
	if ms < 0 {
		ms = 0
	}
	sec := ms / 1000
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

func wrapCaptionParagraphs(escaped string) string {
	// escaped is one long line of text; chunk by ~480 chars on spaces.
	words := strings.Fields(escaped)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	var line strings.Builder
	for _, w := range words {
		if line.Len() > 0 && line.Len()+1+len(w) > 480 {
			b.WriteString("<p>")
			b.WriteString(line.String())
			b.WriteString("</p>")
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(w)
	}
	if line.Len() > 0 {
		b.WriteString("<p>")
		b.WriteString(line.String())
		b.WriteString("</p>")
	}
	return b.String()
}

// HasTimedCaptions reports a captions block that already has the timeline layout.
func HasTimedCaptions(contentHTML string) bool {
	if contentHTML == "" {
		return false
	}
	return strings.Contains(contentHTML, `data-yt-captions-timed="1"`) ||
		strings.Contains(contentHTML, `yt-caption-time`) ||
		strings.Contains(contentHTML, `yt-captions-timed`)
}

// HasCaptionsSection reports whether HTML already includes our captions block
// with a marker that survives HTML sanitization (prefer id=).
// Note: do not match "yt-captions-miss".
func HasCaptionsSection(contentHTML string) bool {
	if contentHTML == "" {
		return false
	}
	if strings.Contains(contentHTML, `id="lrss-yt-captions"`) ||
		strings.Contains(contentHTML, `data-yt-captions="1"`) {
		return true
	}
	// class="yt-captions" but not yt-captions-miss / yt-captions-title alone
	if strings.Contains(contentHTML, `class="yt-captions"`) ||
		strings.Contains(contentHTML, `class='yt-captions'`) {
		return true
	}
	return strings.Contains(contentHTML, `yt-captions-title`)
}

// StripLegacyCaptions removes an older captions heading/body that lost its
// markers after sanitization, so we can re-attach a proper section.
func StripLegacyCaptions(contentHTML string) string {
	if contentHTML == "" || HasCaptionsSection(contentHTML) {
		return contentHTML
	}
	// Look for bare "Captions …" / "字幕" headings left by older sanitizer runs.
	lower := strings.ToLower(contentHTML)
	idx := strings.Index(lower, "captions (")
	if idx < 0 {
		idx = strings.Index(contentHTML, "字幕")
	}
	if idx < 0 {
		return contentHTML
	}
	// Walk back to the opening <h2>/<h3>/<section if present.
	head := contentHTML[:idx]
	cut := strings.LastIndex(strings.ToLower(head), "<h3")
	if cut < 0 {
		cut = strings.LastIndex(strings.ToLower(head), "<h2")
	}
	if cut < 0 {
		cut = strings.LastIndex(strings.ToLower(head), "<section")
	}
	if cut < 0 {
		cut = idx
	}
	return strings.TrimSpace(contentHTML[:cut])
}

// AppendHTML adds a captions section if not already present.
func AppendHTML(contentHTML string, res Result) string {
	block := FormatHTML(res)
	if block == "" {
		return contentHTML
	}
	if HasCaptionsSection(contentHTML) {
		return contentHTML
	}
	if strings.TrimSpace(contentHTML) == "" {
		return block
	}
	return contentHTML + "\n" + block
}
