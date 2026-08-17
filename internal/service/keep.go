package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"lrss/internal/llm"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/settings"
)

const (
	KeyKeepPending        = "app.keep_pending"
	keepDebounce          = 20 * time.Second
	keepBatchSize         = 12
	keepMaxBatchesPerTick = 2
	keepScanUnreadCap     = 80
	keepJudgeTimeout      = 90 * time.Second
)

type keepPending struct {
	IDs           []string `json:"ids"`
	LastEnqueueAt string   `json:"lastEnqueueAt"`
}

// KeepWorker buffers article IDs and asks the LLM which to put in 精选.
type KeepWorker struct {
	store       *settings.Store
	articles    ArticleStore
	feeds       FeedStore
	folders     FolderStore
	keepFolders KeepFolderStore
	llm         *llm.Service

	mu       sync.Mutex
	force    bool
	judging  bool
	lastKept int
}

func NewKeepWorker(
	store *settings.Store,
	articles ArticleStore,
	feeds FeedStore,
	folders FolderStore,
	llmSvc *llm.Service,
) *KeepWorker {
	return &KeepWorker{
		store:    store,
		articles: articles,
		feeds:    feeds,
		folders:  folders,
		llm:      llmSvc,
	}
}

// SetKeepFolders injects the 精选 folder store used for AI routing (optional).
func (w *KeepWorker) SetKeepFolders(s KeepFolderStore) {
	if w == nil {
		return
	}
	w.keepFolders = s
}

// Enqueue merges article IDs into the pending buffer and bumps LastEnqueueAt.
func (w *KeepWorker) Enqueue(ctx context.Context, ids []string) {
	if w == nil || w.store == nil || len(ids) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p, _ := w.loadPending(ctx)
	seen := map[string]bool{}
	for _, id := range p.IDs {
		seen[id] = true
	}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		p.IDs = append(p.IDs, id)
	}
	p.LastEnqueueAt = time.Now().UTC().Format(time.RFC3339)
	if err := w.store.SetJSON(ctx, KeyKeepPending, p); err != nil {
		log.Printf("keep enqueue: %v", err)
	}
}

// NotifyForce skips the debounce on the next TryJudge.
func (w *KeepWorker) NotifyForce() {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.force = true
	w.mu.Unlock()
}

// EnqueueUnread lists unread articles, enqueues their IDs, and forces the next tick.
func (w *KeepWorker) EnqueueUnread(ctx context.Context, limit int) (int, error) {
	if w == nil || w.articles == nil {
		return 0, nil
	}
	if limit <= 0 {
		limit = keepScanUnreadCap
	}
	list, err := w.articles.List(ctx, "unread", repo.ListOpts{Limit: limit, Lite: true})
	if err != nil {
		return 0, err
	}
	ids := make([]string, 0, len(list))
	for _, a := range list {
		if strings.TrimSpace(a.ID) != "" {
			ids = append(ids, a.ID)
		}
	}
	added := w.countNewPending(ctx, ids)
	w.Enqueue(ctx, ids)
	w.NotifyForce()
	return added, nil
}

func (w *KeepWorker) countNewPending(ctx context.Context, ids []string) int {
	if w.store == nil {
		return len(ids)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p, _ := w.loadPending(ctx)
	seen := map[string]bool{}
	for _, id := range p.IDs {
		seen[id] = true
	}
	n := 0
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		n++
	}
	return n
}

// TryJudge runs one keep-judge batch if the toggle, debounce, and LLM allow it.
func (w *KeepWorker) TryJudge(ctx context.Context) (bool, error) {
	if w == nil || w.store == nil {
		return false, nil
	}
	prefs, err := w.store.LoadUIPrefs(ctx)
	if err != nil || !prefs.SmartFilterEnabled {
		return false, nil
	}
	llmCfg, err := w.store.LoadLLMConfig(ctx)
	if err != nil || !llmCfg.IsConfigured() {
		return false, nil
	}

	w.mu.Lock()
	if w.judging {
		w.mu.Unlock()
		return false, nil
	}
	p, err := w.loadPending(ctx)
	if err != nil || len(p.IDs) == 0 {
		w.mu.Unlock()
		return false, err
	}
	force := w.force
	if !force {
		if t, perr := time.Parse(time.RFC3339, p.LastEnqueueAt); perr == nil {
			if time.Since(t) < keepDebounce {
				w.mu.Unlock()
				return false, nil
			}
		}
	}
	w.judging = true
	w.force = false
	snapshot := append([]string(nil), p.IDs...)
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.judging = false
		w.mu.Unlock()
	}()

	return w.judge(ctx, snapshot, prefs)
}

