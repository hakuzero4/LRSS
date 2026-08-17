package service_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lrss/internal/db"
	"lrss/internal/llm"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/service"
	"lrss/internal/settings"
)

type keepStubChat struct {
	model    string
	content  string
	err      error
	calls    int
	lastSys  string
	lastUser string
}

func (s *keepStubChat) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	s.calls++
	for _, m := range req.Messages {
		if m.Role == "system" {
			s.lastSys = m.Content
		}
		if m.Role == "user" {
			s.lastUser = m.Content
		}
	}
	if s.err != nil {
		return llm.ChatResponse{}, s.err
	}
	return llm.ChatResponse{Content: s.content, Model: s.model}, nil
}

func (s *keepStubChat) ModelName() string { return s.model }

func openKeepEnv(t *testing.T) (context.Context, *settings.Store, *repo.Repos) {
	t.Helper()
	ctx := context.Background()
	database, err := db.Open(ctx, db.Options{Path: filepath.Join(t.TempDir(), "keep.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return ctx, settings.NewStore(database.SQL), repo.New(database.SQL)
}

func insertKeepArticle(t *testing.T, ctx context.Context, repos *repo.Repos, title, summary, body string) model.Article {
	t.Helper()
	feed := &model.Feed{Title: "KeepFeed", FeedURL: "https://ex.test/keep-" + title + ".xml"}
	if err := repos.Feeds.Insert(ctx, feed); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	sum, txt := summary, body
	if _, err := repos.Articles.UpsertFromParsed(ctx, feed.ID, []repo.ParsedItem{
		{
			GUID:        "g-" + title,
			URL:         "https://ex.test/" + title,
			Title:       title,
			Summary:     &sum,
			ContentText: &txt,
			PublishedAt: &now,
		},
	}); err != nil {
		t.Fatal(err)
	}
	arts, err := repos.Articles.List(ctx, "all", repo.ListOpts{Limit: 5, Query: title})
	if err != nil || len(arts) == 0 {
		t.Fatalf("seed list: %v n=%d", err, len(arts))
	}
	for _, a := range arts {
		if a.Title == title {
			return a
		}
	}
	t.Fatalf("seed article %q not found", title)
	return model.Article{}
}

func enableKeepFilter(t *testing.T, ctx context.Context, store *settings.Store, block string) {
	t.Helper()
	prefs := settings.DefaultUIPrefs()
	prefs.SmartFilterEnabled = true
	prefs.BlockKeywords = block
	if err := store.SaveUIPrefs(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  "http://127.0.0.1:9",
		Model:    "x",
	}); err != nil {
		t.Fatal(err)
	}
}

func keepVerdictJSON(articleID string) string {
	// articleID is unused: JudgeKeepBatch maps n → KeepItem.ID.
	_ = articleID
	return `{"items":[{"n":1,"keep":true,"confidence":0.95,"reason":"high signal original reporting","topics":["tech"]}]}`
}

func TestKeepWorker_SkipWhenOffOrNoLLM(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, nil)

	did, err := w.TryJudge(ctx)
	if err != nil || did {
		t.Fatalf("off+empty did=%v err=%v", did, err)
	}

	prefs, err := store.LoadUIPrefs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	prefs.SmartFilterEnabled = true
	if err := store.SaveUIPrefs(ctx, prefs); err != nil {
		t.Fatal(err)
	}
	w.Enqueue(ctx, []string{"a1"})
	did, err = w.TryJudge(ctx)
	if err != nil || did {
		t.Fatalf("no llm config should skip did=%v err=%v", did, err)
	}
}

func TestKeepWorker_Debounce(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "")
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, &llm.Service{Store: store})
	w.Enqueue(ctx, []string{"missing"})
	did, err := w.TryJudge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if did {
		t.Fatal("fresh enqueue should debounce")
	}
}

