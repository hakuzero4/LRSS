package llm_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"lrss/internal/db"
	"lrss/internal/llm"
	"lrss/internal/settings"
)

type stubChat struct {
	model        string
	content      string
	finishReason string
	err          error
	calls        int
	lastSys      string
	lastUser     string
}

func (s *stubChat) Chat(ctx context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
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
	return llm.ChatResponse{
		Content:      s.content,
		Model:        s.model,
		FinishReason: s.finishReason,
	}, nil
}
func (s *stubChat) ModelName() string { return s.model }

func testStore(t *testing.T) (*settings.Store, *db.DB) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	cfg := settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  "https://example.test/v1",
		APIKey:   "k",
		Model:    "test-model",
	}
	if err := store.SaveLLMConfig(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	return store, database
}

func TestService_Summarize_CacheHit(t *testing.T) {
	store, database := testStore(t)
	stub := &stubChat{model: "test-model", content: "## Summary\n- point"}
	svc := &llm.Service{
		Store: store,
		Cache: &llm.Cache{DB: database.SQL},
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	a := llm.ArticleInput{ID: "a1", Title: "T", Body: "Long body about golang testing."}
	r1, err := svc.Summarize(context.Background(), a, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if r1.Cached || !strings.Contains(r1.Markdown, "Summary") {
		t.Fatalf("r1 = %+v", r1)
	}
	if stub.calls != 1 {
		t.Fatalf("calls = %d", stub.calls)
	}
	r2, err := svc.Summarize(context.Background(), a, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Cached || stub.calls != 1 {
		t.Fatalf("expected cache hit: cached=%v calls=%d", r2.Cached, stub.calls)
	}
	// Different UI locale → different cache key → second call.
	_, err = svc.Summarize(context.Background(), a, "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 2 {
		t.Fatalf("locale should miss cache: calls=%d", stub.calls)
	}
	if !strings.Contains(stub.lastUser, "简体中文") && !strings.Contains(stub.lastSys, "简体中文") {
		t.Fatalf("zh prompts missing Chinese instruction: sys=%q user=%q", stub.lastSys, stub.lastUser)
	}
}

func TestService_Disabled(t *testing.T) {
	store, database := testStore(t)
	_ = store.SaveLLMConfig(context.Background(), settings.DefaultLLMConfig())
	svc := &llm.Service{Store: store, Cache: &llm.Cache{DB: database.SQL}}
	_, err := svc.Summarize(context.Background(), llm.ArticleInput{ID: "x", Title: "t", Body: "b"}, "en")
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err = %v", err)
	}
}

func TestService_TranslateAskClassify(t *testing.T) {
	store, database := testStore(t)
	stub := &stubChat{model: "test-model", content: "**Verdict:** organic\n\nAll good."}
	svc := &llm.Service{
		Store: store,
		Cache: &llm.Cache{DB: database.SQL},
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) { return stub, nil },
	}
	a := llm.ArticleInput{ID: "a2", Title: "Hello", Body: "World content for AI."}

	tr, err := svc.Translate(context.Background(), a, "en")
	if err != nil || tr.Markdown == "" {
		t.Fatalf("translate: %+v %v", tr, err)
	}
	ask, err := svc.Ask(context.Background(), a, "What?", "en-US")
	if err != nil || ask.Markdown == "" {
		t.Fatalf("ask: %+v %v", ask, err)
	}
	cl, err := svc.ClassifyPromo(context.Background(), a, "en")
	if err != nil {
		t.Fatal(err)
	}
	if cl.Verdict != "organic" {
		t.Fatalf("verdict = %q md=%q", cl.Verdict, cl.Markdown)
	}
}

func TestService_Suggest_LocalWhenLLMOff(t *testing.T) {
	store, database := testStore(t)
	_ = store.SaveLLMConfig(context.Background(), settings.DefaultLLMConfig())
	svc := &llm.Service{Store: store, Cache: &llm.Cache{DB: database.SQL}}
	res, err := svc.Suggest(context.Background(), llm.ArticleInput{
		ID: "a", Title: "Kubernetes guide #k8s", Summary: "cluster tips",
	}, nil, "en")
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "local" || !strings.Contains(res.Markdown, "Tags") {
		t.Fatalf("%+v", res)
	}
}

func TestService_HTTPPath_Summarize(t *testing.T) {
	// Drive real NewClient against httptest (shipped path).
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "http-model",
			"choices": []map[string]any{
				{"message": map[string]string{"content": "## From HTTP\n- ok"}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "h.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	if err := store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  srv.URL + "/v1",
		Model:    "http-model",
		APIKey:   "k",
	}); err != nil {
		t.Fatal(err)
	}
	svc := &llm.Service{Store: store, Cache: &llm.Cache{DB: database.SQL}}
	res, err := svc.Summarize(ctx, llm.ArticleInput{ID: "h1", Title: "T", Body: "body"}, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Markdown, "From HTTP") {
		t.Fatalf("md = %q", res.Markdown)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
	// cache
	res2, err := svc.Summarize(ctx, llm.ArticleInput{ID: "h1", Title: "T", Body: "body"}, "en-US")
	if err != nil || !res2.Cached || calls != 1 {
		t.Fatalf("cache: %+v calls=%d err=%v", res2, calls, err)
	}
}

func TestService_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	t.Cleanup(srv.Close)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "e.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	store := settings.NewStore(database.SQL)
	_ = store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  srv.URL + "/v1",
		Model:    "m",
	})
	svc := &llm.Service{Store: store, Cache: &llm.Cache{DB: database.SQL}}
	_, err = svc.Summarize(ctx, llm.ArticleInput{ID: "1", Title: "t", Body: "b"}, "en")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "502") && !strings.Contains(err.Error(), "http") {
		t.Fatalf("err = %v", err)
	}
}

