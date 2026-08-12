package ytcaptions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const (
	watchURLBase     = "https://www.youtube.com/watch?v="
	innertubePlayer  = "https://www.youtube.com/youtubei/v1/player"
	webUA            = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
	defaultWebClient = "2.20260320.08.00"
	androidUA        = "com.google.android.youtube/20.10.38 (Linux; U; Android 14; en_US; Pixel 8 Pro Build/AP1A.240405.002)"
	iosUA            = "com.google.ios.youtube/20.10.4 (iPhone16,2; U; CPU iOS 18_3 like Mac OS X; en_US)"
)

// innertubeSession is extracted from the public watch HTML (no Google Cloud API key).
type innertubeSession struct {
	APIKey          string
	WebClientVersion string
	VisitorData     string
}

type innertubeClient struct {
	id               string
	name             string // ANDROID | WEB | IOS
	version          string // empty → use session web version
	ua               string
	clientNameHeader string // X-YouTube-Client-Name
	extra            map[string]any
}

// Clients tried in order (aligned with baoyu-youtube-transcript / open-source tools).
var innertubeClients = []innertubeClient{
	{
		id: "android", name: "ANDROID", version: "20.10.38", ua: androidUA, clientNameHeader: "3",
		extra: map[string]any{
			"clientFormFactor":  "SMALL_FORM_FACTOR",
			"androidSdkVersion": 34,
			"osName":            "Android",
			"osVersion":         "14",
			"platform":          "MOBILE",
		},
	},
	{
		id: "web", name: "WEB", ua: webUA, clientNameHeader: "1",
	},
	{
		id: "ios", name: "IOS", version: "20.10.4", ua: iosUA, clientNameHeader: "5",
		extra: map[string]any{
			"deviceMake":  "Apple",
			"deviceModel": "iPhone16,2",
			"osName":      "iPhone",
			"osVersion":   "18.3.0",
			"platform":    "MOBILE",
		},
	},
}

var (
	reAPIKey    = regexp.MustCompile(`"INNERTUBE_API_KEY"\s*:\s*"([a-zA-Z0-9_-]+)"`)
	reClientVer = regexp.MustCompile(`"INNERTUBE_CLIENT_VERSION"\s*:\s*"([^"]+)"`)
	reClientVer2 = regexp.MustCompile(`"clientVersion"\s*:\s*"([^"]+)"`)
	reVisitor   = regexp.MustCompile(`"VISITOR_DATA"\s*:\s*"([^"]+)"`)
	reVisitor2  = regexp.MustCompile(`"visitorData"\s*:\s*"([^"]+)"`)
	reConsentV  = regexp.MustCompile(`name="v" value="([^"]+)"`)
)

// fetchViaInnertubeSession: watch HTML → session → multi-client player → timedtext.
func fetchViaInnertubeSession(ctx context.Context, videoID string, prefer []string, client *http.Client) (Result, error) {
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	session, err := fetchWatchSession(ctx, client, videoID)
	if err != nil {
		// Session optional: still try bare player with fixed clients.
		session = innertubeSession{}
	}

	var lastErr error
	var tracks []captionTrack
	var usedUA string
	for _, ic := range innertubeClients {
		t, err := listTracksWithSession(ctx, client, videoID, session, ic)
		if err != nil {
			lastErr = err
			if isHardBlock(err) {
				// try next client
				continue
			}
			// non-block errors also try next client
			continue
		}
		if len(t) == 0 {
			lastErr = fmt.Errorf("ytcaptions: no caption tracks (%s)", ic.id)
			continue
		}
		tracks = t
		usedUA = ic.ua
		break
	}
	if len(tracks) == 0 {
		if lastErr != nil {
			return Result{}, lastErr
		}
		return Result{}, fmt.Errorf("ytcaptions: no caption tracks")
	}

	track := pickTrack(tracks, prefer)
	if track.BaseURL == "" {
		return Result{}, fmt.Errorf("ytcaptions: empty caption url")
	}
	// Download timedtext with browser-like UA when possible.
	dlClient := client
	if usedUA != "" {
		// wrap not needed; downloadTrack sets android UA — use downloadTrackWithUA
	}
	cues, err := downloadTrackWithUA(ctx, dlClient, track.BaseURL, firstNonEmpty(usedUA, webUA))
	if err != nil {
		// one retry with android UA
		cues, err = downloadTrackWithUA(ctx, dlClient, track.BaseURL, androidUA)
		if err != nil {
			return Result{}, err
		}
	}
	res := Result{
		Language: track.LanguageCode,
		Kind:     track.Kind,
		Cues:     cues,
	}
	normalizeResult(&res)
	if res.Text == "" {
		return Result{}, fmt.Errorf("ytcaptions: empty transcript")
	}
	return res, nil
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func isHardBlock(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "login_required") ||
		strings.Contains(s, "bot") ||
		strings.Contains(s, "blocked") ||
		strings.Contains(s, "429")
}

