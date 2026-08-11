// Package ytcaptions fetches YouTube caption/transcript text for a video id
// (manual tracks preferred, then auto-generated) without a Data API key.
package ytcaptions

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"lrss/internal/httpx"
)

const (
	defaultTimeout = 18 * time.Second
	maxBody        = 3 << 20
	innertubeURL   = "https://www.youtube.com/youtubei/v1/player?prettyPrint=false"
	// Public Android client values used by many open-source transcript tools.
	clientName    = "ANDROID"
	clientVersion = "20.10.38"
	androidUA     = "com.google.android.youtube/20.10.38 (Linux; U; Android 14) gzip"
)

// Result is plain caption text for display under the video embed.
type Result struct {
	Language string // BCP-47-ish code from the track (e.g. en, zh-Hans)
	Kind     string // "asr" for auto-generated, else ""
	Text     string
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
// Order:
//  1. github.com/kkdai/youtube/v2 (pure Go, youtube-dl style client)
//  2. Direct innertube player + timedtext (local fallback)
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
		// Plain net/http first; surf can be flaky against YouTube on some PCs.
		httpClient = &http.Client{Timeout: timeout}
	}

	// --- 1) third-party Go library ---
	if res, err := fetchViaKKDAI(cctx, videoID, prefer, httpClient); err == nil && strings.TrimSpace(res.Text) != "" {
		res.Text = cleanCaptionText(res.Text)
		if res.Text != "" {
			return res, nil
		}
	}

	// --- 2) local innertube fallback ---
	return fetchViaInnertube(cctx, videoID, prefer, httpClient, timeout)
}

func fetchViaInnertube(ctx context.Context, videoID string, prefer []string, primary *http.Client, timeout time.Duration) (Result, error) {
	clients := []*http.Client{primary}
	if primary != nil {
		// optional surf fallback
		clients = append(clients, httpx.Std(httpx.Options{Timeout: timeout, UserAgent: androidUA}))
	}

	var (
		tracks []captionTrack
		client *http.Client
		err    error
	)
	for _, c := range clients {
		for attempt := 0; attempt < 2; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					return Result{}, ctx.Err()
				case <-time.After(350 * time.Millisecond):
				}
			}
			actx, cancel := context.WithTimeout(ctx, 8*time.Second)
			tracks, err = listTracks(actx, c, videoID)
			cancel()
			if err == nil && len(tracks) > 0 {
				client = c
				break
			}
		}
		if client != nil {
			break
		}
	}
	if client == nil || len(tracks) == 0 {
		if err != nil {
			return Result{}, err
		}
		return Result{}, fmt.Errorf("ytcaptions: no caption tracks")
	}
	track := pickTrack(tracks, prefer)
	if track.BaseURL == "" {
		return Result{}, fmt.Errorf("ytcaptions: empty caption url")
	}
	text, err := downloadTrack(ctx, client, track.BaseURL)
	if err != nil {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		text, err = downloadTrack(ctx, client, track.BaseURL)
		if err != nil {
			return Result{}, err
		}
	}
	text = cleanCaptionText(text)
	if text == "" {
		return Result{}, fmt.Errorf("ytcaptions: empty transcript")
	}
	return Result{
		Language: track.LanguageCode,
		Kind:     track.Kind,
		Text:     text,
	}, nil
}

type captionTrack struct {
	BaseURL      string
	LanguageCode string
	Kind         string // "asr" or empty
	Name         string
}

type innertubeClient struct {
	name, version, ua, clientNameHeader string
}