func TestKeepWorker_NotifyForceKeepsArticle(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "")
	art := insertKeepArticle(t, ctx, repos, "Serious Analysis", "deep dive", "body of a real story")
	stub := &keepStubChat{model: "x", content: keepVerdictJSON(art.ID)}
	llmSvc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	w.Enqueue(ctx, []string{art.ID})
	w.NotifyForce()
	did, err := w.TryJudge(ctx)
	if err != nil {
		t.Fatalf("TryJudge: %v", err)
	}
	if !did {
		t.Fatal("expected judge to run")
	}
	kept, err := repos.Articles.IsKept(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !kept {
		t.Fatal("article should be kept after keep=true verdict")
	}
	got, err := repos.Articles.Get(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepSource != "filter" || got.KeepConfidence < 0.7 {
		t.Fatalf("keep fields = isKept=%v src=%q conf=%v reason=%q", got.IsKept, got.KeepSource, got.KeepConfidence, got.KeepReason)
	}
}

func TestKeepWorker_BlockKeywordConsumed(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "ads,促销")
	blocked := insertKeepArticle(t, ctx, repos, "Buy cheap ads now", "促销 spam", "ad body")
	ok := insertKeepArticle(t, ctx, repos, "Serious Analysis", "deep dive", "body of a real story")
	stub := &keepStubChat{model: "x", content: keepVerdictJSON(ok.ID)}
	llmSvc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	w.Enqueue(ctx, []string{blocked.ID, ok.ID})
	w.NotifyForce()
	if _, err := w.TryJudge(ctx); err != nil {
		t.Fatalf("TryJudge: %v", err)
	}
	blockedKept, err := repos.Articles.IsKept(ctx, blocked.ID)
	if err != nil {
		t.Fatal(err)
	}
	if blockedKept {
		t.Fatal("block-keyword article must not be kept")
	}
	if stub.calls != 1 {
		t.Fatalf("chatter calls=%d want 1 (only the non-blocked article)", stub.calls)
	}
	if strings.Contains(strings.ToLower(stub.lastUser), "buy cheap ads") {
		t.Fatal("blocked title should not be sent to chatter")
	}
	okKept, err := repos.Articles.IsKept(ctx, ok.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !okKept {
		t.Fatal("non-blocked article should be kept when chatter returns keep=true")
	}
}

func TestKeepWorker_BlockKeywordOnly_NoChatter(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "ads")
	blocked := insertKeepArticle(t, ctx, repos, "Buy cheap ads now", "summary", "body")
	stub := &keepStubChat{model: "x", content: `[{"n":1,"keep":true,"confidence":0.99,"reason":"x"}]`}
	llmSvc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	w.Enqueue(ctx, []string{blocked.ID})
	w.NotifyForce()
	if _, err := w.TryJudge(ctx); err != nil {
		t.Fatal(err)
	}
	if stub.calls != 0 {
		t.Fatalf("chatter calls=%d want 0", stub.calls)
	}
	kept, err := repos.Articles.IsKept(ctx, blocked.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept {
		t.Fatal("blocked article kept")
	}
	_, pending, _ := w.Snapshot()
	if pending != 0 {
		t.Fatalf("blocked id should be consumed, pending=%d", pending)
	}
}

func TestKeep_ListUnkeepCountSmart(t *testing.T) {
	ctx, _, repos := openKeepEnv(t)
	art := insertKeepArticle(t, ctx, repos, "Kept Story", "sum", "body")
	_ = insertKeepArticle(t, ctx, repos, "Other Story", "sum", "body")

	if err := repos.Articles.Keep(ctx, art.ID, "manual pick", "manual", 1, []string{"news"}); err != nil {
		t.Fatal(err)
	}
	list, err := repos.Articles.List(ctx, "kept", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != art.ID || !list[0].IsKept {
		t.Fatalf("List kept = %+v", list)
	}
	counts, err := repos.Articles.CountSmart(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Kept != 1 {
		t.Fatalf("CountSmart.Kept=%d want 1", counts.Kept)
	}

	if err := repos.Articles.Unkeep(ctx, art.ID); err != nil {
		t.Fatal(err)
	}
	list, err = repos.Articles.List(ctx, "kept", repo.ListOpts{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("after Unkeep: %+v", list)
	}
	counts, err = repos.Articles.CountSmart(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if counts.Kept != 0 {
		t.Fatalf("CountSmart.Kept after unkeep=%d", counts.Kept)
	}
}

func TestKeepPendingJSONRoundTrip(t *testing.T) {
	ctx, store, _ := openKeepEnv(t)
	type p struct {
		IDs           []string `json:"ids"`
		LastEnqueueAt string   `json:"lastEnqueueAt"`
	}
	want := p{IDs: []string{"a", "b"}, LastEnqueueAt: time.Now().UTC().Format(time.RFC3339)}
	if err := store.SetJSON(ctx, service.KeyKeepPending, want); err != nil {
		t.Fatal(err)
	}
	var got p
	if err := store.GetJSON(ctx, service.KeyKeepPending, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.IDs) != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestKeepWorker_EnqueueUnread(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	_ = insertKeepArticle(t, ctx, repos, "Unread One", "s", "b")
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, nil)
	n, err := w.EnqueueUnread(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if n < 1 {
		t.Fatalf("added=%d", n)
	}
	state, pending, _ := w.Snapshot()
	if pending < 1 || state != "queued" {
		t.Fatalf("snapshot state=%q pending=%d", state, pending)
	}
	n2, err := w.EnqueueUnread(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if n2 != 0 {
		t.Fatalf("second enqueue added=%d want 0", n2)
	}
}

func TestKeepWorker_Count(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	art := insertKeepArticle(t, ctx, repos, "Counted", "s", "b")
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, nil)
	n, err := w.Count(ctx)
	if err != nil || n != 0 {
		t.Fatalf("empty count=%d err=%v", n, err)
	}
	if err := repos.Articles.Keep(ctx, art.ID, "x", "manual", 1, nil); err != nil {
		t.Fatal(err)
	}
	n, err = w.Count(ctx)
	if err != nil || n != 1 {
		t.Fatalf("count=%d err=%v", n, err)
	}
}

func TestKeepWorker_RoutesToExistingFolder(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "")
	art := insertKeepArticle(t, ctx, repos, "Rust 1.80", "release", "edition and std changes")
	folder, err := repos.KeepFolders.Create(ctx, "Rust", "")
	if err != nil {
		t.Fatal(err)
	}
	stub := &keepStubChat{
		model:   "x",
		content: `{"items":[{"n":1,"keep":true,"confidence":0.95,"reason":"toolchain release","folder":"rust"}]}`,
	}
	llmSvc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	w.SetKeepFolders(repos.KeepFolders)
	w.Enqueue(ctx, []string{art.ID})
	w.NotifyForce()
	if _, err := w.TryJudge(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := repos.Articles.Get(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepFolderID != folder.ID {
		t.Fatalf("routed keep = kept=%v folder=%q want %q", got.IsKept, got.KeepFolderID, folder.ID)
	}
	inFolder, err := repos.Articles.List(ctx, "kept:"+folder.ID, repo.ListOpts{Limit: 10})
	if err != nil || len(inFolder) != 1 || inFolder[0].ID != art.ID {
		t.Fatalf("list kept:folder = %v err=%v", inFolder, err)
	}
}

func TestKeepWorker_UnknownFolderGoesToRoot(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "")
	art := insertKeepArticle(t, ctx, repos, "Unknown Topic", "note", "body")
	if _, err := repos.KeepFolders.Create(ctx, "Rust", ""); err != nil {
		t.Fatal(err)
	}
	stub := &keepStubChat{
		model:   "x",
		content: `{"items":[{"n":1,"keep":true,"confidence":0.95,"reason":"worth reading","folder":"NoSuchFolder"}]}`,
	}
	llmSvc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	w.SetKeepFolders(repos.KeepFolders)
	w.Enqueue(ctx, []string{art.ID})
	w.NotifyForce()
	if _, err := w.TryJudge(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := repos.Articles.Get(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepFolderID != "" {
		t.Fatalf("unknown folder should stay at root: kept=%v folder=%q", got.IsKept, got.KeepFolderID)
	}
}

func TestKeepWorker_KeepFalseDoesNotFile(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "")
	art := insertKeepArticle(t, ctx, repos, "Soft Promo", "buy now", "affiliate")
	if _, err := repos.KeepFolders.Create(ctx, "Rust", ""); err != nil {
		t.Fatal(err)
	}
	stub := &keepStubChat{
		model:   "x",
		content: `{"items":[{"n":1,"keep":false,"confidence":0.99,"reason":"ad","folder":"Rust"}]}`,
	}
	llmSvc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	w.SetKeepFolders(repos.KeepFolders)
	w.Enqueue(ctx, []string{art.ID})
	w.NotifyForce()
	if _, err := w.TryJudge(ctx); err != nil {
		t.Fatal(err)
	}
	kept, err := repos.Articles.IsKept(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept {
		t.Fatal("keep=false must not write article_keeps")
	}
}

func TestKeepWorker_NoFoldersGoesToRoot(t *testing.T) {
	ctx, store, repos := openKeepEnv(t)
	enableKeepFilter(t, ctx, store, "")
	art := insertKeepArticle(t, ctx, repos, "Root Only", "note", "body")
	stub := &keepStubChat{model: "x", content: keepVerdictJSON(art.ID)}
	llmSvc := &llm.Service{
		Store: store,
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	w := service.NewKeepWorker(store, repos.Articles, repos.Feeds, repos.Folders, llmSvc)
	w.SetKeepFolders(repos.KeepFolders)
	w.Enqueue(ctx, []string{art.ID})
	w.NotifyForce()
	if _, err := w.TryJudge(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := repos.Articles.Get(ctx, art.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsKept || got.KeepFolderID != "" {
		t.Fatalf("no subfolders → root: kept=%v folder=%q", got.IsKept, got.KeepFolderID)
	}
}
