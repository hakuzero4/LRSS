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
// Accepts both unversioned names (lrss-windows-amd64.exe) and
// versioned ones (lrss-0.1.12-windows-amd64.exe).
func PickAsset(assets []Asset, goos, goarch string) (Asset, error) {
	if len(assets) == 0 {
		return Asset{}, fmt.Errorf("no_assets")
	}
	var best Asset
	bestScore := 0
	for _, a := range assets {
		n := strings.TrimSpace(a.Name)
		if n == "" || a.BrowserDownloadURL == "" {
			continue
		}
		if s := assetScore(n, goos, goarch); s > bestScore {
			best = a
			bestScore = s
		}
	}
	if bestScore == 0 {
		return Asset{}, fmt.Errorf("no_matching_asset")
	}
	return best, nil
}

func assetScore(name, goos, goarch string) int {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || n == "sha256sums.txt" {
		return 0
	}
	// Installers are for humans; in-app update must keep the portable binary.
	if strings.Contains(n, "setup") || strings.Contains(n, "installer") || strings.Contains(n, "nsis") {
		return 0
	}
	arch := strings.ToLower(strings.TrimSpace(goarch))
	if arch == "" || !strings.Contains(n, arch) {
		return 0
	}
	switch goos {
	case "windows":
		if !strings.Contains(n, "windows") {
			return 0
		}
		if strings.HasSuffix(n, ".exe") {
			return 30
		}
		if strings.HasSuffix(n, ".zip") {
			return 20
		}
	case "linux":
		if !strings.Contains(n, "linux") {
			return 0
		}
		if strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz") {
			return 20
		}
		if strings.Contains(n, ".zip") || strings.HasSuffix(n, ".exe") {
			return 0
		}
		return 30
	case "darwin":
		if !strings.Contains(n, "macos") && !strings.Contains(n, "darwin") {
			return 0
		}
		if strings.Contains(n, "universal") {
			return 0
		}
		if strings.HasSuffix(n, ".app.zip") {
			return 30
		}
		if strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz") {
			return 15
		}
		if !strings.Contains(n, ".zip") {
			return 10
		}
	}
	return 0
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
