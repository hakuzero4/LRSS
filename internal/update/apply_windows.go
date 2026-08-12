//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ApplyDownloaded replaces the running Windows executable after exit, then relaunches.
func ApplyDownloaded(downloadPath, assetName string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	downloadPath, err = filepath.Abs(downloadPath)
	if err != nil {
		return err
	}

	// If we got a zip, extract the .exe first.
	src := downloadPath
	if IsZipName(assetName) || strings.EqualFold(filepath.Ext(downloadPath), ".zip") {
		extracted, err := extractWindowsExeFromZip(downloadPath)
		if err != nil {
			return err
		}
		src = extracted
	}

	bat := filepath.Join(os.TempDir(), fmt.Sprintf("lrss-update-%d.bat", time.Now().UnixNano()))
	// cmd.exe batch: wait for PID, copy, start, cleanup.
	pid := os.Getpid()
	content := strings.Join([]string{
		"@echo off",
		"setlocal",
		fmt.Sprintf("set PID=%d", pid),
		fmt.Sprintf("set SRC=%s", quoteBat(src)),
		fmt.Sprintf("set DST=%s", quoteBat(exe)),
		":wait",
		"tasklist /FI \"PID eq %PID%\" 2>NUL | find \"%PID%\" >NUL",
		"if not errorlevel 1 (",
		"  timeout /t 1 /nobreak >NUL",
		"  goto wait",
		")",
		"timeout /t 1 /nobreak >NUL",
		"copy /Y %SRC% %DST% >NUL",
		"if errorlevel 1 (",
		"  echo LRSS update copy failed.",
		"  exit /b 1",
		")",
		"start \"\" %DST%",
		"del %SRC% >NUL 2>&1",
		fmt.Sprintf("del %s >NUL 2>&1", quoteBat(bat)),
		"endlocal",
	}, "\r\n") + "\r\n"

	if err := os.WriteFile(bat, []byte(content), 0o700); err != nil {
		return err
	}

	cmd := exec.Command("cmd.exe", "/C", "start", "", bat)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x00000008, // DETACHED_PROCESS
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start updater: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

func quoteBat(p string) string {
	// Batch: wrap in double quotes; double any embedded quotes.
	return `"` + strings.ReplaceAll(p, `"`, `""`) + `"`
}

func extractWindowsExeFromZip(zipPath string) (string, error) {
	// Prefer archive/zip without pulling heavy deps — use PowerShell Expand-Archive is fragile.
	// Use Go zip.
	return extractNamedFromZip(zipPath, ".exe")
}

// RestartHint is shown to the UI before quit.
func RestartHint() string {
	return "quit_and_replace"
}

// PIDString is used in scripts (exported for tests).
func PIDString() string {
	return strconv.Itoa(os.Getpid())
}
