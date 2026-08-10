package id

import (
	"testing"
)

func TestNew(t *testing.T) {
	a := New()
	b := New()
	if a == "" {
		t.Fatal("New() returned empty string")
	}
	if len(a) != 26 {
		t.Fatalf("ULID length = %d, want 26", len(a))
	}
	if a == b {
		t.Fatal("two New() calls returned the same id")
	}
}

func TestNewUnique(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := New()
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id at iteration %d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}
