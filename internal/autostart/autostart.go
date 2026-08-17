// Package autostart manages OS launch-at-login
// (Windows Run key, macOS LaunchAgent, XDG autostart).
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"lrss/internal/appdata"
)

// Status is the Wails-facing launch-at-login snapshot.
type Status struct {
	Enabled   bool   `json:"enabled"`
	Supported bool   `json:"supported"`
	Error     string `json:"error,omitempty"`
}

// Test-only hooks. Production code must not call these.
var (
	windowsRunValueName = ""
	userHomeDir         = os.UserHomeDir
	resolveExe          = resolveExecutable
)

func runValueName() string {
	if windowsRunValueName != "" {
		return windowsRunValueName
	}
	return appdata.WindowsRunValue()
}

// Supported reports whether this OS has a launch-at-login implementation.
func Supported() bool {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		return true
	default:
		return false
	}
}

// StatusNow returns the current launch-at-login state. It never panics;
// failures are reported in Status.Error.
func StatusNow() (st Status) {
	defer func() {
		if rec := recover(); rec != nil {
			st.Supported = Supported()
			st.Error = fmt.Sprint(rec)
		}
	}()
	st.Supported = Supported()
	if !st.Supported {
		return st
	}
	enabled, err := Enabled()
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.Enabled = enabled
	return st
}

// Enabled reports whether launch-at-login is currently turned on.
// On unsupported platforms it returns (false, nil).
func Enabled() (bool, error) {
	if !Supported() {
		return false, nil
	}
	return isEnabled()
}

// Set enables or disables launch-at-login.
// It returns an error if the platform is unsupported.
func Set(enabled bool) error {
	if !Supported() {
		return fmt.Errorf("launch at login is not supported on %s", runtime.GOOS)
	}
	return setEnabled(enabled)
}

// SetValueNameForTest overrides the Windows HKCU Run value name.
// Tests must restore via the returned function and delete any created value.
func SetValueNameForTest(name string) func() {
	prev := windowsRunValueName
	windowsRunValueName = name
	return func() { windowsRunValueName = prev }
}

// SetHomeDirForTest overrides the home directory used for LaunchAgent / .desktop paths.
func SetHomeDirForTest(dir string) func() {
	prev := userHomeDir
	userHomeDir = func() (string, error) { return dir, nil }
	return func() { userHomeDir = prev }
}

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return exe, nil
	}
	return abs, nil
}
