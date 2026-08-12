//go:build !windows

package ytcaptions

import "os/exec"

func hideConsoleWindow(cmd *exec.Cmd) {}
