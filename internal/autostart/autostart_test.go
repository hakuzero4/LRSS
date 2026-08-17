package autostart

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestQuotePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "no_spaces_win", in: `C:\LRSS\lrss.exe`, want: `C:\LRSS\lrss.exe`},
		{name: "spaces_win", in: `C:\Program Files\LRSS\lrss.exe`, want: `"C:\Program Files\LRSS\lrss.exe"`},
		{name: "no_spaces_unix", in: `/usr/bin/lrss`, want: `/usr/bin/lrss`},
		{name: "spaces_unix", in: `/home/user/My Apps/lrss`, want: `"/home/user/My Apps/lrss"`},
		{name: "tab", in: "C:\\LRSS\tbin\\lrss.exe", want: "\"C:\\LRSS\tbin\\lrss.exe\""},
		{name: "already_quoted", in: `"C:\Program Files\LRSS\lrss.exe"`, want: `"C:\Program Files\LRSS\lrss.exe"`},
		{name: "internal_quote", in: `/tmp/say "hi"/lrss`, want: `"/tmp/say \"hi\"/lrss"`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := quotePath(tc.in)
			if got != tc.want {
				t.Fatalf("quotePath(%q) = %q want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDesktopFile(t *testing.T) {
	t.Setenv("LRSS_PROFILE", "prod")
	tests := []struct {
		name string
		exe  string
		want []string
	}{
		{
			name: "plain",
			exe:  "/usr/bin/lrss",
			want: []string{
				"[Desktop Entry]",
				"Type=Application",
				"Name=LRSS",
				"Exec=/usr/bin/lrss",
			},
		},
		{
			name: "spaces",
			exe:  "/home/user/My Apps/lrss",
			want: []string{
				"[Desktop Entry]",
				"Type=Application",
				"Name=LRSS",
				`Exec="/home/user/My Apps/lrss"`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := desktopFile(tc.exe)
			if !strings.HasPrefix(got, "[Desktop Entry]\n") {
				t.Fatalf("missing header:\n%s", got)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Fatalf("missing %q in:\n%s", w, got)
				}
			}
		})
	}
}

func TestLaunchAgentPlist(t *testing.T) {
	t.Setenv("LRSS_PROFILE", "prod")
	tests := []struct {
		name string
		exe  string
		want []string
	}{
		{
			name: "app_bundle",
			exe:  "/Applications/LRSS.app/Contents/MacOS/LRSS",
			want: []string{
				"<key>Label</key>",
				"<string>com.lrss.app</string>",
				"<key>ProgramArguments</key>",
				"<string>/Applications/LRSS.app/Contents/MacOS/LRSS</string>",
				"<key>RunAtLoad</key>",
				"<true/>",
			},
		},
		{
			name: "xml_special",
			exe:  `/Users/a & b/<LRSS>`,
			want: []string{
				"<string>/Users/a &amp; b/&lt;LRSS&gt;</string>",
				"<string>com.lrss.app</string>",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := launchAgentPlist(tc.exe)
			if !strings.Contains(got, `<?xml version="1.0"`) {
				t.Fatalf("missing xml header:\n%s", got)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Fatalf("missing %q in:\n%s", w, got)
				}
			}
		})
	}
}

func TestAutostartPaths(t *testing.T) {
	t.Setenv("LRSS_PROFILE", "prod")
	home := filepath.Join("home", "me")
	gotDesk := desktopFilePath(home)
	wantDesk := filepath.Join(home, ".config", "autostart", "lrss.desktop")
	if gotDesk != wantDesk {
		t.Fatalf("desktop path = %q want %q", gotDesk, wantDesk)
	}
	gotAgent := launchAgentPath(home)
	wantAgent := filepath.Join(home, "Library", "LaunchAgents", "com.lrss.app.plist")
	if gotAgent != wantAgent {
		t.Fatalf("agent path = %q want %q", gotAgent, wantAgent)
	}
}

func TestAutostartPaths_DevProfile(t *testing.T) {
	t.Setenv("LRSS_PROFILE", "dev")
	home := filepath.Join("home", "me")
	gotDesk := desktopFilePath(home)
	wantDesk := filepath.Join(home, ".config", "autostart", "lrss-dev.desktop")
	if gotDesk != wantDesk {
		t.Fatalf("desktop path = %q want %q", gotDesk, wantDesk)
	}
	gotAgent := launchAgentPath(home)
	wantAgent := filepath.Join(home, "Library", "LaunchAgents", "com.lrss.app.dev.plist")
	if gotAgent != wantAgent {
		t.Fatalf("agent path = %q want %q", gotAgent, wantAgent)
	}
	body := desktopFile("/usr/bin/lrss")
	if !strings.Contains(body, "Name=LRSS Dev") {
		t.Fatalf("dev desktop name:\n%s", body)
	}
}

func TestWriteAndRemoveAutostartFile(t *testing.T) {
	dir := t.TempDir()
	path := desktopFilePath(dir)
	exists, err := fileExists(path)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("file should not exist yet")
	}
	body := desktopFile("/tmp/lrss")
	if err := writeAutostartFile(path, body); err != nil {
		t.Fatal(err)
	}
	exists, err = fileExists(path)
	if err != nil || !exists {
		t.Fatalf("after write exists=%v err=%v", exists, err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("content = %q", got)
	}
	if err := removeIfExists(path); err != nil {
		t.Fatal(err)
	}
	if err := removeIfExists(path); err != nil {
		t.Fatalf("second remove: %v", err)
	}
	exists, err = fileExists(path)
	if err != nil || exists {
		t.Fatalf("after remove exists=%v err=%v", exists, err)
	}
}

func TestSupportedAndStatusNow(t *testing.T) {
	switch runtime.GOOS {
	case "windows", "darwin", "linux":
		if !Supported() {
			t.Fatalf("expected supported on %s", runtime.GOOS)
		}
	default:
		if Supported() {
			t.Fatalf("expected unsupported on %s", runtime.GOOS)
		}
	}
	st := StatusNow()
	if st.Supported != Supported() {
		t.Fatalf("StatusNow.Supported = %v want %v (err=%q)", st.Supported, Supported(), st.Error)
	}
	if !st.Supported {
		if st.Enabled {
			t.Fatal("unsupported should not report enabled")
		}
		if err := Set(true); err == nil {
			t.Fatal("Set on unsupported should error")
		}
	}
}
