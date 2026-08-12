package update

import (
	"strconv"
	"strings"
)

// NormalizeVersion strips a leading "v" and pre-release/build metadata.
func NormalizeVersion(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// CompareVersions returns -1 if a<b, 0 if equal, 1 if a>b (semver major.minor.patch).
func CompareVersions(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	if n < 3 {
		n = 3
	}
	for i := 0; i < n; i++ {
		var da, db int
		if i < len(pa) {
			da = pa[i]
		}
		if i < len(pb) {
			db = pb[i]
		}
		if da < db {
			return -1
		}
		if da > db {
			return 1
		}
	}
	return 0
}

func versionParts(raw string) []int {
	s := NormalizeVersion(raw)
	if s == "" {
		return nil
	}
	segs := strings.Split(s, ".")
	out := make([]int, 0, len(segs))
	for _, p := range segs {
		n, err := strconv.Atoi(p)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}
