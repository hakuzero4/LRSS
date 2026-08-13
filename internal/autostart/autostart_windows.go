//go:build windows

package autostart

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const windowsRunKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

func isEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, windowsRunKeyPath, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, fmt.Errorf("open registry Run key: %w", err)
	}
	defer k.Close()

	val, _, err := k.GetStringValue(windowsRunValueName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read registry Run value: %w", err)
	}
	return strings.TrimSpace(val) != "", nil
}

func setEnabled(enabled bool) error {
	if !enabled {
		return deleteRunValue()
	}
	exe, err := resolveExe()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRunKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry Run key: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue(windowsRunValueName, quotePath(exe)); err != nil {
		return fmt.Errorf("set registry Run value: %w", err)
	}
	return nil
}

func deleteRunValue() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, windowsRunKeyPath, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return fmt.Errorf("open registry Run key: %w", err)
	}
	defer k.Close()
	err = k.DeleteValue(windowsRunValueName)
	if err == nil || err == registry.ErrNotExist {
		return nil
	}
	return fmt.Errorf("delete registry Run value: %w", err)
}
