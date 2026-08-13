package service

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"lrss/internal/llm"
	"lrss/internal/model"
	"lrss/internal/settings"
)

const (
	KeyBriefingPending     = "app.briefing_pending"
	briefingDebounce       = 60 * time.Second
	briefingMaxArticles     = 40
	briefingMinArticles     = 3
	briefingKeepUnstarred   = 30
	briefingGenerateTimeout = 4 * time.Minute
)

type briefingPending struct {
	IDs           []string `json:"ids"`
	LastEnqueueAt string   `json:"lastEnqueueAt"`
}

// BriefingWorker buffers new article IDs and generates one digest per session.
type BriefingWorker struct {
	store     *settings.Store
	briefings BriefingStore
	articles  ArticleStore
	feeds     FeedStore
	folders   FolderStore
	llm       *llm.Service

	mu          sync.Mutex
	forceEmpty  bool
	generating  bool
	generatingN int
}

func NewBriefingWorker(
	store *settings.Store,
	briefings BriefingStore,
	articles ArticleStore,
	feeds FeedStore,
	folders FolderStore,
	llmSvc *llm.Service,
) *BriefingWorker {
	return &BriefingWorker{
		store:     store,
		briefings: briefings,
		articles:  articles,
		feeds:     feeds,
		folders:   folders,
		llm:       llmSvc,
	}
}

// Enqueue merges article IDs into the pending buffer.
func (w *BriefingWorker) Enqueue(ctx context.Context, ids []string) {
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
	if err := w.store.SetJSON(ctx, KeyBriefingPending, p); err != nil {
		log.Printf("briefing enqueue: %v", err)
	}
}

// NotifyForceQueueEmpty allows generate without waiting for debounce.
func (w *BriefingWorker) NotifyForceQueueEmpty() {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.forceEmpty = true
	w.mu.Unlock()
}

// TryGenerate runs one briefing if the toggle, debounce, and LLM allow it.
func (w *BriefingWorker) TryGenerate(ctx context.Context) (bool, error) {
	if w == nil || w.store == nil || w.briefings == nil {
		return false, nil
	}
	prefs, err := w.store.LoadUIPrefs(ctx)
	if err != nil || !prefs.SmartBriefing {
		return false, nil
	}
	llmCfg, err := w.store.LoadLLMConfig(ctx)
	if err != nil || !llmCfg.IsConfigured() {
		return false, nil
	}

	w.mu.Lock()
	if w.generating {
		w.mu.Unlock()
		return false, nil
	}
	p, err := w.loadPending(ctx)
	if err != nil || len(p.IDs) == 0 {
		w.mu.Unlock()
		return false, err
	}
	force := w.forceEmpty
	if !force {
		if t, perr := time.Parse(time.RFC3339, p.LastEnqueueAt); perr == nil {
			if time.Since(t) < briefingDebounce {
				w.mu.Unlock()
				return false, nil
			}
		}
	}
	w.generating = true
	w.generatingN = len(p.IDs)
	w.forceEmpty = false
	snapshot := append([]string(nil), p.IDs...)
	w.mu.Unlock()

	defer func() {
		w.mu.Lock()
		w.generating = false
		w.generatingN = 0
		w.mu.Unlock()
	}()

	did, err := w.generate(ctx, "", snapshot, prefs)
	if err != nil {
		return false, err
	}
	return did, nil
}

func (w *BriefingWorker) loadPending(ctx context.Context) (briefingPending, error) {
	var p briefingPending
	err := w.store.GetJSON(ctx, KeyBriefingPending, &p)
	if err != nil {
		// missing key → empty
		return briefingPending{}, nil
	}
	return p, nil
}

