package desktop

import (
	"runtime"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestWindowsThemeFromPref(t *testing.T) {
	tests := []struct {
		in   string
		want application.Theme
	}{
		{"dark", application.Dark},
		{"Dark", application.Dark},
		{"light", application.Light},
		{"LIGHT", application.Light},
		{"system", application.SystemDefault},
		{"", application.SystemDefault},
		{"  system  ", application.SystemDefault},
		{"nope", application.SystemDefault},
	}
	for _, tt := range tests {
		if got := WindowsThemeFromPref(tt.in); got != tt.want {
			t.Errorf("WindowsThemeFromPref(%q) = %v want %v", tt.in, got, tt.want)
		}
	}
}

func TestApplyMicaDisabled(t *testing.T) {
	opts := application.WebviewWindowOptions{
		BackgroundColour: application.NewRGB(246, 247, 249),
	}
	if ApplyMica(nil, true, true, "light") {
		t.Fatal("nil opts must not enable mica")
	}
	got := ApplyMica(&opts, false, true, "dark")
	if runtime.GOOS == "windows" {
		if got {
			t.Fatal("hardware accel off must not enable mica")
		}
		if opts.Windows.Theme != application.Dark {
			t.Fatalf("theme = %v want Dark (title bar still follows prefs)", opts.Windows.Theme)
		}
		if opts.Windows.BackdropType == application.Mica {
			t.Fatal("BackdropType must stay unset when accel is off")
		}
		if opts.BackgroundType != 0 {
			t.Fatalf("BackgroundType = %v want default solid", opts.BackgroundType)
		}
	} else if got {
		t.Fatal("ApplyMica must be a no-op off Windows")
	}
}

func TestApplyMicaEnabled(t *testing.T) {
	opts := application.WebviewWindowOptions{
		BackgroundColour: application.NewRGB(246, 247, 249),
	}
	got := ApplyMica(&opts, true, true, "light")
	if runtime.GOOS != "windows" {
		if got {
			t.Fatal("ApplyMica must be a no-op off Windows")
		}
		if opts.Windows.BackdropType == application.Mica {
			t.Fatal("must not set Mica off Windows")
		}
		return
	}
	if !got {
		t.Fatal("expected mica on Windows with hardware accel")
	}
	if opts.BackgroundType != application.BackgroundTypeTranslucent {
		t.Fatalf("BackgroundType = %v want Translucent", opts.BackgroundType)
	}
	if opts.Windows.BackdropType != application.Mica {
		t.Fatalf("BackdropType = %v want Mica", opts.Windows.BackdropType)
	}
	if opts.Windows.Theme != application.Light {
		t.Fatalf("Theme = %v want Light", opts.Windows.Theme)
	}
	c := opts.BackgroundColour
	if c.Red != 0 || c.Green != 0 || c.Blue != 0 || c.Alpha != 0 {
		t.Fatalf("BackgroundColour = %+v want transparent", c)
	}
	if opts.Windows.CustomTheme.LightModeActive == nil ||
		opts.Windows.CustomTheme.LightModeActive.TitleBarColour == nil ||
		*opts.Windows.CustomTheme.LightModeActive.TitleBarColour != dwmwaColorNone {
		t.Fatal("title bar must use DWMWA_COLOR_NONE so Mica shows in the caption")
	}
}

func TestApplyMicaPrefOffKeepsTranslucent(t *testing.T) {
	opts := application.WebviewWindowOptions{
		BackgroundColour: application.NewRGB(246, 247, 249),
	}
	got := ApplyMica(&opts, true, false, "light")
	if got {
		t.Fatal("pref off must not report mica enabled")
	}
	if runtime.GOOS != "windows" {
		return
	}
	if opts.BackgroundType != application.BackgroundTypeTranslucent {
		t.Fatalf("BackgroundType = %v want Translucent (so toggle can turn mica on)", opts.BackgroundType)
	}
	if opts.Windows.BackdropType != application.None {
		t.Fatalf("BackdropType = %v want None", opts.Windows.BackdropType)
	}
	if opts.Windows.CustomTheme.LightModeActive != nil {
		t.Fatal("caption theme must stay default when mica is off")
	}
}
