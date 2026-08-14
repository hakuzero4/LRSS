package llm

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"lrss/internal/model"
)

// MaxChatTurns is how many previous user+assistant pairs to send (not counting the new question).
const MaxChatTurns = 6

// MaxChatArticles caps excerpts sent in one turn (local-model context).
const MaxChatArticles = 16

// MaxMatchedFeeds is how many named subscriptions a question can pull from.
const MaxMatchedFeeds = 3

// LibrarySessionArticleID is the synthetic session key for library-wide chat.
const LibrarySessionArticleID = "__library__"

const articleChatSystemPromptBase = `You are a reading assistant inside a personal RSS reader. Answer only from the provided excerpts labeled [n].

Rules:
- Extract facts, numbers, dates, names, and what changed. Prefer the excerpts over general knowledge.
- Cite claims with [n] using only numbers that appear in the input. Never invent numbers. Never output article IDs or URLs.
- If the excerpts do not contain the answer, say so in one sentence. Do not guess.
- Write scannable Markdown. No "As an AI" throat-clearing.
- Do not start with "This article discusses" / 「本文介绍了」.
- When several excerpts cover the same event, synthesize and cite every relevant [n].
- For list / "what's new / hot / recent" questions, cover every excerpt that matches — one bullet + [n] each. Do not stop after two items when more excerpts apply.
- When the question names a source or feed, prefer excerpts from that feed and skip unrelated ones.`

// ChatExcerpt is one numbered article the model may cite.
type ChatExcerpt struct {
	Article ArticleInput
	Compact bool // search/list hits: title + short summary only
}

// ArticleChatInput is one multi-turn question about one or more articles.
type ArticleChatInput struct {
	Excerpts    []ChatExcerpt
	Selection   string
	Question    string
	Locale      string
	History     []Message // previous user/assistant turns, oldest first
	FolderNames []string
}

// ArticleChatResult is the completed assistant reply.
type ArticleChatResult struct {
	Markdown  string
	Model     string
	Citations []model.ChatCitation
}

// MergeChatExcerpts orders current + attached + library hits, de-duplicating by ID.
func MergeChatExcerpts(current *ArticleInput, attached, library []ArticleInput, max int) []ChatExcerpt {
	if max <= 0 {
		max = MaxChatArticles
	}
	seen := map[string]bool{}
	out := make([]ChatExcerpt, 0, max)
	add := func(a ArticleInput, compact bool) {
		id := strings.TrimSpace(a.ID)
		if id == "" {
			id = strings.TrimSpace(a.URL) + "\n" + strings.TrimSpace(a.Title)
		}
		if id == "" || seen[id] || len(out) >= max {
			return
		}
		seen[id] = true
		out = append(out, ChatExcerpt{Article: a, Compact: compact})
	}
	if current != nil && (strings.TrimSpace(current.ID) != "" || strings.TrimSpace(current.Title) != "" || strings.TrimSpace(current.Body) != "") {
		add(*current, false)
	}
	for _, a := range attached {
		add(a, false)
	}
	for _, a := range library {
		add(a, true)
	}
	return out
}

// FormatArticleRef renders excerpt [n].
func FormatArticleRef(a ArticleInput, n int) string {
	return formatArticleRef(a, n, false)
}