func (w *BriefingWorker) generate(ctx context.Context, existingID string, ids []string, prefs settings.UIPrefs) (bool, error) {
	type row struct {
		art  model.Article
		feed model.Feed
		pub  string
	}
	var rows []row
	excludeNsfw := !prefs.NsfwMode
	for _, id := range ids {
		a, err := w.articles.Get(ctx, id)
		if err != nil {
			continue
		}
		feed, err := w.feeds.Get(ctx, a.FeedID)
		if err != nil {
			continue
		}
		if excludeNsfw && feedIsNsfw(ctx, w.folders, feed) {
			continue
		}
		pub := ""
		if a.PublishedAt != nil {
			pub = *a.PublishedAt
		}
		if pub == "" {
			pub = a.FetchedAt
		}
		rows = append(rows, row{art: a, feed: feed, pub: pub})
	}
	if len(rows) == 0 {
		w.removeConsumed(ctx, ids)
		return false, nil
	}
	if len(rows) < briefingMinArticles {
		// Too thin for a desk note — keep IDs until a later refresh adds more.
		w.mu.Lock()
		w.forceEmpty = false
		w.mu.Unlock()
		return false, nil
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].pub > rows[j].pub })
	omitted := 0
	if len(rows) > briefingMaxArticles {
		omitted = len(rows) - briefingMaxArticles
		rows = rows[:briefingMaxArticles]
	}

	locale := prefs.Locale
	if locale == "" {
		locale = "zh-CN"
	}
	items := make([]llm.BriefingItem, len(rows))
	byIndex := map[int]llm.BriefingSource{}
	for i, r := range rows {
		n := i + 1
		sum := ""
		if r.art.Summary != nil {
			sum = *r.art.Summary
		}
		if sum == "" && r.art.ContentText != nil {
			sum = *r.art.ContentText
		}
		items[i] = llm.BriefingItem{
			Index:     n,
			Title:     r.art.Title,
			Feed:      r.feed.Title,
			Published: r.pub,
			Summary:   sum,
		}
		byIndex[n] = llm.BriefingSource{ID: r.art.ID, Title: r.art.Title, FeedTitle: r.feed.Title}
	}

	sourceIDs := make([]string, 0, len(rows))
	for _, r := range rows {
		sourceIDs = append(sourceIDs, r.art.ID)
	}
	seed := model.BriefingPayload{SourceIDs: sourceIDs}
	b := &model.Briefing{
		ID:           existingID,
		Status:       "pending",
		Locale:       locale,
		ArticleCount: len(rows),
		OmittedCount: omitted,
		Payload:      seed,
	}
	if existingID == "" {
		if err := w.briefings.Insert(ctx, b); err != nil {
			return false, err
		}
	} else if err := w.briefings.UpdateGenerated(ctx, existingID, "pending", "", "", "", len(rows), omitted, seed); err != nil {
		return false, err
	}

	if w.llm == nil {
		seed.Overview = ""
		_ = w.briefings.UpdateGenerated(ctx, b.ID, "error", "", "", "llm unavailable", len(rows), omitted, seed)
		w.removeConsumed(ctx, ids)
		return true, nil
	}

	gctx, cancel := context.WithTimeout(ctx, briefingGenerateTimeout)
	defer cancel()
	res, err := w.llm.Brief(gctx, items, locale)
	if err != nil {
		_ = w.briefings.UpdateGenerated(ctx, b.ID, "error", "", "", friendlyLLMError(err), len(rows), omitted, seed)
		w.removeConsumed(ctx, ids)
		return true, nil
	}
	payload, err := llm.ParseAndMapBriefing(res.Markdown, byIndex)
	if err != nil {
		payload = seed
		_ = w.briefings.UpdateGenerated(ctx, b.ID, "error", res.Model, "", err.Error(), len(rows), omitted, payload)
		w.removeConsumed(ctx, ids)
		return true, nil
	}
	payload.SourceIDs = sourceIDs
	if err := w.briefings.UpdateGenerated(ctx, b.ID, "ready", res.Model, payload.Overview, "", len(rows), omitted, payload); err != nil {
		return false, err
	}
	if _, err := w.briefings.PruneOld(ctx, briefingKeepUnstarred); err != nil {
		log.Printf("briefing prune: %v", err)
	}
	w.removeConsumed(ctx, ids)
	return true, nil
}

