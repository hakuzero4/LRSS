package desktop

import (
	"log"
	"sync/atomic"

	"lrss/internal/appdata"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Hooks are callbacks the tray menu uses. All fields may be nil.
type Hooks struct {
	RefreshAll       func() error
	SetWebAccess     func(enabled bool) error
	WebAccessEnabled func() bool
	Locale           func() string // "zh-CN" | "en-US"
}

// Setup registers the always-on system tray, left-click = show window,
// and WindowClosing = hide unless beginQuit has been called.
// The returned beginQuit marks the next close as a real quit (no hide).
func Setup(app *application.App, win *application.WebviewWindow, icon []byte, hooks Hooks) (beginQuit func()) {
	var quitting atomic.Bool
	beginQuit = func() {
		quitting.Store(true)
	}
	if app == nil || win == nil {
		return beginQuit
	}

	systray := app.SystemTray.New()
	if len(icon) > 0 {
		systray.SetIcon(icon)
	}
	systray.SetLabel(appdata.DisplayName())
	systray.SetTooltip(appdata.DisplayName())

	var currentMenu *application.Menu
	rebuildMenu := func() {
		menu := buildTrayMenu(app, win, hooks, beginQuit)
		systray.SetMenu(menu)
		if currentMenu != nil {
			currentMenu.Destroy()
		}
		currentMenu = menu
	}
	rebuildMenu()

	// Do not AttachWindow — that docks a popup next to the tray.
	systray.OnClick(func() {
		showMainWindow(win)
	})
	systray.OnRightClick(func() {
		rebuildMenu()
		systray.OpenMenu()
	})

	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !quitting.Load() {
			win.Hide()
			e.Cancel()
		}
	})

	// Dock click (macOS): show the reader even when the window is hidden.
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(_ *application.ApplicationEvent) {
		showMainWindow(win)
	})

	app.OnShutdown(func() {
		beginQuit()
		systray.Destroy()
	})

	return beginQuit
}

func showMainWindow(win *application.WebviewWindow) {
	if win == nil {
		return
	}
	win.Show()
	win.Focus()
}

func buildTrayMenu(app *application.App, win *application.WebviewWindow, hooks Hooks, beginQuit func()) *application.Menu {
	locale := ""
	if hooks.Locale != nil {
		locale = hooks.Locale()
	}
	labels := LabelsForLocale(locale)

	webOn := false
	if hooks.WebAccessEnabled != nil {
		webOn = hooks.WebAccessEnabled()
	}

	menu := app.NewMenu()
	menu.Add(labels.OpenWindow).OnClick(func(_ *application.Context) {
		showMainWindow(win)
	})
	menu.Add(labels.RefreshFeeds).OnClick(func(_ *application.Context) {
		if hooks.RefreshAll == nil {
			return
		}
		go func() {
			if err := hooks.RefreshAll(); err != nil {
				log.Printf("tray: refresh feeds: %v", err)
			}
		}()
	})
	menu.Add(WebAccessActionLabel(labels, webOn)).OnClick(func(_ *application.Context) {
		if hooks.SetWebAccess == nil {
			return
		}
		next := !webOn
		go func() {
			if err := hooks.SetWebAccess(next); err != nil {
				log.Printf("tray: set web access %v: %v", next, err)
			}
		}()
	})
	menu.AddSeparator()
	menu.Add(labels.Quit).OnClick(func(_ *application.Context) {
		if beginQuit != nil {
			beginQuit()
		}
		app.Quit()
	})
	return menu
}