func formatArticleRef(a ArticleInput, n int, compact bool) string {
	if n < 1 {
		n = 1
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[%d]\n", n)
	title := strings.TrimSpace(a.Title)
	if title == "" {
		title = "(untitled)"
	}
	b.WriteString("Title: ")
	b.WriteString(title)
	b.WriteByte('\n')
	if ft := strings.TrimSpace(a.FeedTitle); ft != "" {
		b.WriteString("Feed: ")
		b.WriteString(ft)
		b.WriteByte('\n')
	}
	if a.Author != "" {
		b.WriteString("Author: ")
		b.WriteString(strings.TrimSpace(a.Author))
		b.WriteByte('\n')
	}
	if a.URL != "" {
		b.WriteString("URL: ")
		b.WriteString(strings.TrimSpace(a.URL))
		b.WriteByte('\n')
	}
	if pub := strings.TrimSpace(a.Published); pub != "" {
		b.WriteString("Published: ")
		b.WriteString(pub)
		b.WriteByte('\n')
	}
	if compact {
		sum := strings.TrimSpace(a.Summary)
		if sum == "" {
			sum = strings.TrimSpace(a.Body)
		}
		b.WriteString("Summary: ")
		b.WriteString(BudgetText(sum, 400))
		b.WriteByte('\n')
		return b.String()
	}
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	if i := strings.Index(bundle, "Body:\n"); i >= 0 {
		b.WriteString(bundle[i:])
	} else if s := strings.TrimSpace(a.Summary); s != "" {
		b.WriteString("Summary: ")
		b.WriteString(BudgetText(s, 800))
		b.WriteByte('\n')
	}
	return b.String()
}

func trimChatHistory(hist []Message, maxTurns int) []Message {
	if maxTurns <= 0 {
		maxTurns = MaxChatTurns
	}
	maxMsgs := maxTurns * 2
	if len(hist) <= maxMsgs {
		return hist
	}
	return hist[len(hist)-maxMsgs:]
}

func citeHint(n int, locale string) string {
	if n <= 1 {
		if NormalizeUILocale(locale) == "zh" {
			return "[1]"
		}
		return "[1]"
	}
	if NormalizeUILocale(locale) == "zh" {
		return fmt.Sprintf("[1]–[%d]", n)
	}
	return fmt.Sprintf("[1]–[%d]", n)
}

// BuildArticleChatMessages assembles system + context + history + question.
func BuildArticleChatMessages(in ArticleChatInput) []Message {
	locale := in.Locale
	sys := SystemPromptFor(FeatureChat, locale)
	excerpts := in.Excerpts
	if len(excerpts) == 0 {
		excerpts = []ChatExcerpt{}
	}

	var ctx strings.Builder
	if NormalizeUILocale(locale) == "zh" {
		ctx.WriteString("订阅摘录。回答时用编号引用，例如 ")
		ctx.WriteString(citeHint(len(excerpts), locale))
		ctx.WriteString("。\n\n")
	} else {
		ctx.WriteString("Library excerpts. Cite with numbers such as ")
		ctx.WriteString(citeHint(len(excerpts), locale))
		ctx.WriteString(".\n\n")
	}
	for i, ex := range excerpts {
		if i > 0 {
			ctx.WriteString("\n")
		}
		ctx.WriteString(formatArticleRef(ex.Article, i+1, ex.Compact))
	}
	if names := in.FolderNames; len(names) > 0 {
		ctx.WriteString("\n\n")
		if NormalizeUILocale(locale) == "zh" {
			ctx.WriteString("现有文件夹：")
		} else {
			ctx.WriteString("Available folders: ")
		}
		ctx.WriteString(strings.Join(names, ", "))
	}
	if sel := strings.TrimSpace(in.Selection); sel != "" {
		ctx.WriteString("\n\n")
		if NormalizeUILocale(locale) == "zh" {
			ctx.WriteString("用户划选的段落：\n")
		} else {
			ctx.WriteString("User-selected passage:\n")
		}
		ctx.WriteString(BudgetText(sel, MaxSelectTranslateChars))
	}

	msgs := []Message{
		{Role: "system", Content: sys},
		{Role: "user", Content: ctx.String()},
	}
	for _, m := range trimChatHistory(in.History, MaxChatTurns) {
		role := strings.TrimSpace(m.Role)
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		if role != "user" && role != "assistant" {
			continue
		}
		msgs = append(msgs, Message{Role: role, Content: content})
	}
	q := strings.TrimSpace(in.Question)
	if q == "" {
		q = DefaultChatQuestion(locale)
	}
	hint := citeHint(len(excerpts), locale)
	if NormalizeUILocale(locale) == "zh" {
		msgs = append(msgs, Message{Role: "user", Content: "问题：" + q + "\n\n请用 Markdown 回答。列举 / 最近 / 热门类问题请把相关摘录尽量列全（每条一个要点 + 编号），不要只写两三条。需要出处时用 " + hint + "。"})
	} else {
		msgs = append(msgs, Message{Role: "user", Content: "Question: " + q + "\n\nAnswer in Markdown. For list / recent / hot questions, cover every matching excerpt (one bullet + [n] each) — do not stop after two. Cite " + hint + " when needed."})
	}
	return msgs
}

// MapArticleCitations keeps only [n] that exist in allowed.
func MapArticleCitations(text string, allowed map[int]model.ChatCitation) []model.ChatCitation {
	ns := extractCiteNs(text)
	if len(ns) == 0 {
		return nil
	}
	var out []model.ChatCitation
	seen := map[int]bool{}
	for _, n := range ns {
		if seen[n] {
			continue
		}
		c, ok := allowed[n]
		if !ok {
			continue
		}
		seen[n] = true
		c.N = n
		out = append(out, c)
	}
	return out
}

// AllowedCites maps 1-based indices to excerpts.
func AllowedCites(excerpts []ChatExcerpt) map[int]model.ChatCitation {
	out := make(map[int]model.ChatCitation, len(excerpts))
	for i, ex := range excerpts {
		n := i + 1
		out[n] = model.ChatCitation{
			N:         n,
			ArticleID: ex.Article.ID,
			Title:     ex.Article.Title,
			FeedTitle: ex.Article.FeedTitle,
		}
	}
	return out
}

// AllowedArticleCite is [1] → the current article (compat helper).
func AllowedArticleCite(a ArticleInput) map[int]model.ChatCitation {
	return AllowedCites([]ChatExcerpt{{Article: a}})
}

// DefaultChatQuestion returns the empty-question fallback.
func DefaultChatQuestion(locale string) string {
	if NormalizeUILocale(locale) == "zh" {
		return "这篇在说什么？列出要点、数字和没说清的前提。"
	}
	return "What is this article about? List claims, numbers, and anything left unclear."
}

// FormatChatUserLine is unused by the model path but handy for tests.
func FormatChatUserLine(question, locale string) string {
	q := strings.TrimSpace(question)
	if q == "" {
		q = DefaultChatQuestion(locale)
	}
	return q
}

// SessionArticleKey is always the global assistant session.
// The open article is context you attach, not a separate chat.
func SessionArticleKey(articleID string) string {
	_ = articleID
	return LibrarySessionArticleID
}

// FeedRef is a subscription the chat retriever can match against a question.
type FeedRef struct {
	ID      string
	Title   string
	SiteURL string
	FeedURL string
}

// chatQueryStop drops words that never identify a feed.
var chatQueryStop = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "of": {}, "in": {}, "on": {}, "to": {}, "for": {},
	"is": {}, "are": {}, "was": {}, "were": {}, "what": {}, "which": {}, "who": {},
	"how": {}, "why": {}, "when": {}, "where": {}, "this": {}, "that": {}, "these": {},
	"those": {}, "and": {}, "or": {}, "with": {}, "from": {}, "about": {},
	"recent": {}, "latest": {}, "hot": {}, "new": {}, "news": {}, "today": {},
	"discuss": {}, "discussion": {}, "discussions": {}, "topic": {}, "topics": {},
	"list": {}, "show": {}, "tell": {}, "me": {}, "please": {}, "any": {},
	"recently": {}, "popular": {}, "trending": {}, "top": {},
	"最近": {}, "热门": {}, "热点": {}, "讨论": {}, "话题": {}, "哪些": {}, "什么": {},
	"有哪些": {}, "最近的": {}, "热门的": {}, "看看": {}, "说说": {}, "总结": {},
	"一篇": {}, "文章": {}, "订阅": {}, "订阅库": {}, "今天": {}, "最新": {},
	"有什么": {}, "怎么": {}, "如何": {}, "一下": {},
}