func (w *KeepWorker) keepFolderRefs(ctx context.Context) []llm.KeepFolderRef {
	if w == nil || w.keepFolders == nil {
		return nil
	}
	list, err := w.keepFolders.List(ctx)
	if err != nil || len(list) == 0 {
		return nil
	}
	out := make([]llm.KeepFolderRef, 0, len(list))
	for _, f := range list {
		name := strings.TrimSpace(f.Name)
		if name == "" {
			continue
		}
		out = append(out, llm.KeepFolderRef{ID: f.ID, Name: name, Hint: strings.TrimSpace(f.Hint)})
	}
	return out
}

func (w *KeepWorker) judge(ctx context.Context, ids []string, prefs settings.UIPrefs) (bool, error) {
	if w.articles == nil {
		return false, nil
	}
	capN := keepBatchSize * keepMaxBatchesPerTick
	if len(ids) > capN {
		ids = ids[:capN]
	}

	blockKws := parseKeepBlockKeywords(prefs.BlockKeywords)
	excludeNsfw := !prefs.NsfwMode

	type candidate struct {
		art  model.Article
		feed model.Feed
	}
	var (
		skipIDs    []string
		candidates []candidate
	)
	for _, id := range ids {
		a, err := w.articles.Get(ctx, id)
		if err != nil {
			skipIDs = append(skipIDs, id)
			continue
		}
		if a.IsKept || isUntitledKeepTitle(a.Title) {
			skipIDs = append(skipIDs, id)
			continue
		}
		sum := ""
		if a.Summary != nil {
			sum = *a.Summary
		}
		if matchesKeepBlockKeywords(a.Title, sum, blockKws) {
			skipIDs = append(skipIDs, id)
			continue
		}
		if w.feeds == nil {
			skipIDs = append(skipIDs, id)
			continue
		}
		feed, err := w.feeds.Get(ctx, a.FeedID)
		if err != nil {
			skipIDs = append(skipIDs, id)
			continue
		}
		if excludeNsfw && feedIsNsfw(ctx, w.folders, feed) {
			skipIDs = append(skipIDs, id)
			continue
		}
		candidates = append(candidates, candidate{art: a, feed: feed})
	}

	if len(skipIDs) > 0 {
		w.removeConsumed(ctx, skipIDs)
	}
	if len(candidates) == 0 {
		return false, nil
	}
	if len(candidates) > keepBatchSize {
		candidates = candidates[:keepBatchSize]
	}

	if w.llm == nil {
		return false, fmt.Errorf("keep judge: llm unavailable")
	}

	locale := prefs.Locale
	if locale == "" {
		locale = "zh-CN"
	}
	items := make([]llm.KeepItem, len(candidates))
	judgedIDs := make([]string, 0, len(candidates))
	inBatch := map[string]bool{}
	for i, c := range candidates {
		sum := ""
		if c.art.Summary != nil {
			sum = *c.art.Summary
		}
		ct, ch := "", ""
		if c.art.ContentText != nil {
			ct = *c.art.ContentText
		}
		if c.art.ContentHTML != nil {
			ch = *c.art.ContentHTML
		}
		pub := ""
		if c.art.PublishedAt != nil {
			pub = *c.art.PublishedAt
		}
		if pub == "" {
			pub = c.art.FetchedAt
		}
		items[i] = llm.KeepItem{
			Index:     i + 1,
			ID:        c.art.ID,
			Title:     c.art.Title,
			Feed:      c.feed.Title,
			Published: pub,
			Summary:   sum,
			Body:      llm.PlainBody(ct, ch),
		}
		judgedIDs = append(judgedIDs, c.art.ID)
		inBatch[c.art.ID] = true
	}

	gctx, cancel := context.WithTimeout(ctx, keepJudgeTimeout)
	defer cancel()
	folderRefs := w.keepFolderRefs(ctx)
	verdicts, err := w.llm.JudgeKeepBatch(gctx, items, prefs.SmartFilterProfile, prefs.SmartFilterStrictness, locale, folderRefs)
	if err != nil {
		log.Printf("keep judge: %v", err)
		return false, err
	}

	kept := 0
	for _, v := range verdicts {
		id := strings.TrimSpace(v.ArticleID)
		if id == "" || !inBatch[id] || !v.Keep {
			continue
		}
		if err := w.articles.Keep(ctx, id, v.Reason, "filter", v.Confidence, v.Topics); err != nil {
			log.Printf("keep article %s: %v", id, err)
			continue
		}
		if fid := strings.TrimSpace(v.FolderID); fid != "" {
			if err := w.articles.SetKeepFolder(ctx, id, fid); err != nil {
				log.Printf("keep folder %s → %s: %v", id, fid, err)
			}
		}
		kept++
	}
	w.removeConsumed(ctx, judgedIDs)
	w.mu.Lock()
	w.lastKept += kept
	w.mu.Unlock()
	return true, nil
}

