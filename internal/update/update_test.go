package update_test

import (
	"testing"

	"lrss/internal/update"
)

func TestCompareVersions(t *testing.T) {
	if update.CompareVersions("0.1.0", "0.1.1") >= 0 {
		t.Fatal("0.1.0 should be older than 0.1.1")
	}
	if update.CompareVersions("v0.2.0", "0.1.9") <= 0 {
		t.Fatal("0.2.0 should be newer")
	}
	if update.CompareVersions("0.1.1", "v0.1.1") != 0 {
		t.Fatal("equal")
	}
	if update.CompareVersions("0.2.0", "0.10.0") >= 0 {
		t.Fatal("numeric compare")
	}
}

func TestPickAsset(t *testing.T) {
	assets := []update.Asset{
		{Name: "lrss-windows-amd64.exe", BrowserDownloadURL: "https://x/win"},
		{Name: "lrss-linux-amd64", BrowserDownloadURL: "https://x/lin"},
		{Name: "LRSS-macOS-arm64.app.zip", BrowserDownloadURL: "https://x/mac"},
		{Name: "LRSS-macOS-universal.app.zip", BrowserDownloadURL: "https://x/uni"},
		{Name: "SHA256SUMS.txt", BrowserDownloadURL: "https://x/sum"},
	}
	a, err := update.PickAsset(assets, "windows", "amd64")
	if err != nil || a.Name != "lrss-windows-amd64.exe" {
		t.Fatalf("windows: %+v %v", a, err)
	}
	a, err = update.PickAsset(assets, "linux", "amd64")
	if err != nil || a.Name != "lrss-linux-amd64" {
		t.Fatalf("linux: %+v %v", a, err)
	}
	a, err = update.PickAsset(assets, "darwin", "arm64")
	if err != nil || a.Name != "LRSS-macOS-arm64.app.zip" {
		t.Fatalf("darwin arm64: %+v %v", a, err)
	}
	a, err = update.PickAsset(assets, "darwin", "amd64")
	if err != nil || a.Name != "LRSS-macOS-universal.app.zip" {
		// no amd64-specific in list — universal fallback
		if err != nil {
			t.Fatal(err)
		}
		if a.Name != "LRSS-macOS-universal.app.zip" {
			t.Fatalf("want universal got %s", a.Name)
		}
	}
}
