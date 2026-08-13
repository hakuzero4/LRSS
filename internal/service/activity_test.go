package service

import "testing"

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
