package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"lrss/internal/httpx"
)

const (
	DefaultOwner = "hakuzero4"
	DefaultRepo  = "LRSS"
)

// ReleaseInfo is a simplified GitHub release.
type ReleaseInfo struct {
	TagName string
	Name    string
	HTMLURL string
	Assets  []Asset
}

// Asset is a downloadable release file.
type Asset struct {
	Name               string
	BrowserDownloadURL string
	Size               int64
}

// CheckResult is returned to the UI.
type CheckResult struct {
	Status   string `json:"status"` // upToDate | updateAvailable | error
	Current  string `json:"current"`
	Latest   string `json:"latest,omitempty"`
	HTMLURL  string `json:"htmlUrl,omitempty"`
	Name     string `json:"name,omitempty"`
	Message  string `json:"message,omitempty"`
	Asset    string `json:"asset,omitempty"`
	CanInstall bool `json:"canInstall"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int64  `json:"size"`
	} `json:"assets"`
}

// Client talks to GitHub Releases.
type Client struct {
	Owner      string
	Repo       string
	HTTP       *http.Client
	GOOS       string
	GOARCH     string
	AppVersion string
}

func (c *Client) owner() string {
	if c.Owner != "" {
		return c.Owner
	}
	return DefaultOwner
}

func (c *Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	return DefaultRepo
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return httpx.Std(httpx.Options{Timeout: 45 * time.Second, UserAgent: "LRSS-updater"})
}

func (c *Client) goos() string {
	if c.GOOS != "" {
		return c.GOOS
	}
	return runtime.GOOS
}

func (c *Client) goarch() string {
	if c.GOARCH != "" {
		return c.GOARCH
	}
	return runtime.GOARCH
}

// LatestRelease fetches /releases/latest.
func (c *Client) LatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", c.owner(), c.repo())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "LRSS-updater")

	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no_releases")
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http_%d", res.StatusCode)
	}
	var raw ghRelease
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid_release: %w", err)
	}
	if strings.TrimSpace(raw.TagName) == "" {
		return nil, fmt.Errorf("invalid_release")
	}
	info := &ReleaseInfo{
		TagName: raw.TagName,
		Name:    raw.Name,
		HTMLURL: raw.HTMLURL,
	}
	for _, a := range raw.Assets {
		info.Assets = append(info.Assets, Asset{
			Name:               a.Name,
			BrowserDownloadURL: a.BrowserDownloadURL,
			Size:               a.Size,
		})
	}
	return info, nil
}

// PickAsset chooses the best download for the current platform.
func PickAsset(assets []Asset, goos, goarch string) (Asset, error) {
	if len(assets) == 0 {
		return Asset{}, fmt.Errorf("no_assets")
	}
	names := make([]string, 0, len(assets))
	byName := map[string]Asset{}
	for _, a := range assets {
		n := strings.TrimSpace(a.Name)
		if n == "" || a.BrowserDownloadURL == "" {
			continue
		}
		// Prefer non-archive bare binaries when both exist.
		byName[n] = a
		names = append(names, n)
	}

	var candidates []string
	switch goos {
	case "windows":
		candidates = []string{
			fmt.Sprintf("lrss-windows-%s.exe", goarch),
			fmt.Sprintf("lrss-windows-%s.exe.zip", goarch),
			fmt.Sprintf("lrss-windows-%s.zip", goarch),
		}
	case "linux":
		candidates = []string{
			fmt.Sprintf("lrss-linux-%s", goarch),
			fmt.Sprintf("lrss-linux-%s.tar.gz", goarch),
		}
	case "darwin":
		// Prefer arch-specific .app, then universal.
		candidates = []string{
			fmt.Sprintf("LRSS-macOS-%s.app.zip", goarch),
			"LRSS-macOS-universal.app.zip",
			fmt.Sprintf("lrss-darwin-%s.tar.gz", goarch),
			fmt.Sprintf("lrss-darwin-%s", goarch),
		}
	default:
		return Asset{}, fmt.Errorf("unsupported_os:%s", goos)
	}

	for _, want := range candidates {
		if a, ok := byName[want]; ok {
			return a, nil
		}
		// case-insensitive fallback
		for _, n := range names {
			if strings.EqualFold(n, want) {
				return byName[n], nil
			}
		}
	}
	return Asset{}, fmt.Errorf("no_matching_asset")
}

// Check compares the running version to the latest GitHub release.
func (c *Client) Check(ctx context.Context) CheckResult {
	current := NormalizeVersion(c.AppVersion)
	if current == "" {
		current = "0.0.0"
	}
	rel, err := c.LatestRelease(ctx)
	if err != nil {
		msg := err.Error()
		return CheckResult{Status: "error", Current: current, Message: msg, CanInstall: false}
	}
	latest := NormalizeVersion(rel.TagName)
	html := rel.HTMLURL
	if html == "" {
		html = fmt.Sprintf("https://github.com/%s/%s/releases", c.owner(), c.repo())
	}
	asset, assetErr := PickAsset(rel.Assets, c.goos(), c.goarch())
	canInstall := assetErr == nil && asset.BrowserDownloadURL != ""

	if CompareVersions(current, latest) < 0 {
		cr := CheckResult{
			Status:     "updateAvailable",
			Current:    current,
			Latest:     latest,
			HTMLURL:    html,
			Name:       rel.Name,
			CanInstall: canInstall,
		}
		if canInstall {
			cr.Asset = asset.Name
		} else if assetErr != nil {
			cr.Message = assetErr.Error()
		}
		return cr
	}
	return CheckResult{
		Status:     "upToDate",
		Current:    current,
		Latest:     latest,
		HTMLURL:    html,
		CanInstall: false,
	}
}
