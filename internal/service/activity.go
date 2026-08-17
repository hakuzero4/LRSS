package service

import "strings"

// JobActivity is a live snapshot of background refresh + briefing work.
type JobActivity struct {
	Refreshing       bool     `json:"refreshing"`
	FeedID           string   `json:"feedId,omitempty"`
	FeedTitle        string   `json:"feedTitle,omitempty"`
	Pending          int      `json:"pending"`
	QueuedTitles     []string `json:"queuedTitles,omitempty"`
	BriefingState    string   `json:"briefingState,omitempty"` // queued | generating
	BriefingPending  int      `json:"briefingPending,omitempty"`
	BriefingArticles int      `json:"briefingArticles,omitempty"`
	KeepState        string   `json:"keepState,omitempty"` // queued | judging
	KeepPending      int      `json:"keepPending,omitempty"`
	KeepLast         int      `json:"keepLast,omitempty"`
	// KeepLog is the newest keep/skip decisions (capped) so the UI can show why.
	KeepLog []KeepDecision `json:"keepLog,omitempty"`
	// ArticlesAdded is a process-lifetime insert counter (not a per-tick delta).
	ArticlesAdded int `json:"articlesAdded"`
}

func (lib *Library) beginRefreshFeed(id, title string) {
	if lib == nil {
		return
	}
	lib.actMu.Lock()
	lib.actFeedID = strings.TrimSpace(id)
	lib.actFeedTitle = strings.TrimSpace(title)
	lib.actMu.Unlock()
}

func (lib *Library) endRefreshFeed() {
	if lib == nil {
		return
	}
	lib.actMu.Lock()
	lib.actFeedID = ""
	lib.actFeedTitle = ""
	lib.actMu.Unlock()
}

// RefreshSnapshot returns the feed currently being fetched and force-queue leftovers.
func (lib *Library) RefreshSnapshot() (id, title string, pending int, queuedIDs []string) {
	if lib == nil {
		return "", "", 0, nil
	}
	lib.actMu.Lock()
	id, title = lib.actFeedID, lib.actFeedTitle
	lib.actMu.Unlock()
	lib.forceMu.Lock()
	pending = len(lib.forceIDs)
	n := 3
	if n > pending {
		n = pending
	}
	if n > 0 {
		queuedIDs = append([]string(nil), lib.forceIDs[:n]...)
	}
	lib.forceMu.Unlock()
	return id, title, pending, queuedIDs
}
