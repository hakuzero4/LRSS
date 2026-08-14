package service

import (
	"context"
	"testing"
)

func TestLibrary_InsertedTotal_IndependentOfHook(t *testing.T) {
	lib := NewLibrary(nil, nil, nil, nil)
	if lib.InsertedTotal() != 0 {
		t.Fatalf("idle inserted = %d", lib.InsertedTotal())
	}
	lib.emitInserted(context.Background(), []string{"a", "b"})
	if n := lib.InsertedTotal(); n != 2 {
		t.Fatalf("inserted = %d want 2", n)
	}
	lib.emitInserted(context.Background(), []string{"c"})
	if n := lib.InsertedTotal(); n != 3 {
		t.Fatalf("inserted = %d want 3", n)
	}
}

func TestLibrary_RefreshSnapshot(t *testing.T) {
	lib := NewLibrary(nil, nil, nil, nil)
	id, title, pending, queued := lib.RefreshSnapshot()
	if id != "" || title != "" || pending != 0 || len(queued) != 0 {
		t.Fatalf("idle = %q %q %d %v", id, title, pending, queued)
	}
	lib.beginRefreshFeed("f1", "NVIDIA Blog")
	id, title, _, _ = lib.RefreshSnapshot()
	if id != "f1" || title != "NVIDIA Blog" {
		t.Fatalf("active = %q %q", id, title)
	}
	lib.endRefreshFeed()
	id, title, _, _ = lib.RefreshSnapshot()
	if id != "" || title != "" {
		t.Fatalf("after end = %q %q", id, title)
	}
}
