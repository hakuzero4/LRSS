//go:build windows

package ytcaptions

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hideConsoleWindow prevents console apps (yt-dlp, python, py) from flashing
// a terminal when LRSS (a GUI / Wails app) runs them as subprocesses.
func hideConsoleWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