func TestNeedsFullContentFetch(t *testing.T) {
	if !llm.NeedsFullContentFetch("T", "", "", "https://x") {
		t.Fatal("empty body + url should fetch")
	}
	if llm.NeedsFullContentFetch("T", "", "body", "") {
		t.Fatal("no url should not fetch")
	}
	if !llm.NeedsFullContentFetch("T", "sum", "Short. Read more…", "https://x") {
		t.Fatal("truncation cue should fetch")
	}
	long := strings.Repeat("Complete paragraph about the topic here. ", 40)
	if llm.NeedsFullContentFetch("T", "sum", long, "https://x") {
		t.Fatal("long body should not auto-fetch")
	}
}

func TestService_DetectContentFullness(t *testing.T) {
	store, database := testStore(t)
	stub := &stubChat{model: "test-model", content: "VERDICT: partial\nmodel would say partial wrongly"}
	svc := &llm.Service{
		Store: store,
		Cache: &llm.Cache{DB: database.SQL},
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	ctx := context.Background()
	// empty body → local partial, no chat call
	r0, err := svc.DetectContentFullness(ctx, llm.ArticleInput{ID: "a0", Body: "", URL: "https://x"})
	if err != nil || r0.Verdict != llm.FullnessPartial || stub.calls != 0 {
		t.Fatalf("empty: %+v calls=%d err=%v", r0, stub.calls, err)
	}
	// truncation cue → local partial, no chat
	r1, err := svc.DetectContentFullness(ctx, llm.ArticleInput{
		ID: "a1", Title: "T", Body: "Short teaser. Read more…", URL: "https://x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r1.Verdict != llm.FullnessPartial || stub.calls != 0 {
		t.Fatalf("truncation local: %+v calls=%d", r1, stub.calls)
	}
	// long clean body → local full, no chat
	long := strings.Repeat("This is a complete paragraph about the topic. ", 80)
	r2, err := svc.DetectContentFullness(ctx, llm.ArticleInput{
		ID: "a2", Title: "Long", Body: long, URL: "https://x",
	})
	if err != nil || r2.Verdict != llm.FullnessFull || stub.calls != 0 {
		t.Fatalf("long full: %+v calls=%d err=%v", r2, stub.calls, err)
	}
	// Full-looking medium article without truncation → full (no model; no false partial)
	medium := strings.Repeat("Another sentence about the product launch. ", 25)
	r3, err := svc.DetectContentFullness(ctx, llm.ArticleInput{
		ID: "a3", Title: "Post", Body: medium, Summary: "Launch notes.", URL: "https://x",
	})
	if err != nil || r3.Verdict != llm.FullnessFull || stub.calls != 0 {
		t.Fatalf("medium full: %+v calls=%d err=%v", r3, stub.calls, err)
	}
	// Ambiguous short, no cue → conservative full (do not auto-fetch)
	r4, err := svc.DetectContentFullness(ctx, llm.ArticleInput{
		ID: "a4", Title: "Amb", Body: "Only a short lead without a hard cue.", URL: "https://x",
	})
	if err != nil || r4.Verdict != llm.FullnessFull || stub.calls != 0 {
		t.Fatalf("conservative full: %+v calls=%d err=%v", r4, stub.calls, err)
	}
}

func TestService_SelectTranslate(t *testing.T) {
	store, database := testStore(t)
	stub := &stubChat{model: "test-model", content: "  你好世界  "}
	svc := &llm.Service{
		Store: store,
		Cache: &llm.Cache{DB: database.SQL},
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	ctx := context.Background()
	r, err := svc.SelectTranslate(ctx, "hello world", "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if r.Feature != llm.FeatureSelectTranslate || r.Markdown != "你好世界" {
		t.Fatalf("got %+v", r)
	}
	if !strings.Contains(stub.lastUser, "hello world") {
		t.Fatalf("user prompt missing selection: %s", stub.lastUser)
	}
	// cache hit
	r2, err := svc.SelectTranslate(ctx, "hello world", "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Cached {
		t.Fatal("expected cache hit")
	}
	if stub.calls != 1 {
		t.Fatalf("calls = %d want 1", stub.calls)
	}
}

func TestPromptsNonEmpty(t *testing.T) {
	for _, f := range []string{
		llm.FeatureSummarize, llm.FeatureTranslate, llm.FeatureSelectTranslate, llm.FeatureAsk,
		llm.FeatureSuggest, llm.FeatureClassify,
	} {
		if strings.TrimSpace(llm.SystemPromptFor(f, "zh-CN")) == "" {
			t.Fatalf("empty system for %s", f)
		}
	}
	if llm.NormalizeUILocale("zh-CN") != "zh" || llm.NormalizeUILocale("en-US") != "en" {
		t.Fatal("locale normalize")
	}
	if !strings.Contains(llm.UserPromptSummarize("body", "zh-CN"), "简体中文") {
		t.Fatal("zh summarize prompt")
	}
	_ = fmt.Sprintf("ok")
}

func TestService_Summarize_TruncatedNotCached(t *testing.T) {
	store, database := testStore(t)
	stub := &stubChat{
		model:        "test-model",
		content:      "## Summary\n- partial bullet that hit the limit",
		finishReason: "length",
	}
	svc := &llm.Service{
		Store: store,
		Cache: &llm.Cache{DB: database.SQL},
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	a := llm.ArticleInput{ID: "trunc1", Title: "T", Body: "Long enough body for summarize."}
	ctx := context.Background()
	_, err := svc.Summarize(ctx, a, "en-US")
	if err == nil {
		t.Fatal("expected truncated error")
	}
	if !strings.Contains(err.Error(), "truncated") && !strings.Contains(err.Error(), "length") {
		t.Fatalf("err = %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("calls = %d", stub.calls)
	}
	// Subsequent get must miss cache (nothing stored) and call the model again.
	stub.finishReason = "stop"
	stub.content = "## Summary\n- complete now"
	r2, err := svc.Summarize(ctx, a, "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Cached {
		t.Fatal("truncated result must not have been cached")
	}
	if stub.calls != 2 {
		t.Fatalf("expected second model call after miss, calls=%d", stub.calls)
	}
	if !strings.Contains(r2.Markdown, "complete") {
		t.Fatalf("md = %q", r2.Markdown)
	}
}

func TestService_Translate_TruncatedNotCached(t *testing.T) {
	store, database := testStore(t)
	partial := "<<o>> Hello world ends abruptly mid pair\n<<t>> 你好"
	stub := &stubChat{
		model:        "test-model",
		content:      partial,
		finishReason: "length",
	}
	svc := &llm.Service{
		Store: store,
		Cache: &llm.Cache{DB: database.SQL},
		NewChatter: func(cfg settings.LLMConfig) (llm.Chatter, error) {
			return stub, nil
		},
	}
	a := llm.ArticleInput{ID: "tr-trunc", Title: "Hi", Body: "Hello world article body."}
	ctx := context.Background()
	res, err := svc.Translate(ctx, a, "zh-CN")
	if err == nil {
		t.Fatalf("expected error, got success md=%q", res.Markdown)
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Fatalf("err = %v", err)
	}
	// Cache must not serve the truncated bilingual blob.
	stub.finishReason = "stop"
	stub.content = "<<o>> Hello world article body.\n<<t>> 你好世界文章正文。"
	r2, err := svc.Translate(ctx, a, "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Cached {
		t.Fatal("truncated translate must not be cached")
	}
	if stub.calls != 2 {
		t.Fatalf("calls = %d want 2", stub.calls)
	}
}

func TestIsIncompleteCompletion(t *testing.T) {
	if !llm.IsIncompleteCompletion("length") || !llm.IsIncompleteCompletion("LENGTH") {
		t.Fatal("length")
	}
	if !llm.IsIncompleteCompletion("max_tokens") {
		t.Fatal("max_tokens")
	}
	if llm.IsIncompleteCompletion("stop") || llm.IsIncompleteCompletion("") {
		t.Fatal("stop/empty should be complete")
	}
	if llm.RejectIfIncomplete("length") == nil {
		t.Fatal("RejectIfIncomplete length")
	}
	if llm.RejectIfIncomplete("stop") != nil {
		t.Fatal("RejectIfIncomplete stop")
	}
}
