package sysfont_test

import (
	"strings"
	"testing"

	"lrss/internal/sysfont"
)

func TestList_NonEmptyAndSorted(t *testing.T) {
	list := sysfont.List()
	if len(list) < 5 {
		t.Fatalf("expected several fonts, got %d", len(list))
	}
	// Must include a common family.
	found := false
	for _, n := range list {
		if strings.EqualFold(n, "Arial") || strings.EqualFold(n, "Segoe UI") || strings.EqualFold(n, "system-ui") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing common family in %v", list[:min(10, len(list))])
	}
	// Unique (case-insensitive) and sorted.
	prev := ""
	seen := map[string]struct{}{}
	for _, n := range list {
		if n == "" {
			t.Fatal("empty name")
		}
		key := strings.ToLower(n)
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate %q", n)
		}
		seen[key] = struct{}{}
		if prev != "" && strings.ToLower(prev) > key {
			t.Fatalf("not sorted: %q before %q", prev, n)
		}
		prev = n
	}
}

func TestNormalizeViaList_NoJunk(t *testing.T) {
	for _, n := range sysfont.List() {
		if strings.Contains(n, "(TrueType)") {
			t.Fatalf("registry suffix not stripped: %q", n)
		}
		if strings.ContainsAny(n, `/\<>|"`) {
			t.Fatalf("unsafe chars: %q", n)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
