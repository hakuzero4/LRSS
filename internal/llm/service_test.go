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
	model   string
	content string
	err     error
	calls   int
	lastSys string
	lastUser string
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
	return llm.ChatResponse{Content: s.content, Model: s.model}, nil
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
	r1, err := svc.Summarize(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if r1.Cached || !strings.Contains(r1.Markdown, "Summary") {
		t.Fatalf("r1 = %+v", r1)
	}
	if stub.calls != 1 {
		t.Fatalf("calls = %d", stub.calls)
	}
	r2, err := svc.Summarize(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	if !r2.Cached || stub.calls != 1 {
		t.Fatalf("expected cache hit: cached=%v calls=%d", r2.Cached, stub.calls)
	}
}

func TestService_Disabled(t *testing.T) {
	store, database := testStore(t)
	_ = store.SaveLLMConfig(context.Background(), settings.DefaultLLMConfig())
	svc := &llm.Service{Store: store, Cache: &llm.Cache{DB: database.SQL}}
	_, err := svc.Summarize(context.Background(), llm.ArticleInput{ID: "x", Title: "t", Body: "b"})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("err = %v", err)
	}
}

func TestService_TranslateAskDigestClassify(t *testing.T) {
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
	ask, err := svc.Ask(context.Background(), a, "What?")
	if err != nil || ask.Markdown == "" {
		t.Fatalf("ask: %+v %v", ask, err)
	}
	dig, err := svc.Digest(context.Background(), []llm.DigestItem{{Title: "One", Summary: "s"}})
	if err != nil || dig.Markdown == "" {
		t.Fatalf("digest: %+v %v", dig, err)
	}
	_, err = svc.Digest(context.Background(), nil)
	if err == nil {
		t.Fatal("empty digest should fail")
	}
	cl, err := svc.ClassifyPromo(context.Background(), a)
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
	}, nil)
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
	res, err := svc.Summarize(ctx, llm.ArticleInput{ID: "h1", Title: "T", Body: "body"})
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
	res2, err := svc.Summarize(ctx, llm.ArticleInput{ID: "h1", Title: "T", Body: "body"})
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
	_, err = svc.Summarize(ctx, llm.ArticleInput{ID: "1", Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "502") && !strings.Contains(err.Error(), "http") {
		t.Fatalf("err = %v", err)
	}
}

func TestPromptsNonEmpty(t *testing.T) {
	for _, f := range []string{
		llm.FeatureSummarize, llm.FeatureTranslate, llm.FeatureAsk,
		llm.FeatureDigest, llm.FeatureSuggest, llm.FeatureClassify,
	} {
		if strings.TrimSpace(llm.SystemPromptFor(f)) == "" {
			t.Fatalf("empty system for %s", f)
		}
	}
	_ = fmt.Sprintf("ok")
}
