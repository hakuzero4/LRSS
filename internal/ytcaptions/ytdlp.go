package ytcaptions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// yt-dlp JSON (-J) fields used for caption track URLs.
type ytDlpInfo struct {
	Subtitles         map[string][]ytDlpTrackEntry `json:"subtitles"`
	AutomaticCaptions map[string][]ytDlpTrackEntry `json:"automatic_captions"`
}

type ytDlpTrackEntry struct {
	URL  string `json:"url"`
	Ext  string `json:"ext"`
	Name string `json:"name"`
}

// detectYtDlp returns a command that can run yt-dlp, or empty if unavailable.
func detectYtDlp() (bin string, prefixArgs []string, ok bool) {
	candidates := []struct {
		bin  string
		args []string
	}{
		{"yt-dlp", nil},
		{"yt-dlp.exe", nil},
		{"python", []string{"-m", "yt_dlp"}},
		{"python3", []string{"-m", "yt_dlp"}},
		{"py", []string{"-3", "-m", "yt_dlp"}},
	}
	for _, c := range candidates {
		path, err := exec.LookPath(c.bin)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		cmd := exec.CommandContext(ctx, path, append(append([]string{}, c.args...), "--version")...)
		err = cmd.Run()
		cancel()
		if err == nil {
			return path, c.args, true
		}
	}
	return "", nil, false
}

// fetchViaYtDlp uses a local yt-dlp binary when InnerTube is blocked.
// Optional env YOUTUBE_TRANSCRIPT_COOKIES_FROM_BROWSER (e.g. chrome, edge) is
// passed through as --cookies-from-browser (same idea as baoyu skill).
func fetchViaYtDlp(ctx context.Context, videoID string, prefer []string, httpClient *http.Client) (Result, error) {
	bin, prefix, ok := detectYtDlp()
	if !ok {
		return Result{}, fmt.Errorf("ytcaptions: yt-dlp not found")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	args := append(append([]string{}, prefix...),
		"-J",
		"--skip-download",
		"--no-warnings",
		"https://www.youtube.com/watch?v="+videoID,
	)
	if browser := strings.TrimSpace(os.Getenv("YOUTUBE_TRANSCRIPT_COOKIES_FROM_BROWSER")); browser != "" {
		args = append(args, "--cookies-from-browser", browser)
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return Result{}, fmt.Errorf("ytcaptions: yt-dlp: %w (%s)", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return Result{}, fmt.Errorf("ytcaptions: yt-dlp: %w", err)
	}
	var info ytDlpInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return Result{}, fmt.Errorf("ytcaptions: yt-dlp json: %w", err)
	}

	tracks := tracksFromYtDlp(info)
	if len(tracks) == 0 {
		return Result{}, fmt.Errorf("ytcaptions: yt-dlp: no caption tracks")
	}
	track := pickTrack(tracks, prefer)
	if track.BaseURL == "" {
		return Result{}, fmt.Errorf("ytcaptions: yt-dlp: empty caption url")
	}
	cues, err := downloadTrackWithUA(ctx, httpClient, track.BaseURL, webUA)
	if err != nil {
		return Result{}, err
	}
	res := Result{
		Language: track.LanguageCode,
		Kind:     track.Kind,
		Cues:     cues,
	}
	normalizeResult(&res)
	if res.Text == "" {
		return Result{}, fmt.Errorf("ytcaptions: yt-dlp empty transcript")
	}
	return res, nil
}

func tracksFromYtDlp(info ytDlpInfo) []captionTrack {
	var out []captionTrack
	preferExt := []string{"json3", "srv3", "srv2", "srv1", "vtt", "ttml"}
	pickURL := func(entries []ytDlpTrackEntry) string {
		for _, ext := range preferExt {
			for _, e := range entries {
				if e.URL != "" && strings.EqualFold(e.Ext, ext) {
					return e.URL
				}
			}
		}
		for _, e := range entries {
			if e.URL != "" {
				return e.URL
			}
		}
		return ""
	}
	for lang, entries := range info.Subtitles {
		if u := pickURL(entries); u != "" {
			out = append(out, captionTrack{BaseURL: u, LanguageCode: lang, Kind: ""})
		}
	}
	for lang, entries := range info.AutomaticCaptions {
		if u := pickURL(entries); u != "" {
			out = append(out, captionTrack{BaseURL: u, LanguageCode: lang, Kind: "asr"})
		}
	}
	return out
}