func (w *KeepWorker) loadPending(ctx context.Context) (keepPending, error) {
	var p keepPending
	err := w.store.GetJSON(ctx, KeyKeepPending, &p)
	if err != nil {
		return keepPending{}, nil
	}
	return p, nil
}

func (w *KeepWorker) removeConsumed(ctx context.Context, consumed []string) {
	if w.store == nil || len(consumed) == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	p, _ := w.loadPending(ctx)
	drop := map[string]bool{}
	for _, id := range consumed {
		drop[id] = true
	}
	var keep []string
	for _, id := range p.IDs {
		if !drop[id] {
			keep = append(keep, id)
		}
	}
	p.IDs = keep
	if err := w.store.SetJSON(ctx, KeyKeepPending, p); err != nil {
		log.Printf("keep pending save: %v", err)
	}
}

// Snapshot reports queued/judging keep work for the activity bar.
func (w *KeepWorker) Snapshot() (state string, pending, lastKept int) {
	if w == nil {
		return "", 0, 0
	}
	w.mu.Lock()
	judging := w.judging
	lastKept = w.lastKept
	w.mu.Unlock()
	var p keepPending
	if w.store != nil {
		p, _ = w.loadPending(context.Background())
	}
	pending = len(p.IDs)
	if judging {
		return "judging", pending, lastKept
	}
	if pending > 0 {
		return "queued", pending, lastKept
	}
	return "", 0, lastKept
}

// Count returns the total article_keeps rows (best-effort; 0 on error).
func (w *KeepWorker) Count(ctx context.Context) (int, error) {
	if w == nil || w.articles == nil {
		return 0, nil
	}
	type keeper interface {
		CountKeeps(context.Context) (int, error)
	}
	if k, ok := w.articles.(keeper); ok {
		n, err := k.CountKeeps(ctx)
		if err != nil {
			return 0, nil
		}
		return n, nil
	}
	list, err := w.articles.List(ctx, "kept", repo.ListOpts{Limit: 100000, Lite: true})
	if err != nil {
		return 0, nil
	}
	return len(list), nil
}

func isUntitledKeepTitle(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return true
	}
	if strings.EqualFold(t, "untitled") {
		return true
	}
	return t == "无标题"
}

func parseKeepBlockKeywords(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '|', '/':
			return true
		default:
			return false
		}
	})
	var out []string
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matchesKeepBlockKeywords(title, summary string, kws []string) bool {
	if len(kws) == 0 {
		return false
	}
	hay := strings.ToLower(title) + "\n" + strings.ToLower(summary)
	for _, kw := range kws {
		if kw != "" && strings.Contains(hay, kw) {
			return true
		}
	}
	return false
}
