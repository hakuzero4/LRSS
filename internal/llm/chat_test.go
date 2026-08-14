package llm_test

import (
	"strings"
	"testing"

	"lrss/internal/llm"
)

func TestBuildArticleChatMessages_IncludesContextHistoryAndQuestion(t *testing.T) {
	in := llm.ArticleChatInput{
		Excerpts: []llm.ChatExcerpt{{
			Article: llm.ArticleInput{
				ID:        "a1",
				Title:     "Widget 2.0 ships",
				FeedTitle: "TechDesk",
				Body:      "Widget 2.0 launched today with a 30% smaller binary.",
			},
		}},
		Selection:   "30% smaller binary",
		Question:    "变了什么？",
		Locale:      "zh-CN",
		FolderNames: []string{"工程", "产品"},
		History: []llm.Message{
			{Role: "user", Content: "旧问题"},
			{Role: "assistant", Content: "旧回答 [1]"},
		},
	}
	msgs := llm.BuildArticleChatMessages(in)
	if len(msgs) < 4 {
		t.Fatalf("len=%d want >=4", len(msgs))
	}
	if msgs[0].Role != "system" || !strings.Contains(msgs[0].Content, "简体中文") {
		t.Fatalf("system = %#v", msgs[0])
	}
	if !strings.Contains(msgs[1].Content, "[1]") || !strings.Contains(msgs[1].Content, "Widget 2.0") {
		t.Fatalf("context missing article: %s", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "30% smaller binary") {
		t.Fatalf("context missing selection: %s", msgs[1].Content)
	}
	if !strings.Contains(msgs[1].Content, "工程") {
		t.Fatalf("context missing folders: %s", msgs[1].Content)
	}
	if msgs[2].Role != "user" || msgs[3].Role != "assistant" {
		t.Fatalf("history roles = %s %s", msgs[2].Role, msgs[3].Role)
	}
	last := msgs[len(msgs)-1]
	if last.Role != "user" || !strings.Contains(last.Content, "变了什么") {
		t.Fatalf("question = %#v", last)
	}
}

func TestBuildArticleChatMessages_MultipleExcerpts(t *testing.T) {
	msgs := llm.BuildArticleChatMessages(llm.ArticleChatInput{
		Excerpts: []llm.ChatExcerpt{
			{Article: llm.ArticleInput{ID: "a", Title: "One", Body: "first body"}},
			{Article: llm.ArticleInput{ID: "b", Title: "Two", Summary: "second hit"}, Compact: true},
		},
		Question: "compare",
		Locale:   "en",
	})
	ctx := msgs[1].Content
	if !strings.Contains(ctx, "[1]") || !strings.Contains(ctx, "[2]") {
		t.Fatalf("missing numbers: %s", ctx)
	}
	if !strings.Contains(ctx, "first body") || !strings.Contains(ctx, "second hit") {
		t.Fatalf("missing bodies: %s", ctx)
	}
	if !strings.Contains(msgs[len(msgs)-1].Content, "[1]–[2]") {
		t.Fatalf("cite hint: %s", msgs[len(msgs)-1].Content)
	}
}

func TestMergeChatExcerpts_DedupAndOrder(t *testing.T) {
	cur := llm.ArticleInput{ID: "a", Title: "A", Body: "aaa"}
	got := llm.MergeChatExcerpts(&cur,
		[]llm.ArticleInput{{ID: "a", Title: "A-dup"}, {ID: "b", Title: "B", Body: "bbb"}},
		[]llm.ArticleInput{{ID: "b", Title: "B-dup"}, {ID: "c", Title: "C", Summary: "ccc"}},
		10,
	)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Article.ID != "a" || got[0].Compact {
		t.Fatalf("current should be first full: %#v", got[0])
	}
	if got[1].Article.ID != "b" || got[1].Compact {
		t.Fatalf("attached full: %#v", got[1])
	}
	if got[2].Article.ID != "c" || !got[2].Compact {
		t.Fatalf("library compact: %#v", got[2])
	}
}

func TestMergeChatExcerpts_Caps(t *testing.T) {
	var lib []llm.ArticleInput
	for i := 0; i < 20; i++ {
		lib = append(lib, llm.ArticleInput{ID: string(rune('a'+i%26)) + string(rune('0'+i/26)), Title: "t"})
	}
	got := llm.MergeChatExcerpts(nil, nil, lib, 4)
	if len(got) != 4 {
		t.Fatalf("len=%d", len(got))
	}
}

func TestBuildArticleChatMessages_TrimsLongHistory(t *testing.T) {
	var hist []llm.Message
	for i := 0; i < 20; i++ {
		hist = append(hist, llm.Message{Role: "user", Content: "u"}, llm.Message{Role: "assistant", Content: "a"})
	}
	msgs := llm.BuildArticleChatMessages(llm.ArticleChatInput{
		Excerpts: []llm.ChatExcerpt{{Article: llm.ArticleInput{ID: "a", Title: "T", Body: "B"}}},
		Question: "now",
		Locale:   "en",
		History:  hist,
	})
	if len(msgs) != 2+12+1 {
		t.Fatalf("len=%d want 15", len(msgs))
	}
}

func TestMapArticleCitations_DropsUnknown(t *testing.T) {
	allowed := llm.AllowedArticleCite(llm.ArticleInput{ID: "a1", Title: "Hello", FeedTitle: "F"})
	got := llm.MapArticleCitations("See [1] and also [7] plus [1] again.", allowed)
	if len(got) != 1 || got[0].ArticleID != "a1" || got[0].N != 1 {
		t.Fatalf("got %#v", got)
	}
	if llm.MapArticleCitations("no cites", allowed) != nil {
		t.Fatal("expected nil when no cites")
	}
}

func TestFormatArticleRef_StartsAtIndex(t *testing.T) {
	s := llm.FormatArticleRef(llm.ArticleInput{Title: "T", Body: "hello body", FeedTitle: "Feed"}, 3)
	if !strings.HasPrefix(s, "[3]\n") {
		t.Fatalf("prefix: %q", s[:8])
	}
	if !strings.Contains(s, "Feed: Feed") || !strings.Contains(s, "hello body") {
		t.Fatalf("ref = %s", s)
	}
}

func TestSessionArticleKey(t *testing.T) {
	if llm.SessionArticleKey("") != llm.LibrarySessionArticleID {
		t.Fatal("empty → library")
	}
	if llm.SessionArticleKey("art-1") != llm.LibrarySessionArticleID {
		t.Fatal("open article does not fork a session")
	}
}

func TestSystemPromptForChat(t *testing.T) {
	p := llm.SystemPromptFor(llm.FeatureChat, "zh-CN")
	if !strings.Contains(p, "RSS") && !strings.Contains(p, "reading assistant") {
		t.Fatalf("prompt = %s", p)
	}
	if !strings.Contains(p, "简体中文") {
		t.Fatalf("missing locale: %s", p)
	}
	if !strings.Contains(p, "one bullet") && !strings.Contains(p, "every excerpt") {
		t.Fatalf("list coverage missing: %s", p)
	}
}

func TestBuildArticleChatMessages_AsksToListAll(t *testing.T) {
	msgs := llm.BuildArticleChatMessages(llm.ArticleChatInput{
		Excerpts: []llm.ChatExcerpt{{Article: llm.ArticleInput{ID: "a", Title: "T", Body: "B"}}},
		Question: "v2ex 最近热门",
		Locale:   "zh-CN",
	})
	last := msgs[len(msgs)-1].Content
	if !strings.Contains(last, "列全") {
		t.Fatalf("question should ask to list all matches: %s", last)
	}
}

func TestFormatArticleRef_IncludesPublished(t *testing.T) {
	s := llm.FormatArticleRef(llm.ArticleInput{Title: "T", Body: "hello", Published: "2026-08-14T09:00:00Z"}, 1)
	if !strings.Contains(s, "Published: 2026-08-14T09:00:00Z") {
		t.Fatalf("ref = %s", s)
	}
}

func TestMatchFeedsForQuery_NamesSource(t *testing.T) {
	feeds := []llm.FeedRef{
		{ID: "v", Title: "V2EX", SiteURL: "https://www.v2ex.com/", FeedURL: "https://www.v2ex.com/index.xml"},
		{ID: "b", Title: "Bloomberg Technology", SiteURL: "https://www.bloomberg.com", FeedURL: "https://feeds.bloomberg.com/tech"},
		{ID: "n", Title: "NYT > Technology", FeedURL: "https://rss.nytimes.com/services/xml/rss/nyt/Technology.xml"},
	}
	got := llm.MatchFeedsForQuery("v2ex 最近的热门讨论有哪些?", feeds, 3)
	if len(got) != 1 || got[0].ID != "v" {
		t.Fatalf("got %#v", got)
	}
	if llm.MatchFeedsForQuery("今天该先看什么", feeds, 3) != nil {
		t.Fatal("generic question should not pin a feed")
	}
	hn := llm.MatchFeedsForQuery("what are recent hacker news posts", []llm.FeedRef{
		{ID: "h", Title: "Hacker News", SiteURL: "https://news.ycombinator.com/", FeedURL: "https://hnrss.org/frontpage"},
		{ID: "b", Title: "Bloomberg Technology", FeedURL: "https://feeds.bloomberg.com/tech"},
	}, 3)
	if len(hn) != 1 || hn[0].ID != "h" {
		t.Fatalf("hn = %#v", hn)
	}
}

func TestDefaultChatQuestion(t *testing.T) {
	if !strings.Contains(llm.DefaultChatQuestion("zh-CN"), "这篇") {
		t.Fatal("zh default")
	}
	if !strings.Contains(llm.DefaultChatQuestion("en-US"), "article") {
		t.Fatal("en default")
	}
}
