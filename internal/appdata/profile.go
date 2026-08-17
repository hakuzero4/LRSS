// Package appdata isolates the installed app from `wails3 task dev`.
// Production binaries keep %LOCALAPPDATA%/LRSS.
// Dev / test binaries (no production build tag) use LRSS-dev so they
// do not share SQLite, WebView2 user data, or autostart keys.
package appdata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

const (
	EnvProfile = "LRSS_PROFILE"  // prod | dev
	EnvDataDir = "LRSS_DATA_DIR" // absolute override for the SQLite directory

	dirProd = "LRSS"
	dirDev  = "LRSS-dev"
)

// IsDev reports whether this process is the development profile.
// Override with LRSS_PROFILE=prod|dev.
func IsDev() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(EnvProfile))) {
	case "prod", "production":
		return false
	case "dev", "development":
		return true
	default:
		return defaultDev()
	}
}

// DirName is the XDG / LocalAppData folder name.
func DirName() string {
	if IsDev() {
		return dirDev
	}
	return dirProd
}

// AppName is the Wails application.Name (and Windows autostart value).
func AppName() string {
	if IsDev() {
		return "LRSS Dev"
	}
	return "LRSS"
}

// DisplayName is shown in the window title and tray.
func DisplayName() string {
	if IsDev() {
		return "LRSS (dev)"
	}
	return "LRSS"
}

// DarwinLabel is the LaunchAgent identifier.
func DarwinLabel() string {
	if IsDev() {
		return "com.lrss.app.dev"
	}
	return "com.lrss.app"
}

// WindowsRunValue is the HKCU Run value name.
func WindowsRunValue() string {
	if IsDev() {
		return "LRSS-dev"
	}
	return "LRSS"
}

// DesktopFileName is the XDG autostart filename.
func DesktopFileName() string {
	if IsDev() {
		return "lrss-dev.desktop"
	}
	return "lrss.desktop"
}

// DataDir is the directory that holds lrss.db.
// LRSS_DATA_DIR, if set, wins.
func DataDir() (string, error) {
	if d := strings.TrimSpace(os.Getenv(EnvDataDir)); d != "" {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return "", fmt.Errorf("create LRSS_DATA_DIR: %w", err)
		}
		return d, nil
	}
	dir := filepath.Join(xdg.DataHome, DirName(), "data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	return dir, nil
}

// WebViewUserDataDir is a profile-private WebView2 folder.
// Empty means "leave Wails default" (production: %APPDATA%\lrss.exe).
func WebViewUserDataDir() (string, error) {
	if !IsDev() {
		return "", nil
	}
	dir := filepath.Join(xdg.DataHome, DirName(), "webview")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create webview data dir: %w", err)
	}
	return dir, nil
}
