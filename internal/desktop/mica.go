package desktop

import (
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// dwmwaColorNone is DWMWA_COLOR_NONE (0xFFFFFFFE). Setting the caption to this
// hides the solid title-bar fill so Mica shows through. 0xFFFFFFFF is
// DWMWA_COLOR_DEFAULT, not "white".
const dwmwaColorNone uint32 = 0xFFFFFFFE

// WindowsThemeFromPref maps UIPrefs.theme to a Wails Windows title-bar / Mica theme.
// "system" and unknown values follow the OS.
func WindowsThemeFromPref(theme string) application.Theme {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "dark":
		return application.Dark
	case "light":
		return application.Light
	default:
		return application.SystemDefault
	}
}

func micaCaptionTheme() *application.WindowTheme {
	none := dwmwaColorNone
	return &application.WindowTheme{TitleBarColour: &none}
}

// ApplyMica prepares the Windows window for Mica. Hardware acceleration is
// required (WebView2 --disable-gpu paints black instead of DWM material).
//
// When micaEnabled is false but GPU is on, the window stays translucent so
// the user can toggle Mica later without recreating it. BackdropType is then
// None and the caption uses the system default (solid).
//
// Returns true when Mica is actually requested this launch.
func ApplyMica(opts *application.WebviewWindowOptions, hardwareAccel, micaEnabled bool, theme string) bool {
	if opts == nil || runtime.GOOS != "windows" {
		return false
	}
	opts.Windows.Theme = WindowsThemeFromPref(theme)
	if !hardwareAccel {
		return false
	}
	// Translucent so DWM backdrops (and runtime toggles) can show through.
	opts.BackgroundType = application.BackgroundTypeTranslucent
	opts.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
	if !micaEnabled {
		opts.Windows.BackdropType = application.None
		return false
	}
	opts.Windows.BackdropType = application.Mica
	caption := micaCaptionTheme()
	opts.Windows.CustomTheme = application.ThemeSettings{
		LightModeActive:   caption,
		LightModeInactive: caption,
		DarkModeActive:    caption,
		DarkModeInactive:  caption,
	}
	return true
}

// ApplyWindowMicaFrom toggles DWM Mica on an existing Wails window.
func ApplyWindowMicaFrom(win *application.WebviewWindow, enabled bool) {
	if win == nil {
		return
	}
	ApplyWindowMica(uintptr(win.NativeWindow()), enabled)
}
