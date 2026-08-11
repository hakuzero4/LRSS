//go:build windows

package sysfont

import (
	"golang.org/x/sys/windows/registry"
)

// listPlatform enumerates font face names from the Windows Fonts registry.
func listPlatform() []string {
	var out []string
	out = append(out, readFontsKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`)...)
	out = append(out, readFontsKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`)...)
	// Also list font families (Win10+), which are cleaner family names.
	out = append(out, readFontsKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\FontLink\SystemLink`)...)
	return out
}

func readFontsKey(root registry.Key, path string) []string {
	k, err := registry.OpenKey(root, path, registry.READ)
	if err != nil {
		return nil
	}
	defer k.Close()

	names, err := k.ReadValueNames(0)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n == "" {
			continue
		}
		out = append(out, n)
	}
	return out
}
