package web_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"lrss/internal/db"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"
	"lrss/internal/web"
)

type stubAI struct {
	configured bool
	history    web.ChatHistoryResult
	send       web.ChatSendResult
	sendErr    error
	lastSend   web.ChatSendRequest
	cleared    *string
	canceled   *string
	listeners  []func(web.AIStreamEvent)
}

func (s *stubAI) Summarize(string, string) (web.AIResult, error)          { return web.AIResult{}, nil }
func (s *stubAI) Translate(string, string) (web.AIResult, error)          { return web.AIResult{}, nil }
func (s *stubAI) TranslateSelection(string, string) (web.AIResult, error) { return web.AIResult{}, nil }
func (s *stubAI) Ask(string, string, string) (web.AIResult, error)        { return web.AIResult{}, nil }
func (s *stubAI) SuggestFolders(string, string) (web.AIResult, error)     { return web.AIResult{}, nil }
func (s *stubAI) ClassifyPromo(string, string) (web.AIResult, error)      { return web.AIResult{}, nil }
func (s *stubAI) DetectContentFullness(string) (web.AIResult, error)      { return web.AIResult{}, nil }
func (s *stubAI) EnsureFullContent(string) (web.AIResult, error)          { return web.AIResult{}, nil }
func (s *stubAI) IsLLMConfigured() (bool, error)                          { return s.configured, nil }
func (s *stubAI) ClearTranslation(string) error                           { return nil }
func (s *stubAI) ApplySuggestedFolder(string, string) error               { return nil }
func (s *stubAI) ChatHistory(articleId string) (web.ChatHistoryResult, error) {
	out := s.history
	out.ArticleID = articleId
	if out.Messages == nil {
		out.Messages = []model.ChatMessage{}
	}
	return out, nil
}
func (s *stubAI) ChatClear(articleId string) error {
	s.cleared = &articleId
	return nil
}
func (s *stubAI) ChatCancel(sessionId string) error {
	s.canceled = &sessionId
	return nil
}
func (s *stubAI) ChatSend(req web.ChatSendRequest) (web.ChatSendResult, error) {
	s.lastSend = req
	ev := web.AIStreamEvent{
		Feature:   "chat",
		SessionID: s.send.SessionID,
		Text:      s.send.Markdown,
		Done:      true,
		Model:     s.send.Model,
	}
	for _, fn := range append([]func(web.AIStreamEvent){}, s.listeners...) {
		fn(ev)
	}
	return s.send, s.sendErr
}
func (s *stubAI) SubscribeStream(fn func(web.AIStreamEvent)) func() {
	s.listeners = append(s.listeners, fn)
	return func() {}
}

func testEnvAI(t *testing.T, ai web.AI) *web.Server {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	searchSvc := search.New(database.SQL, store)
	assets := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>LRSS</title>")},
	}
	return web.New(web.APIDeps{Library: lib, Store: store, Search: searchSvc, AI: ai}, assets)
}

func waitOK(t *testing.T, url, token string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", url)
}

func authJSON(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestServer_ChatAndStream(t *testing.T) {
	ai := &stubAI{
		configured: true,
		history: web.ChatHistoryResult{
			SessionID: "sess-1",
			Messages: []model.ChatMessage{
				{ID: "m1", Role: "user", Content: "hi"},
			},
		},
		send: web.ChatSendResult{
			SessionID: "sess-1",
			MessageID: "m2",
			Markdown:  "hello from library",
			Model:     "test-model",
			Citations: []model.ChatCitation{{N: 1, ArticleID: "a1", Title: "T"}},
		},
	}
	srv := testEnvAI(t, ai)
	st, err := srv.Apply(context.Background(), settings.WebAccessConfig{
		Enabled: true,
		Bind:    "localhost",
		Port:    18769,
		Token:   "chat-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })
	if !st.Running {
		t.Fatalf("status = %+v", st)
	}

	base := "http://127.0.0.1:18769"
	waitOK(t, base+"/api/meta", "chat-token")

	resp := authJSON(t, http.MethodGet, base+"/api/ai/chat?articleId=", "chat-token", nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("history status=%d body=%s", resp.StatusCode, b)
	}
	var hist web.ChatHistoryResult
	if err := json.NewDecoder(resp.Body).Decode(&hist); err != nil {
		t.Fatal(err)
	}
	if hist.SessionID != "sess-1" || len(hist.Messages) != 1 {
		t.Fatalf("history = %+v", hist)
	}

	sendBody := web.ChatSendRequest{
		Message:    "v2ex 最近的热门讨论有哪些?",
		UseLibrary: true,
		Locale:     "zh-CN",
	}
	resp2 := authJSON(t, http.MethodPost, base+"/api/ai/chat", "chat-token", sendBody)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp2.Body)
		t.Fatalf("send status=%d body=%s", resp2.StatusCode, b)
	}
	var sent web.ChatSendResult
	if err := json.NewDecoder(resp2.Body).Decode(&sent); err != nil {
		t.Fatal(err)
	}
	if sent.Markdown != "hello from library" || sent.SessionID != "sess-1" || len(sent.Citations) != 1 {
		t.Fatalf("send = %+v", sent)
	}
	if !ai.lastSend.UseLibrary || ai.lastSend.Message == "" {
		t.Fatalf("lastSend = %+v", ai.lastSend)
	}

	resp3 := authJSON(t, http.MethodPost, base+"/api/ai/chat/clear", "chat-token", map[string]string{"articleId": ""})
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("clear status=%d", resp3.StatusCode)
	}
	if ai.cleared == nil {
		t.Fatal("expected ChatClear")
	}

	resp4 := authJSON(t, http.MethodPost, base+"/api/ai/chat/cancel", "chat-token", map[string]string{"sessionId": "sess-1"})
	resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("cancel status=%d", resp4.StatusCode)
	}
	if ai.canceled == nil || *ai.canceled != "sess-1" {
		t.Fatalf("canceled = %v", ai.canceled)
	}

	ctxStream, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	reqS, err := http.NewRequestWithContext(ctxStream, http.MethodGet, base+"/api/ai/stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	reqS.Header.Set("Authorization", "Bearer chat-token")
	respS, err := http.DefaultClient.Do(reqS)
	if err != nil {
		t.Fatal(err)
	}
	defer respS.Body.Close()
	if respS.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(respS.Body)
		t.Fatalf("stream status=%d body=%s", respS.StatusCode, b)
	}
	if !strings.Contains(respS.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content-type = %q", respS.Header.Get("Content-Type"))
	}

	gotEvent := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(respS.Body)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "data: ") {
				gotEvent <- strings.TrimPrefix(line, "data: ")
				return
			}
		}
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && len(ai.listeners) == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	resp5 := authJSON(t, http.MethodPost, base+"/api/ai/chat", "chat-token", sendBody)
	resp5.Body.Close()

	select {
	case payload := <-gotEvent:
		if !strings.Contains(payload, "hello from library") {
			t.Fatalf("sse payload = %s", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not receive SSE chat event")
	}
}

func TestServer_ChatRequiresAI(t *testing.T) {
	srv, _, _ := testEnv(t)
	if _, err := srv.Apply(context.Background(), settings.WebAccessConfig{
		Enabled: true, Bind: "localhost", Port: 18770, Token: "",
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })
	waitOK(t, "http://127.0.0.1:18770/api/meta", "")

	resp, err := http.Get("http://127.0.0.1:18770/api/ai/chat")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
}
