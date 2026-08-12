//go:build !windows

package update

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ApplyDownloaded replaces the current binary or .app after process exit.
func ApplyDownloaded(downloadPath, assetName string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = filepath.Abs(exe)
	} else {
		exe, _ = filepath.Abs(exe)
	}
	downloadPath, err = filepath.Abs(downloadPath)
	if err != nil {
		return err
	}

	if runtime.GOOS == "darwin" && (IsZipName(assetName) || strings.Contains(strings.ToLower(assetName), ".app")) {
		return applyDarwinApp(downloadPath, exe)
	}

	// Linux binary or tar.gz
	src := downloadPath
	if IsTarGzName(assetName) || IsTarGzName(downloadPath) {
		extracted, err := extractBinaryFromTarGz(downloadPath)
		if err != nil {
			return err
		}
		src = extracted
	}
	if err := os.Chmod(src, 0o755); err != nil {
		return err
	}

	script := filepath.Join(os.TempDir(), fmt.Sprintf("lrss-update-%d.sh", time.Now().UnixNano()))
	// Wait for PID, then mv into place and relaunch.
	body := fmt.Sprintf(`#!/bin/sh
set -e
PID=%d
SRC=%q
DST=%q
while kill -0 "$PID" 2>/dev/null; do
  sleep 1
done
sleep 1
mv -f "$SRC" "$DST"
chmod +x "$DST"
nohup "$DST" >/dev/null 2>&1 &
rm -f %q
`, os.Getpid(), src, exe, script)

	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", script)
	cmd.Dir = os.TempDir()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start updater: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

func applyDarwinApp(zipPath, currentExe string) error {
	dir, err := os.MkdirTemp("", "lrss-upd-app-*")
	if err != nil {
		return err
	}
	if err := unzipAll(zipPath, dir); err != nil {
		return err
	}
	appPath, err := findAppBundle(dir)
	if err != nil {
		return err
	}

	// Prefer replacing the running .app if we are inside one.
	destApp := ""
	if i := strings.Index(currentExe, ".app/Contents/MacOS/"); i > 0 {
		destApp = currentExe[:i+4] // includes ".app"
	}
	if destApp == "" {
		// Fallback: install beside executable / to /Applications/LRSS.app
		destApp = "/Applications/LRSS.app"
	}

	script := filepath.Join(os.TempDir(), fmt.Sprintf("lrss-update-%d.sh", time.Now().UnixNano()))
	body := fmt.Sprintf(`#!/bin/sh
set -e
PID=%d
SRC=%q
DST=%q
while kill -0 "$PID" 2>/dev/null; do
  sleep 1
done
sleep 1
rm -rf "$DST"
mkdir -p "$(dirname "$DST")"
# Move extracted .app into place
mv "$SRC" "$DST"
# Ad-hoc re-sign so Gatekeeper is happier after replace
if command -v codesign >/dev/null 2>&1; then
  codesign --force --deep --sign - "$DST" 2>/dev/null || true
fi
open "$DST"
rm -f %q
`, os.Getpid(), appPath, destApp, script)

	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		return err
	}
	cmd := exec.Command("/bin/sh", script)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start updater: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

func extractBinaryFromTarGz(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	outDir := filepath.Join(os.TempDir(), fmt.Sprintf("lrss-untar-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	var found string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if strings.HasPrefix(base, "lrss") && !strings.Contains(base, ".") {
			dest := filepath.Join(outDir, base)
			w, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(w, tr); err != nil {
				w.Close()
				return "", err
			}
			w.Close()
			found = dest
			break
		}
	}
	if found == "" {
		return "", fmt.Errorf("binary_not_in_archive")
	}
	return found, nil
}

func RestartHint() string {
	return "quit_and_replace"
}
