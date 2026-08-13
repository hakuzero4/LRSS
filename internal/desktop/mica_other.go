//go:build !windows

package desktop

// ApplyWindowMica is a no-op off Windows.
func ApplyWindowMica(hwnd uintptr, enabled bool) {}
