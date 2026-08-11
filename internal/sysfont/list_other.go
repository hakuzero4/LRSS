//go:build !windows

package sysfont

// listPlatform: non-Windows uses common families only (no registry).
// Future: macOS font book / fontconfig on Linux.
func listPlatform() []string {
	return nil
}