func (w *BriefingWorker) removeConsumed(ctx context.Context, consumed []string) {
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
	if err := w.store.SetJSON(ctx, KeyBriefingPending, p); err != nil {
		log.Printf("briefing pending save: %v", err)
	}
}

func feedIsNsfw(ctx context.Context, folders FolderStore, feed model.Feed) bool {
	if feed.IsNsfw {
		return true
	}
	if folders == nil || feed.FolderID == nil || strings.TrimSpace(*feed.FolderID) == "" {
		return false
	}
	fo, err := folders.Get(ctx, *feed.FolderID)
	if err != nil {
		return false
	}
	return fo.IsNsfw
}

// ListBriefings / mutators used by appsvc.
func (w *BriefingWorker) List(ctx context.Context, limit int) ([]model.Briefing, error) {
	if w == nil || w.briefings == nil {
		return nil, fmt.Errorf("briefings unavailable")
	}
	return w.briefings.List(ctx, limit)
}

func (w *BriefingWorker) Get(ctx context.Context, id string) (model.Briefing, error) {
	if w == nil || w.briefings == nil {
		return model.Briefing{}, fmt.Errorf("briefings unavailable")
	}
	return w.briefings.Get(ctx, id)
}

func (w *BriefingWorker) SetRead(ctx context.Context, id string, read bool) error {
	if w == nil || w.briefings == nil {
		return fmt.Errorf("briefings unavailable")
	}
	return w.briefings.SetRead(ctx, id, read)
}

func (w *BriefingWorker) SetStarred(ctx context.Context, id string, starred bool) error {
	if w == nil || w.briefings == nil {
		return fmt.Errorf("briefings unavailable")
	}
	return w.briefings.SetStarred(ctx, id, starred)
}

func (w *BriefingWorker) Delete(ctx context.Context, id string) error {
	if w == nil || w.briefings == nil {
		return fmt.Errorf("briefings unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("briefing id is required")
	}
	return w.briefings.Delete(ctx, id)
}

// Snapshot reports queued/generating briefing work for the activity bar.
func (w *BriefingWorker) Snapshot() (state string, pending, articles int) {
	if w == nil {
		return "", 0, 0
	}
	w.mu.Lock()
	gen := w.generating
	n := w.generatingN
	w.mu.Unlock()
	var p briefingPending
	if w.store != nil {
		p, _ = w.loadPending(context.Background())
	}
	pending = len(p.IDs)
	if gen {
		if n <= 0 {
			n = pending
		}
		return "generating", pending, n
	}
	if pending > 0 {
		return "queued", pending, 0
	}
	return "", 0, 0
}

func (w *BriefingWorker) UnreadCount(ctx context.Context) (int, error) {
	if w == nil || w.briefings == nil {
		return 0, fmt.Errorf("briefings unavailable")
	}
	return w.briefings.UnreadCount(ctx)
}

// Retry re-runs generation for a failed briefing using stored source article IDs.
func (w *BriefingWorker) Retry(ctx context.Context, id string) error {
	if w == nil || w.briefings == nil {
		return fmt.Errorf("briefings unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("briefing id is required")
	}
	b, err := w.briefings.Get(ctx, id)
	if err != nil {
		return err
	}
	ids := append([]string(nil), b.Payload.SourceIDs...)
	if len(ids) == 0 {
		return fmt.Errorf("no source articles to retry")
	}
	seed := model.BriefingPayload{SourceIDs: ids}
	if err := w.briefings.UpdateGenerated(ctx, id, "pending", "", "", "", b.ArticleCount, b.OmittedCount, seed); err != nil {
		return err
	}
	prefs, err := w.store.LoadUIPrefs(ctx)
	if err != nil {
		return err
	}
	_, err = w.generate(ctx, id, ids, prefs)
	return err
}

func friendlyLLMError(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	low := strings.ToLower(s)
	if strings.Contains(low, "timeout") || strings.Contains(low, "deadline exceeded") {
		return "模型响应超时（本地模型汇总多篇文章可能需要几分钟）。请点重试，或检查 设置 → 搜索/AI 里的接口是否可用。"
	}
	return s
}
