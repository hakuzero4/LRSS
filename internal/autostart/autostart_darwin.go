//go:build darwin

package autostart

import "fmt"

func agentFile() (string, error) {
	home, err := userHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return launchAgentPath(home), nil
}

func isEnabled() (bool, error) {
	path, err := agentFile()
	if err != nil {
		return false, err
	}
	return fileExists(path)
}

func setEnabled(enabled bool) error {
	path, err := agentFile()
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
	return writeAutostartFile(path, launchAgentPlist(exe))
}
