package ytcaptions

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kkdai/youtube/v2"
)

// fetchViaKKDAI uses github.com/kkdai/youtube/v2 (pure Go YouTube client)
// GetTranscript / CaptionTracks — the Go-side equivalent of youtube-dl style extraction.
func fetchViaKKDAI(ctx context.Context, videoID string, prefer []string, httpClient *http.Client) (Result, error) {
	client := youtube.Client{}
	if httpClient != nil {
		client.HTTPClient = httpClient
	}

	// 1) Preferred: transcript API by language (does not require full GetVideo).
	var lastErr error
	langs := make([]string, 0, len(prefer)+1)
	langs = append(langs, prefer...)
	langs = append(langs, "") // library default
	seen := map[string]bool{}
	for _, lang := range langs {
		if seen[lang] {
			continue
		}
		seen[lang] = true
		v := &youtube.Video{ID: videoID}
		tr, err := client.GetTranscriptCtx(ctx, v, lang)
		if err != nil {
			lastErr = err
			continue
		}
		cues := transcriptCues(tr)
		if len(cues) == 0 {
			lastErr = fmt.Errorf("ytcaptions: empty kkdai transcript")
			continue
		}
		kind := ""
		if lang == "" {
			lang = "en"
		}
		res := Result{Language: lang, Kind: kind, Cues: cues}
		res.fillTextFromCues()
		return res, nil
	}

	// 2) Fallback: resolve video metadata for caption track base URLs, then download timedtext.
	video, err := client.GetVideoContext(ctx, "https://www.youtube.com/watch?v="+videoID)
	if err != nil {
		if lastErr != nil {
			return Result{}, fmt.Errorf("ytcaptions: kkdai transcript: %v; get video: %w", lastErr, err)
		}
		return Result{}, fmt.Errorf("ytcaptions: kkdai get video: %w", err)
	}
	if len(video.CaptionTracks) == 0 {
		if lastErr != nil {
			return Result{}, fmt.Errorf("ytcaptions: kkdai: %w", lastErr)
		}
		return Result{}, fmt.Errorf("ytcaptions: kkdai: no caption tracks")
	}

	tracks := make([]captionTrack, 0, len(video.CaptionTracks))
	for _, t := range video.CaptionTracks {
		tracks = append(tracks, captionTrack{
			BaseURL:      t.BaseURL,
			LanguageCode: t.LanguageCode,
			Kind:         t.Kind,
			Name:         t.Name.SimpleText,
		})
	}
	track := pickTrack(tracks, prefer)
	if track.BaseURL == "" {
		return Result{}, fmt.Errorf("ytcaptions: kkdai: empty caption url")
	}

	hc := httpClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	cues, err := downloadTrackWithUA(ctx, hc, track.BaseURL, webUA)
	if err != nil {
		return Result{}, fmt.Errorf("ytcaptions: kkdai timedtext: %w", err)
	}
	res := Result{
		Language: track.LanguageCode,
		Kind:     track.Kind,
		Cues:     cues,
	}
	normalizeResult(&res)
	if res.Text == "" {
		return Result{}, fmt.Errorf("ytcaptions: kkdai empty timedtext")
	}
	return res, nil
}

func transcriptCues(tr youtube.VideoTranscript) []Cue {
	cues := make([]Cue, 0, len(tr))
	for _, seg := range tr {
		t := strings.TrimSpace(seg.Text)
		if t == "" {
			continue
		}
		cues = append(cues, Cue{StartMs: seg.StartMs, Text: t})
	}
	return cues
}
