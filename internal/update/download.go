package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DownloadFile downloads url to destPath (creates parent dirs).
func DownloadFile(ctx context.Context, client *http.Client, url, destPath string) error {
	if client == nil {
		return fmt.Errorf("nil http client")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LRSS-updater")
	req.Header.Set("Accept", "application/octet-stream")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download http_%d", res.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	tmp := destPath + ".partial"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(f, res.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return closeErr
	}
	_ = os.Remove(destPath)
	if err := os.Rename(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// IsZipName reports whether the asset looks like a zip.
func IsZipName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".zip")
}

// IsTarGzName reports whether the asset looks like tar.gz.
func IsTarGzName(name string) bool {
	n := strings.ToLower(name)
	return strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz")
}
