package appsvc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"lrss/internal/appsvc"
	"lrss/internal/db"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/service"
	"lrss/internal/settings"
)

func TestAIService_ChatSendPersistsTurns(t *testing.T) {
	const body = `<?xml version="1.0"?><rss version="2.0"><channel>
<title>Chat Feed</title><link>https://ex.test/</link>
<item><title>Widget 2.0</title><link>https://ex.test/1</link><guid>g-chat</guid>
<description>Widget 2.0 launched with a 30 percent smaller binary.</description></item>
</channel></rss>`
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(feedSrv.Close)

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "体积小了 30%。[1]"}},
			},
		})
	}))
	t.Cleanup(llmSrv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	if err := store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  llmSrv.URL + "/v1",
		Model:    "m",
		APIKey:   "k",
	}); err != nil {
		t.Fatal(err)
	}
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	feed, err := lib.AddFeed(ctx, feedSrv.URL+"/rss", nil)
	if err != nil {
		t.Fatal(err)
	}
	arts, err := lib.ListArticles(ctx, "feed:"+feed.ID, 10, 0, false)
	if err != nil || len(arts) == 0 {
		t.Fatalf("arts: %v %#v", err, arts)
	}

	ai := appsvc.NewAI(store, lib, database.SQL)
	hist, err := ai.ChatHistory(arts[0].ID)
	if err != nil || len(hist.Messages) != 0 {
		t.Fatalf("empty hist: %+v %v", hist, err)
	}

	res, err := ai.ChatSend(appsvc.ChatSendRequest{
		ArticleID: arts[0].ID,
		Message:   "变了什么？",
		Locale:    "zh-CN",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Markdown, "30") || res.SessionID == "" {
		t.Fatalf("res = %+v", res)
	}
	if len(res.Citations) != 1 || res.Citations[0].ArticleID != arts[0].ID {
		t.Fatalf("cites = %#v", res.Citations)
	}

	hist, err = ai.ChatHistory(arts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if hist.SessionID != res.SessionID || len(hist.Messages) != 2 {
		t.Fatalf("hist = %+v", hist)
	}
	if hist.Messages[0].Role != "user" || hist.Messages[1].Role != "assistant" {
		t.Fatalf("roles = %s %s", hist.Messages[0].Role, hist.Messages[1].Role)
	}

	if err := ai.ChatClear(arts[0].ID); err != nil {
		t.Fatal(err)
	}
	hist, err = ai.ChatHistory(arts[0].ID)
	if err != nil || len(hist.Messages) != 0 {
		t.Fatalf("cleared: %+v %v", hist, err)
	}
}

func TestAIService_ChatSendAttachAndLibrary(t *testing.T) {
	const body = `<?xml version="1.0"?><rss version="2.0"><channel>
<title>Lib Feed</title><link>https://lib.test/</link>
<item><title>Alpha Launch</title><link>https://lib.test/a</link><guid>ga</guid>
<description>Alpha shipped a new API today.</description></item>
<item><title>Beta Recall</title><link>https://lib.test/b</link><guid>gb</guid>
<description>Beta recalled 12 thousand units.</description></item>
</channel></rss>`
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(feedSrv.Close)

	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "两件事：Alpha 发了 API [1]，Beta 召回 [2]。"}},
			},
		})
	}))
	t.Cleanup(llmSrv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t-lib.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	if err := store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  llmSrv.URL + "/v1",
		Model:    "m",
		APIKey:   "k",
	}); err != nil {
		t.Fatal(err)
	}
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	feed, err := lib.AddFeed(ctx, feedSrv.URL+"/rss", nil)
	if err != nil {
		t.Fatal(err)
	}
	arts, err := lib.ListArticles(ctx, "feed:"+feed.ID, 10, 0, false)
	if err != nil || len(arts) < 2 {
		t.Fatalf("arts: %v n=%d", err, len(arts))
	}

	ai := appsvc.NewAI(store, lib, database.SQL)
	res, err := ai.ChatSend(appsvc.ChatSendRequest{
		ArticleID:  arts[0].ID,
		AttachIDs:  []string{arts[1].ID},
		Message:    "这两篇分别说了什么？",
		Locale:     "zh-CN",
		UseLibrary: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Citations) < 2 {
		t.Fatalf("want 2 cites, got %#v md=%q", res.Citations, res.Markdown)
	}
	ids := map[string]bool{}
	for _, c := range res.Citations {
		ids[c.ArticleID] = true
	}
	if !ids[arts[0].ID] || !ids[arts[1].ID] {
		t.Fatalf("cites missing articles: %#v", res.Citations)
	}

	libRes, err := ai.ChatSend(appsvc.ChatSendRequest{
		Message:      "今天该先看什么",
		CollectionID: "unread",
		Locale:       "zh-CN",
		UseLibrary:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if libRes.SessionID == "" || libRes.Markdown == "" {
		t.Fatalf("library chat: %+v", libRes)
	}
	if len(libRes.Citations) == 0 {
		t.Fatalf("library cites empty: %#v", libRes)
	}

	// Switching the "current" article must not fork another conversation.
	if _, err := ai.ChatSend(appsvc.ChatSendRequest{
		ArticleID: arts[1].ID,
		Message:   "还是刚才那个对话吗？",
		Locale:    "zh-CN",
	}); err != nil {
		t.Fatal(err)
	}
	global, err := ai.ChatHistory("")
	if err != nil {
		t.Fatal(err)
	}
	if len(global.Messages) < 4 {
		t.Fatalf("global session should keep turns across articles, got %d", len(global.Messages))
	}
}

func TestAIService_ChatSendPrefersNamedFeedOverUnread(t *testing.T) {
	var v2exItems, otherItems strings.Builder
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&v2exItems, `<item><title>V2EX Topic %d</title><link>https://v2ex.com/t/%d</link><guid>v-%d</guid><description>thread %d</description></item>`, i, i, i, i)
		fmt.Fprintf(&otherItems, `<item><title>Wire Story %d</title><link>https://wire.test/%d</link><guid>w-%d</guid><description>wire %d</description></item>`, i, i, i, i)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v2ex", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel>
<title>V2EX</title><link>https://www.v2ex.com/</link>%s</channel></rss>`, v2exItems.String())
	})
	mux.HandleFunc("/wire", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><rss version="2.0"><channel>
<title>Wire Desk</title><link>https://wire.test/</link>%s</channel></rss>`, otherItems.String())
	})
	feedSrv := httptest.NewServer(mux)
	t.Cleanup(feedSrv.Close)

	var captured strings.Builder
	llmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for _, m := range body.Messages {
			captured.WriteString(m.Content)
			captured.WriteByte('\n')
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "V2EX Topic 1 [1]"}},
			},
		})
	}))
	t.Cleanup(llmSrv.Close)

	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t-named-feed.db")
	database, err := db.Open(ctx, db.Options{Path: path})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	store := settings.NewStore(database.SQL)
	if err := store.SaveLLMConfig(ctx, settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  llmSrv.URL + "/v1",
		Model:    "m",
		APIKey:   "k",
	}); err != nil {
		t.Fatal(err)
	}
	repos := repo.New(database.SQL)
	lib := service.NewLibraryFromRepos(repos, &rss.Client{})
	if _, err := lib.AddFeed(ctx, feedSrv.URL+"/wire", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.AddFeed(ctx, feedSrv.URL+"/v2ex", nil); err != nil {
		t.Fatal(err)
	}

	ai := appsvc.NewAI(store, lib, database.SQL)
	res, err := ai.ChatSend(appsvc.ChatSendRequest{
		Message: "v2ex 最近的热门讨论有哪些?",
		Locale:  "zh-CN",
		// No article, no attach, UseLibrary left false — global assistant should still search.
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Markdown == "" {
		t.Fatalf("empty reply: %+v", res)
	}
	ctxText := captured.String()
	v2exN := strings.Count(ctxText, "V2EX Topic")
	wireN := strings.Count(ctxText, "Wire Story")
	if v2exN < 6 {
		t.Fatalf("expected most V2EX threads in context, v2ex=%d wire=%d\n%s", v2exN, wireN, ctxText)
	}
	if wireN > v2exN {
		t.Fatalf("unread wire pile crowded out V2EX: v2ex=%d wire=%d", v2exN, wireN)
	}
}
