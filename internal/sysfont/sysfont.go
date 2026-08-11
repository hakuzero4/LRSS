// Package sysfont lists installed font family names for reader typography settings.
package sysfont

import (
	"sort"
	"strings"
)

// CommonFamilies are cross-platform CSS families that usually resolve without
// enumerating the OS font catalog. Always included in List().
var CommonFamilies = []string{
	"system-ui",
	"Segoe UI",
	"Microsoft YaHei",
	"Microsoft YaHei UI",
	"SimSun",
	"SimHei",
	"KaiTi",
	"FangSong",
	"Noto Sans SC",
	"Noto Serif SC",
	"Source Han Sans SC",
	"PingFang SC",
	"Hiragino Sans GB",
	"Arial",
	"Helvetica Neue",
	"Georgia",
	"Times New Roman",
	"Palatino Linotype",
	"Consolas",
	"Cascadia Code",
	"Courier New",
	"Verdana",
	"Tahoma",
	"Trebuchet MS",
	"Comic Sans MS",
}

// List returns sorted unique font family names suitable for a CSS font-family
// picker. Includes common families plus OS-installed families when available.
func List() []string {
	seen := make(map[string]struct{}, 256)
	var out []string
	add := func(name string) {
		name = normalizeFamily(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	for _, n := range CommonFamilies {
		add(n)
	}
	for _, n := range listPlatform() {
		add(n)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

// normalizeFamily cleans registry / platform font display names.
func normalizeFamily(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	// Windows registry: "Arial (TrueType)", "微软雅黑 & Microsoft YaHei UI (TrueType)"
	if i := strings.Index(name, " ("); i > 0 {
		name = strings.TrimSpace(name[:i])
	}
	// Multi-family entries separated by & — keep first token (family for CSS).
	if i := strings.Index(name, " & "); i > 0 {
		name = strings.TrimSpace(name[:i])
	}
	name = strings.TrimSpace(name)
	// Reject empty / control / path-like junk.
	if name == "" || strings.ContainsAny(name, `/\<>|"`) {
		return ""
	}
	if len(name) > 80 {
		return ""
	}
	return name
}
