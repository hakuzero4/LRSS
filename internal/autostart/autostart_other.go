//go:build !windows && !darwin

package autostart

import "fmt"

func desktopPath() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return desktopFilePath(home), nil
}

func isEnabled() (bool, error) {
	path, err := desktopPath()
	if err != nil {
		return false, err
	}
	return fileExists(path)
}

func setEnabled(enabled bool) error {
	path, err := desktopPath()
	if err != nil {
		return err
	}
	if !enabled {
		return removeIfExists(path)
	}
	exe, err := resolveExe()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	return writeAutostartFile(path, desktopFile(exe))
}
