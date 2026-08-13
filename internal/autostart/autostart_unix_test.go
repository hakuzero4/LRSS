//go:build darwin || linux

package autostart

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestUnixSetEnabled_TempHome(t *testing.T) {
	home := t.TempDir()
	restore := SetHomeDirForTest(home)
	t.Cleanup(restore)

	on, err := Enabled()
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("expected disabled in empty home")
	}

	if err := Set(true); err != nil {
		t.Fatal(err)
	}
	on, err = Enabled()
	if err != nil {
		t.Fatal(err)
	}
	if !on {
		t.Fatal("expected enabled after Set(true)")
	}

	var path string
	if runtime.GOOS == "darwin" {
		path = launchAgentPath(home)
	} else {
		path = desktopFilePath(home)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(s, "<string>com.lrss.app</string>") {
			t.Fatalf("plist missing label:\n%s", s)
		}
		if !strings.Contains(s, "<key>RunAtLoad</key>") {
			t.Fatalf("plist missing RunAtLoad:\n%s", s)
		}
	case "linux":
		if !strings.Contains(s, "Type=Application") || !strings.Contains(s, "Name=LRSS") {
			t.Fatalf("desktop file missing fields:\n%s", s)
		}
		if !strings.Contains(s, "Exec=") {
			t.Fatalf("desktop file missing Exec:\n%s", s)
		}
	}

	if err := Set(false); err != nil {
		t.Fatal(err)
	}
	if err := Set(false); err != nil {
		t.Fatalf("second Set(false): %v", err)
	}
	on, err = Enabled()
	if err != nil {
		t.Fatal(err)
	}
	if on {
		t.Fatal("expected disabled after Set(false)")
	}
}
