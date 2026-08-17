package autostart

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"lrss/internal/appdata"
)

// quotePath wraps path in double quotes when it contains whitespace.
// Already-quoted paths are returned unchanged.
func quotePath(path string) string {
	if path == "" {
		return ""
	}
	if len(path) >= 2 && path[0] == '"' && strings.HasSuffix(path, `"`) {
		return path
	}
	if strings.ContainsAny(path, " \t\"") {
		return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
	}
	return path
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// desktopFile builds an XDG autostart .desktop body.
func desktopFile(exe string) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Name=")
	b.WriteString(appdata.AppName())
	b.WriteString("\n")
	b.WriteString("Exec=")
	b.WriteString(quotePath(exe))
	b.WriteString("\n")
	return b.String()
}

// launchAgentPlist builds a macOS LaunchAgent plist (Label com.lrss.app).
func launchAgentPlist(exe string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">` + "\n")
	b.WriteString("<plist version=\"1.0\">\n")
	b.WriteString("<dict>\n")
	b.WriteString("\t<key>Label</key>\n")
	b.WriteString("\t<string>")
	b.WriteString(appdata.DarwinLabel())
	b.WriteString("</string>\n")
	b.WriteString("\t<key>ProgramArguments</key>\n")
	b.WriteString("\t<array>\n")
	b.WriteString("\t\t<string>")
	b.WriteString(xmlEscape(exe))
	b.WriteString("</string>\n")
	b.WriteString("\t</array>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n")
	b.WriteString("\t<true/>\n")
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

func desktopFilePath(home string) string {
	return filepath.Join(home, ".config", "autostart", appdata.DesktopFileName())
}

func launchAgentPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", appdata.DarwinLabel()+".plist")
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func writeAutostartFile(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(contents), 0o644)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
