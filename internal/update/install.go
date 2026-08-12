package update

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// InstallResult is returned after scheduling replacement.
type InstallResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Latest  string `json:"latest,omitempty"`
	Asset   string `json:"asset,omitempty"`
}

// DownloadAndSchedule downloads the matching asset and schedules replace-on-exit.
func (c *Client) DownloadAndSchedule(ctx context.Context) (InstallResult, error) {
	current := NormalizeVersion(c.AppVersion)
	rel, err := c.LatestRelease(ctx)
	if err != nil {
		return InstallResult{OK: false, Message: err.Error()}, err
	}
	latest := NormalizeVersion(rel.TagName)
	if CompareVersions(current, latest) >= 0 {
		return InstallResult{OK: false, Message: "already_latest", Latest: latest}, fmt.Errorf("already_latest")
	}
	asset, err := PickAsset(rel.Assets, c.goos(), c.goarch())
	if err != nil {
		return InstallResult{OK: false, Message: err.Error(), Latest: latest}, err
	}

	dir := filepath.Join(os.TempDir(), "lrss-update")
	_ = os.MkdirAll(dir, 0o755)
	dest := filepath.Join(dir, asset.Name)
	// Fresh download each time.
	_ = os.Remove(dest)

	dlCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := DownloadFile(dlCtx, c.httpClient(), asset.BrowserDownloadURL, dest); err != nil {
		return InstallResult{OK: false, Message: err.Error(), Latest: latest, Asset: asset.Name}, err
	}

	if err := ApplyDownloaded(dest, asset.Name); err != nil {
		return InstallResult{OK: false, Message: err.Error(), Latest: latest, Asset: asset.Name}, err
	}
	return InstallResult{
		OK:      true,
		Message: RestartHint(),
		Latest:  latest,
		Asset:   asset.Name,
	}, nil
}