func fetchWatchSession(ctx context.Context, client *http.Client, videoID string) (innertubeSession, error) {
	watchURL := watchURLBase + videoID + "&hl=en&persist_hl=1&has_verified=1&bpctr=9999999999"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	if err != nil {
		return innertubeSession{}, err
	}
	setWatchHeaders(req, "")
	resp, err := client.Do(req)
	if err != nil {
		return innertubeSession{}, fmt.Errorf("ytcaptions: watch page: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return innertubeSession{}, err
	}
	html := string(raw)
	// EU consent interstitial
	if strings.Contains(html, `action="https://consent.youtube.com/s"`) {
		if m := reConsentV.FindStringSubmatch(html); len(m) == 2 {
			req2, err := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
			if err != nil {
				return innertubeSession{}, err
			}
			setWatchHeaders(req2, "CONSENT=YES+"+m[1])
			resp2, err := client.Do(req2)
			if err != nil {
				return innertubeSession{}, err
			}
			defer resp2.Body.Close()
			raw2, err := io.ReadAll(io.LimitReader(resp2.Body, maxBody))
			if err != nil {
				return innertubeSession{}, err
			}
			html = string(raw2)
		}
	}
	if strings.Contains(html, `class="g-recaptcha"`) {
		return innertubeSession{}, fmt.Errorf("ytcaptions: IP blocked (reCAPTCHA)")
	}
	sess := innertubeSession{}
	if m := reAPIKey.FindStringSubmatch(html); len(m) == 2 {
		sess.APIKey = m[1]
	}
	if m := reClientVer.FindStringSubmatch(html); len(m) == 2 {
		sess.WebClientVersion = m[1]
	} else if m := reClientVer2.FindStringSubmatch(html); len(m) == 2 {
		sess.WebClientVersion = m[1]
	}
	if m := reVisitor.FindStringSubmatch(html); len(m) == 2 {
		sess.VisitorData = m[1]
	} else if m := reVisitor2.FindStringSubmatch(html); len(m) == 2 {
		sess.VisitorData = m[1]
	}
	if sess.WebClientVersion == "" {
		sess.WebClientVersion = defaultWebClient
	}
	// API key is nice-to-have; empty still allows unauthenticated player POSTs on some clients.
	return sess, nil
}

func setWatchHeaders(req *http.Request, cookie string) {
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("User-Agent", webUA)
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
}

func listTracksWithSession(ctx context.Context, client *http.Client, videoID string, sess innertubeSession, ic innertubeClient) ([]captionTrack, error) {
	ver := ic.version
	if ver == "" {
		ver = sess.WebClientVersion
		if ver == "" {
			ver = defaultWebClient
		}
	}
	clientCtx := map[string]any{
		"hl":            "en",
		"gl":            "US",
		"clientName":    ic.name,
		"clientVersion": ver,
	}
	if sess.VisitorData != "" {
		clientCtx["visitorData"] = sess.VisitorData
	}
	for k, v := range ic.extra {
		clientCtx[k] = v
	}
	payload := map[string]any{
		"context": map[string]any{
			"client":  clientCtx,
			"request": map[string]any{"useSsl": true},
		},
		"videoId": videoID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := innertubePlayer + "?prettyPrint=false"
	if sess.APIKey != "" {
		url = innertubePlayer + "?key=" + sess.APIKey + "&prettyPrint=false"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Origin", "https://www.youtube.com")
	req.Header.Set("Referer", watchURLBase+videoID)
	req.Header.Set("User-Agent", ic.ua)
	req.Header.Set("X-YouTube-Client-Name", ic.clientNameHeader)
	req.Header.Set("X-YouTube-Client-Version", ver)
	if sess.VisitorData != "" {
		req.Header.Set("X-Goog-Visitor-Id", sess.VisitorData)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ytcaptions: player: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("ytcaptions: IP blocked (429)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ytcaptions: player http %d", resp.StatusCode)
	}
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("ytcaptions: player json: %w", err)
	}
	if status, reason := playability(root); status == "LOGIN_REQUIRED" || status == "ERROR" || status == "UNPLAYABLE" {
		if reason == "" {
			reason = status
		}
		return nil, fmt.Errorf("ytcaptions: youtube blocked automated access (%s)", reason)
	}
	tracks := extractTracks(root)
	if len(tracks) == 0 {
		return nil, fmt.Errorf("ytcaptions: no caption tracks")
	}
	return tracks, nil
}

func downloadTrackWithUA(ctx context.Context, client *http.Client, baseURL, ua string) ([]Cue, error) {
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
		return nil, err
	}
	if ua == "" {
		ua = webUA
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ytcaptions: timedtext: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ytcaptions: timedtext http %d", resp.StatusCode)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("ytcaptions: empty timedtext")
	}
	if bytes.Contains(raw, []byte("<timedtext")) || bytes.Contains(raw, []byte("<p ")) {
		if cues := parseSrv3Cues(raw); len(cues) > 0 {
			return cues, nil
		}
	}
	if cues := parseJSON3Cues(raw); len(cues) > 0 {
		return cues, nil
	}
	if t := cleanCaptionText(string(raw)); t != "" {
		return []Cue{{StartMs: 0, Text: t}}, nil
	}
	return nil, fmt.Errorf("ytcaptions: empty timedtext")
}