// QueryFeedTokens extracts distinctive tokens used to match subscriptions.
func QueryFeedTokens(q string) []string {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return nil
	}
	var b strings.Builder
	var toks []string
	seen := map[string]bool{}
	flush := func() {
		t := strings.TrimSpace(b.String())
		b.Reset()
		if t == "" {
			return
		}
		if _, stop := chatQueryStop[t]; stop {
			return
		}
		n := utf8.RuneCountInString(t)
		if n < 2 {
			return
		}
		if seen[t] {
			return
		}
		seen[t] = true
		toks = append(toks, t)
	}
	for _, r := range q {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return toks
}

func feedHostLabels(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		raw = u.Host
	}
	raw = strings.ToLower(raw)
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '.' || r == ':' || r == '/' || r == '-' || r == '_'
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "www" || p == "http" || p == "https" || p == "com" || p == "org" || p == "net" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ScoreFeedMatch scores how well a question points at one subscription.
func ScoreFeedMatch(title, siteURL, feedURL, query string, tokens []string) int {
	titleLow := strings.ToLower(strings.TrimSpace(title))
	q := strings.ToLower(strings.TrimSpace(query))
	if titleLow == "" && siteURL == "" && feedURL == "" {
		return 0
	}
	score := 0
	if titleLow != "" && utf8.RuneCountInString(titleLow) >= 2 && strings.Contains(q, titleLow) {
		score += 20
	}
	labels := append(feedHostLabels(siteURL), feedHostLabels(feedURL)...)
	for _, tok := range tokens {
		if titleLow != "" {
			if titleLow == tok {
				score += 16
			} else if strings.Contains(titleLow, tok) {
				score += 8
			}
		}
		for _, lab := range labels {
			if lab == tok {
				score += 14
			} else if strings.Contains(lab, tok) && utf8.RuneCountInString(tok) >= 3 {
				score += 6
			}
		}
	}
	return score
}

// MatchFeedsForQuery returns subscriptions the question names, best first.
func MatchFeedsForQuery(query string, feeds []FeedRef, max int) []FeedRef {
	if max <= 0 {
		max = MaxMatchedFeeds
	}
	tokens := QueryFeedTokens(query)
	if strings.TrimSpace(query) == "" || len(feeds) == 0 {
		return nil
	}
	type scored struct {
		feed  FeedRef
		score int
	}
	var hits []scored
	for _, f := range feeds {
		s := ScoreFeedMatch(f.Title, f.SiteURL, f.FeedURL, query, tokens)
		if s < 8 {
			continue
		}
		hits = append(hits, scored{feed: f, score: s})
	}
	if len(hits) == 0 {
		return nil
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].feed.Title < hits[j].feed.Title
	})
	if len(hits) > max {
		hits = hits[:max]
	}
	out := make([]FeedRef, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.feed)
	}
	return out
}