var innertubeClients = []innertubeClient{
	{clientName, clientVersion, androidUA, "3"},
	// WEB fallback when ANDROID is blocked or returns no tracks on some networks.
	{"WEB", "2.20250312.00.00", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36", "1"},
}

func listTracks(ctx context.Context, client *http.Client, videoID string) ([]captionTrack, error) {
	var lastErr error
	for _, ic := range innertubeClients {
		tracks, err := listTracksWithClient(ctx, client, videoID, ic)
		if err != nil {
			lastErr = err
			continue
		}
		if len(tracks) > 0 {
			return tracks, nil
		}
		lastErr = fmt.Errorf("ytcaptions: no caption tracks (%s)", ic.name)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("ytcaptions: no caption tracks")
}

func listTracksWithClient(ctx context.Context, client *http.Client, videoID string, ic innertubeClient) ([]captionTrack, error) {
	payload := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    ic.name,
				"clientVersion": ic.version,
				"hl":            "en",
				"gl":            "US",
			},
		},
		"videoId": videoID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, innertubeURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", ic.ua)
	req.Header.Set("X-YouTube-Client-Name", ic.clientNameHeader)
	req.Header.Set("X-YouTube-Client-Version", ic.version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ytcaptions: player: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ytcaptions: player http %d", resp.StatusCode)
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("ytcaptions: player json: %w", err)
	}
	if status, reason := playability(root); status == "LOGIN_REQUIRED" || status == "ERROR" {
		if reason == "" {
			reason = status
		}
		return nil, fmt.Errorf("ytcaptions: youtube blocked automated access (%s)", reason)
	}
	return extractTracks(root), nil
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

func downloadTrack(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	// Ensure srv3/xml or json3. Tracks often already include fmt=srv3.
	u := baseURL
	if !strings.Contains(u, "fmt=") {
		if strings.Contains(u, "?") {
			u += "&fmt=srv3"
		} else {
			u += "?fmt=srv3"
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", androidUA)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ytcaptions: timedtext: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ytcaptions: timedtext http %d", resp.StatusCode)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("ytcaptions: empty timedtext")
	}
	// Prefer XML srv3; fall back to json3 shape if needed.
	if bytes.Contains(raw, []byte("<timedtext")) || bytes.Contains(raw, []byte("<p ")) {
		return parseSrv3(raw), nil
	}
	if text := parseJSON3(raw); text != "" {
		return text, nil
	}
	// Last resort: strip tags.
	return cleanCaptionText(string(raw)), nil
}

type srv3TimedText struct {
	Body struct {
		P []struct {
			Text string `xml:",chardata"`
			// Nested <s> segments in some variants
			S []struct {
				Text string `xml:",chardata"`
			} `xml:"s"`
		} `xml:"p"`
	} `xml:"body"`
}

func parseSrv3(raw []byte) string {
	var doc srv3TimedText
	if err := xml.Unmarshal(raw, &doc); err != nil {
		// Fallback: crude <p>...</p> extraction.
		return parsePTags(string(raw))
	}
	var parts []string
	for _, p := range doc.Body.P {
		var b strings.Builder
		b.WriteString(p.Text)
		for _, s := range p.S {
			b.WriteString(s.Text)
		}
		line := strings.TrimSpace(html.UnescapeString(b.String()))
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, " ")
}

var pTagRe = regexp.MustCompile(`(?is)<p\b[^>]*>(.*?)</p>`)

func parsePTags(raw string) string {
	matches := pTagRe.FindAllStringSubmatch(raw, -1)
	var parts []string
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		line := strings.TrimSpace(html.UnescapeString(stripTags(m[1])))
		if line != "" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, " ")
}

func parseJSON3(raw []byte) string {
	var payload struct {
		Events []struct {
			Segs []struct {
				Utf8 string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	var parts []string
	for _, e := range payload.Events {
		var b strings.Builder
		for _, s := range e.Segs {
			b.WriteString(s.Utf8)
		}
		line := strings.TrimSpace(html.UnescapeString(b.String()))
		if line != "" && line != "\n" {
			parts = append(parts, line)
		}
	}
	return strings.Join(parts, " ")
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

// FormatHTML wraps transcript text as a section for article body HTML.
func FormatHTML(res Result) string {
	if strings.TrimSpace(res.Text) == "" {
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
	// Escape and split into paragraphs every ~4 sentences for readability.
	escaped := html.EscapeString(res.Text)
	paras := wrapCaptionParagraphs(escaped)
	var b strings.Builder
	// id survives aggressive sanitizers better than data-* alone; keep both.
	b.WriteString(`<section id="lrss-yt-captions" class="yt-captions" data-yt-captions="1">`)
	b.WriteString(`<h3 class="yt-captions-title">`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`</h3>`)
	b.WriteString(paras)
	b.WriteString(`</section>`)
	return b.String()
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
