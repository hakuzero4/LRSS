package appsvc

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"lrss/internal/llm"
	"lrss/internal/model"
	"lrss/internal/repo"
	"lrss/internal/rss"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"
)

// AIService exposes user-triggered LLM features to the frontend.
type AIService struct {
	store    *settings.Store
	lib      *service.Library
	feat     *llm.Service
	chats    *repo.ChatRepo
	search   *search.Service
	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

// NewAI constructs the Wails-facing AI API.
func NewAI(store *settings.Store, lib *service.Library, db *sql.DB) *AIService {
	var cache *llm.Cache
	var chats *repo.ChatRepo
	var searcher *search.Service
	if db != nil {
		cache = &llm.Cache{DB: db}
		chats = repo.NewChatRepo(db)
		if store != nil {
			searcher = search.New(db, store)
		}
	}
	return &AIService{
		store:   store,
		lib:     lib,
		chats:   chats,
		search:  searcher,
		cancels: make(map[string]context.CancelFunc),
		feat: &llm.Service{
			Store: store,
			Cache: cache,
		},
	}
}

// AIResult is the JSON shape returned to the UI.
type AIResult struct {
	Markdown   string `json:"markdown"`
	Feature    string `json:"feature"`
	Model      string `json:"model"`
	Cached     bool   `json:"cached"`
	FolderID   string `json:"folderId,omitempty"`
	FolderName string `json:"folderName,omitempty"`
	Verdict    string `json:"verdict,omitempty"`
	// KeptOriginal is always true for translate: original body is never overwritten.
	KeptOriginal    bool   `json:"keptOriginal,omitempty"`
	TranslationRaw  string `json:"translationRaw,omitempty"`
	TranslationLang string `json:"translationLang,omitempty"`
}

func toAIResult(r llm.FeatureResult) AIResult {
	return AIResult{
		Markdown:   r.Markdown,
		Feature:    r.Feature,
		Model:      r.Model,
		Cached:     r.Cached,
		FolderID:   r.FolderID,
		FolderName: r.FolderName,
		Verdict:    r.Verdict,
	}
}

func (s *AIService) articleInput(ctx context.Context, id string) (llm.ArticleInput, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return llm.ArticleInput{}, fmt.Errorf("article id is required")
	}
	if s.lib == nil {
		return llm.ArticleInput{}, fmt.Errorf("library unavailable")
	}
	a, err := s.lib.GetArticle(ctx, id)
	if err != nil {
		return llm.ArticleInput{}, err
	}
	in := mapArticleInput(a)
	if a.FeedID != "" {
		if f, ferr := s.lib.Feeds.Get(ctx, a.FeedID); ferr == nil {
			in.FeedTitle = f.Title
		}
	}
	return in, nil
}

func mapArticleInput(a model.Article) llm.ArticleInput {
	summary, bodyText, bodyHTML, author := "", "", "", ""
	if a.Summary != nil {
		summary = *a.Summary
	}
	if a.ContentText != nil {
		bodyText = *a.ContentText
	}
	if a.ContentHTML != nil {
		bodyHTML = *a.ContentHTML
	}
	if a.Author != nil {
		author = *a.Author
	}
	published := ""
	if a.PublishedAt != nil {
		published = strings.TrimSpace(*a.PublishedAt)
	}
	return llm.ArticleInput{
		ID:        a.ID,
		Title:     a.Title,
		Summary:   summary,
		Body:      llm.PlainBody(bodyText, bodyHTML),
		URL:       a.URL,
		Author:    author,
		Published: published,
	}
}

// Summarize streams a deck-style summary into the reader (via llm:stream events),
// then replaces the article's stored summary. locale is the app UI language.
func (s *AIService) Summarize(articleId, locale string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}

	emitLLMStream(LLMStreamEvent{
		ArticleID: articleId,
		Feature:   llm.FeatureSummarize,
		Done:      false,
	})

	r, err := s.feat.SummarizeStream(ctx, in, locale, func(delta, full string) {
		emitLLMStream(LLMStreamEvent{
			ArticleID: articleId,
			Feature:   llm.FeatureSummarize,
			Delta:     delta,
			Text:      full,
			Done:      false,
		})
	})
	if err != nil {
		emitLLMStream(LLMStreamEvent{
			ArticleID: articleId,
			Feature:   llm.FeatureSummarize,
			Done:      true,
			Error:     err.Error(),
		})
		return AIResult{}, err
	}

	// Persist: replace original feed summary with AI deck text.
	if s.lib != nil && strings.TrimSpace(r.Markdown) != "" {
		if uerr := s.lib.UpdateArticleSummary(ctx, articleId, r.Markdown); uerr != nil {
			// Still return text; emit warning via error field empty + frontend has text.
			emitLLMStream(LLMStreamEvent{
				ArticleID: articleId,
				Feature:   llm.FeatureSummarize,
				Text:      r.Markdown,
				Done:      true,
				Model:     r.Model,
				Cached:    r.Cached,
				Error:     "save summary: " + uerr.Error(),
			})
			return toAIResult(r), uerr
		}
	}

	emitLLMStream(LLMStreamEvent{
		ArticleID: articleId,
		Feature:   llm.FeatureSummarize,
		Text:      r.Markdown,
		Done:      true,
		Model:     r.Model,
		Cached:    r.Cached,
	})
	return toAIResult(r), nil
}

// Translate streams a bilingual interlinear translation (llm:stream, feature=translate).
// targetLang is e.g. zh-CN or en. Output uses <<o>>/<<t>> pairs.
// Always keeps the original content_html; bilingual text is saved on the article as translationRaw.
func (s *AIService) Translate(articleId, targetLang string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	if strings.TrimSpace(targetLang) == "" {
		targetLang = "zh-CN"
	}

	emitLLMStream(LLMStreamEvent{
		ArticleID: articleId,
		Feature:   llm.FeatureTranslate,
		Done:      false,
	})

	r, err := s.feat.TranslateStream(ctx, in, targetLang, func(delta, full string) {
		emitLLMStream(LLMStreamEvent{
			ArticleID: articleId,
			Feature:   llm.FeatureTranslate,
			Delta:     delta,
			Text:      full,
			Done:      false,
		})
	})
	if err != nil {
		emitLLMStream(LLMStreamEvent{
			ArticleID: articleId,
			Feature:   llm.FeatureTranslate,
			Done:      true,
			Error:     err.Error(),
		})
		return AIResult{}, err
	}

	out := toAIResult(r)
	// Persist bilingual next to original — never overwrite content_html / content_text.
	if s.lib != nil && strings.TrimSpace(r.Markdown) != "" {
		if uerr := s.lib.SaveArticleTranslation(ctx, articleId, r.Markdown, targetLang); uerr != nil {
			emitLLMStream(LLMStreamEvent{
				ArticleID: articleId,
				Feature:   llm.FeatureTranslate,
				Text:      r.Markdown,
				Done:      true,
				Model:     r.Model,
				Cached:    r.Cached,
				Error:     "save translation: " + uerr.Error(),
			})
			return out, uerr
		}
		out.TranslationRaw = r.Markdown
		out.TranslationLang = targetLang
		out.KeptOriginal = true
	}

	emitLLMStream(LLMStreamEvent{
		ArticleID: articleId,
		Feature:   llm.FeatureTranslate,
		Text:      r.Markdown,
		Done:      true,
		Model:     r.Model,
		Cached:    r.Cached,
	})
	return out, nil
}

// ClearTranslation removes saved bilingual text; original body is unchanged.
func (s *AIService) ClearTranslation(articleId string) error {
	if s.lib == nil {
		return fmt.Errorf("library unavailable")
	}
	return s.lib.ClearArticleTranslation(context.Background(), articleId)
}

// DetectContentFullness judges whether the stored body looks complete or partial.
// Verdict: full|partial|unclear|skipped_no_url|already_fetched|skipped_youtube.
// Does not fetch the page. Avoids false "partial" on YouTube embeds and full RSS bodies.
func (s *AIService) DetectContentFullness(articleId string) (AIResult, error) {
	ctx := context.Background()
	// Prefer raw article so we can honor full_content_fetched without re-judging.
	if s.lib != nil {
		a, err := s.lib.GetArticle(ctx, articleId)
		if err != nil {
			return AIResult{}, err
		}
		if a.FullContentFetched {
			return AIResult{
				Feature:  llm.FeatureContentFullness,
				Verdict:  "already_fetched",
				Markdown: "VERDICT: full\nalready_fetched",
				Cached:   true,
			}, nil
		}
		if skip, verdict, md := fullnessSkipFromArticle(a); skip {
			return AIResult{
				Feature:  llm.FeatureContentFullness,
				Verdict:  verdict,
				Markdown: md,
				Cached:   true,
			}, nil
		}
		in := mapArticleInput(a)
		if strings.TrimSpace(in.URL) == "" {
			return AIResult{
				Feature:  llm.FeatureContentFullness,
				Verdict:  "skipped_no_url",
				Markdown: "VERDICT: skipped_no_url",
			}, nil
		}
		det, err := s.feat.DetectContentFullness(ctx, in)
		if err != nil {
			return AIResult{}, err
		}
		out := toAIResult(det)
		out.Verdict = det.Verdict
		return out, nil
	}

	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	if strings.TrimSpace(in.URL) == "" {
		return AIResult{
			Feature:  llm.FeatureContentFullness,
			Verdict:  "skipped_no_url",
			Markdown: "VERDICT: skipped_no_url",
		}, nil
	}
	det, err := s.feat.DetectContentFullness(ctx, in)
	if err != nil {
		return AIResult{}, err
	}
	out := toAIResult(det)
	out.Verdict = det.Verdict
	return out, nil
}

// fullnessSkipFromArticle returns early full/skip when auto page-fetch must not run.
func fullnessSkipFromArticle(a model.Article) (skip bool, verdict, markdown string) {
	url := strings.TrimSpace(a.URL)
	if rss.YouTubeVideoID(url) != "" {
		// YouTube "body" is embed + description/captions — not an HTML article to expand.
		return true, "skipped_youtube", "VERDICT: full\nskipped_youtube"
	}
	html := ""
	if a.ContentHTML != nil {
		html = *a.ContentHTML
	}
	// Already have embed / captions block → treat as complete for our reader.
	if strings.Contains(html, "yt-embed") ||
		strings.Contains(html, `data-yt-captions="1"`) ||
		strings.Contains(html, `id="lrss-yt-captions"`) {
		return true, llm.FullnessFull, "VERDICT: full\nyt_content_present"
	}
	return false, "", ""
}

// EnsureFullContent uses the LLM to judge whether the stored body is only a
// partial excerpt; if partial (or empty), fetches the original page via fulltext.
// Verdict: full|partial|fetched|skipped_no_url|skipped_unclear
// Prefer frontend DetectContentFullness + FetchFullContent when UI needs a
// “starting fetch” toast before body replacement.
func (s *AIService) EnsureFullContent(articleId string) (AIResult, error) {
	ctx := context.Background()
	out, err := s.DetectContentFullness(articleId)
	if err != nil {
		return AIResult{}, err
	}
	if out.Verdict != llm.FullnessPartial {
		// full or unclear → do not auto-fetch (avoid false positives).
		return out, nil
	}

	if s.lib == nil {
		return out, fmt.Errorf("library unavailable")
	}
	if _, ferr := s.lib.FetchFullContent(ctx, articleId); ferr != nil {
		return out, ferr
	}
	out.Verdict = "fetched"
	out.Markdown = strings.TrimSpace(out.Markdown) + "\n\nFETCHED: yes"
	return out, nil
}

// TranslateSelection translates a short in-reader text selection (划词翻译).
// Uses a fixed plain-translation prompt (not bilingual markers). targetLang e.g. zh-CN / en.
func (s *AIService) TranslateSelection(text, targetLang string) (AIResult, error) {
	ctx := context.Background()
	text = strings.TrimSpace(text)
	if text == "" {
		return AIResult{}, fmt.Errorf("selection text is required")
	}
	if strings.TrimSpace(targetLang) == "" {
		targetLang = "zh-CN"
	}
	r, err := s.feat.SelectTranslate(ctx, text, targetLang)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// Ask answers a question about the article (empty question → default overview in UI locale).
func (s *AIService) Ask(articleId, question, locale string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	r, err := s.feat.Ask(ctx, in, question, locale)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// SuggestFolders returns tag/folder suggestions for an article.
// When LLM is off, returns local-rule tags only. locale is app UI language.
func (s *AIService) SuggestFolders(articleId, locale string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	var folders []llm.FolderRef
	if s.lib != nil {
		fl, err := s.lib.ListFolders(ctx)
		if err == nil {
			for _, f := range fl {
				folders = append(folders, llm.FolderRef{ID: f.ID, Name: f.Name})
			}
		}
	}
	r, err := s.feat.Suggest(ctx, in, folders, locale)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// ApplySuggestedFolder moves the article's feed into folderId (existing MoveFeed).
func (s *AIService) ApplySuggestedFolder(articleId, folderId string) error {
	ctx := context.Background()
	if s.lib == nil {
		return fmt.Errorf("library unavailable")
	}
	a, err := s.lib.GetArticle(ctx, articleId)
	if err != nil {
		return err
	}
	var folder *string
	if id := strings.TrimSpace(folderId); id != "" {
		folder = &id
	}
	return s.lib.MoveFeed(ctx, a.FeedID, folder)
}

// ClassifyPromo classifies the article as organic / promo / unclear (user-triggered).
// locale is the app UI language for explanation text.
func (s *AIService) ClassifyPromo(articleId, locale string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	r, err := s.feat.ClassifyPromo(ctx, in, locale)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// IsLLMConfigured reports whether chat LLM is ready (for UI enablement).
func (s *AIService) IsLLMConfigured() (bool, error) {
	if s.store == nil {
		return false, nil
	}
	cfg, err := s.store.LoadLLMConfig(context.Background())
	if err != nil {
		return false, err
	}
	return cfg.IsConfigured(), nil
}

// ChatSendRequest is one user turn against the current article and/or library.
type ChatSendRequest struct {
	SessionID    string   `json:"sessionId"`
	Message      string   `json:"message"`
	ArticleID    string   `json:"articleId"`
	AttachIDs    []string `json:"attachIds"`
	CollectionID string   `json:"collectionId"`
	Selection    string   `json:"selection"`
	Locale       string   `json:"locale"`
	UseLibrary   bool     `json:"useLibrary"`
}

// ChatSendResult is the completed assistant turn.
type ChatSendResult struct {
	SessionID string               `json:"sessionId"`
	MessageID string               `json:"messageId"`
	Markdown  string               `json:"markdown"`
	Model     string               `json:"model"`
	Citations []model.ChatCitation `json:"citations,omitempty"`
}

// ChatHistoryResult is the persisted transcript for an article.
type ChatHistoryResult struct {
	SessionID string              `json:"sessionId"`
	ArticleID string              `json:"articleId"`
	Messages  []model.ChatMessage `json:"messages"`
}

const chatTimeout = 90 * time.Second

func (s *AIService) folderNames(ctx context.Context) []string {
	if s.lib == nil {
		return nil
	}
	fl, err := s.lib.ListFolders(ctx)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(fl))
	for _, f := range fl {
		if n := strings.TrimSpace(f.Name); n != "" {
			names = append(names, n)
		}
	}
	return names
}

func historyFromMessages(msgs []model.ChatMessage) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		role := strings.TrimSpace(m.Role)
		if role != "user" && role != "assistant" {
			continue
		}
		if strings.TrimSpace(m.Content) == "" {
			continue
		}
		out = append(out, llm.Message{Role: role, Content: m.Content})
	}
	return out
}

func (s *AIService) trackCancel(sessionID string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
	s.cancelMu.Lock()
	if s.cancels == nil {
		s.cancels = make(map[string]context.CancelFunc)
	}
	if prev, ok := s.cancels[sessionID]; ok {
		prev()
	}
	s.cancels[sessionID] = cancel
	s.cancelMu.Unlock()
	return ctx, func() {
		cancel()
		s.cancelMu.Lock()
		if cur, ok := s.cancels[sessionID]; ok && fmt.Sprintf("%p", cur) == fmt.Sprintf("%p", cancel) {
			delete(s.cancels, sessionID)
		}
		s.cancelMu.Unlock()
	}
}

// ChatHistory returns the saved conversation for an article (empty if none).
// Empty articleId loads the library-wide session.
func (s *AIService) ChatHistory(articleId string) (ChatHistoryResult, error) {
	key := llm.SessionArticleKey(articleId)
	if s.chats == nil {
		return ChatHistoryResult{ArticleID: articleId}, nil
	}
	ctx := context.Background()
	sess, err := s.chats.GetByArticle(ctx, key)
	if err == sql.ErrNoRows {
		return ChatHistoryResult{ArticleID: articleId}, nil
	}
	if err != nil {
		return ChatHistoryResult{}, err
	}
	msgs, err := s.chats.ListMessages(ctx, sess.ID)
	if err != nil {
		return ChatHistoryResult{}, err
	}
	if msgs == nil {
		msgs = []model.ChatMessage{}
	}
	return ChatHistoryResult{SessionID: sess.ID, ArticleID: articleId, Messages: msgs}, nil
}

// ChatClear deletes the article's assistant session.
// Empty articleId clears the library-wide session.
func (s *AIService) ChatClear(articleId string) error {
	key := llm.SessionArticleKey(articleId)
	if s.chats == nil {
		return nil
	}
	if sess, err := s.chats.GetByArticle(context.Background(), key); err == nil {
		s.ChatCancel(sess.ID)
	}
	return s.chats.DeleteByArticle(context.Background(), key)
}

// ChatCancel aborts an in-flight ChatSend for the session.
func (s *AIService) ChatCancel(sessionId string) error {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return nil
	}
	s.cancelMu.Lock()
	fn := s.cancels[sessionId]
	delete(s.cancels, sessionId)
	s.cancelMu.Unlock()
	if fn != nil {
		fn()
	}
	return nil
}

// ChatSend streams a multi-turn answer about the current article and/or library
// (llm:stream, feature=chat).
func (s *AIService) ChatSend(req ChatSendRequest) (ChatSendResult, error) {
	articleID := strings.TrimSpace(req.ArticleID)
	if s.chats == nil {
		return ChatSendResult{}, fmt.Errorf("chat store unavailable")
	}
	locale := strings.TrimSpace(req.Locale)
	if locale == "" {
		locale = "zh-CN"
	}
	question := strings.TrimSpace(req.Message)
	if question == "" {
		question = llm.DefaultChatQuestion(locale)
	}
	if ok, cfgErr := s.IsLLMConfigured(); cfgErr != nil {
		return ChatSendResult{}, cfgErr
	} else if !ok {
		return ChatSendResult{}, fmt.Errorf("llm not configured: enable a chat model in Settings → Search/AI")
	}

	useLibrary := req.UseLibrary
	if !useLibrary && strings.TrimSpace(req.ArticleID) == "" && !hasAttachIDs(req.AttachIDs) {
		// Global assistant with nothing pinned: search the library instead of failing.
		useLibrary = true
	}
	excerpts, err := s.collectChatExcerpts(context.Background(), req, question, useLibrary)
	if err != nil {
		return ChatSendResult{}, err
	}
	if len(excerpts) == 0 {
		return ChatSendResult{}, fmt.Errorf("no article context: open an article, attach one, or search the library")
	}

	sessionKey := llm.SessionArticleKey(articleID)
	sess, err := s.chats.GetOrCreateByArticle(context.Background(), sessionKey, locale)
	if err != nil {
		return ChatSendResult{}, err
	}

	prior, err := s.chats.ListMessages(context.Background(), sess.ID)
	if err != nil {
		return ChatSendResult{}, err
	}
	userMsg := &model.ChatMessage{SessionID: sess.ID, Role: "user", Content: question}
	if sel := strings.TrimSpace(req.Selection); sel != "" {
		userMsg.Content = question + "\n\n> " + sel
	}
	if err := s.chats.InsertMessage(context.Background(), userMsg); err != nil {
		return ChatSendResult{}, err
	}

	ctx, done := s.trackCancel(sess.ID)
	defer done()

	emitLLMStream(LLMStreamEvent{
		ArticleID: articleID,
		SessionID: sess.ID,
		Feature:   llm.FeatureChat,
		Done:      false,
	})

	result, err := s.feat.ChatArticleStream(ctx, llm.ArticleChatInput{
		Excerpts:    excerpts,
		Selection:   req.Selection,
		Question:    question,
		Locale:      locale,
		History:     historyFromMessages(prior),
		FolderNames: s.folderNames(ctx),
	}, func(delta, full string) {
		emitLLMStream(LLMStreamEvent{
			ArticleID: articleID,
			SessionID: sess.ID,
			Feature:   llm.FeatureChat,
			Delta:     delta,
			Text:      full,
			Done:      false,
		})
	})
	if err != nil {
		emitLLMStream(LLMStreamEvent{
			ArticleID: articleID,
			SessionID: sess.ID,
			Feature:   llm.FeatureChat,
			Done:      true,
			Error:     err.Error(),
		})
		return ChatSendResult{}, err
	}

	asst := &model.ChatMessage{
		SessionID: sess.ID,
		Role:      "assistant",
		Content:   result.Markdown,
		Citations: result.Citations,
	}
	if ierr := s.chats.InsertMessage(context.Background(), asst); ierr != nil {
		emitLLMStream(LLMStreamEvent{
			ArticleID: articleID,
			SessionID: sess.ID,
			Feature:   llm.FeatureChat,
			Text:      result.Markdown,
			Done:      true,
			Model:     result.Model,
			Error:     "save chat: " + ierr.Error(),
		})
		return ChatSendResult{
			SessionID: sess.ID,
			Markdown:  result.Markdown,
			Model:     result.Model,
			Citations: result.Citations,
		}, ierr
	}

	emitLLMStream(LLMStreamEvent{
		ArticleID: articleID,
		SessionID: sess.ID,
		Feature:   llm.FeatureChat,
		Text:      result.Markdown,
		Done:      true,
		Model:     result.Model,
	})
	return ChatSendResult{
		SessionID: sess.ID,
		MessageID: asst.ID,
		Markdown:  result.Markdown,
		Model:     result.Model,
		Citations: result.Citations,
	}, nil
}

func (s *AIService) excludeNsfw(ctx context.Context) bool {
	if s.store == nil {
		return false
	}
	prefs, err := s.store.LoadUIPrefs(ctx)
	if err != nil {
		return false
	}
	return !prefs.NsfwMode
}

func (s *AIService) collectChatExcerpts(ctx context.Context, req ChatSendRequest, question string, useLibrary bool) ([]llm.ChatExcerpt, error) {
	var current *llm.ArticleInput
	if id := strings.TrimSpace(req.ArticleID); id != "" {
		in, err := s.articleInput(ctx, id)
		if err != nil {
			return nil, err
		}
		current = &in
	}

	attached := make([]llm.ArticleInput, 0, len(req.AttachIDs))
	for _, raw := range req.AttachIDs {
		id := strings.TrimSpace(raw)
		if id == "" || (current != nil && id == current.ID) {
			continue
		}
		in, err := s.articleInput(ctx, id)
		if err != nil {
			continue
		}
		attached = append(attached, in)
	}

	var library []llm.ArticleInput
	if useLibrary {
		library = s.libraryExcerpts(ctx, req.CollectionID, question, current, attached)
	}
	return llm.MergeChatExcerpts(current, attached, library, llm.MaxChatArticles), nil
}

func hasAttachIDs(ids []string) bool {
	for _, raw := range ids {
		if strings.TrimSpace(raw) != "" {
			return true
		}
	}
	return false
}

func (s *AIService) libraryExcerpts(ctx context.Context, collectionID, question string, current *llm.ArticleInput, attached []llm.ArticleInput) []llm.ArticleInput {
	if s.lib == nil {
		return nil
	}
	exclude := s.excludeNsfw(ctx)
	skip := map[string]bool{}
	if current != nil {
		skip[current.ID] = true
	}
	for _, a := range attached {
		skip[a.ID] = true
	}

	limit := llm.MaxChatArticles
	var out []llm.ArticleInput
	addListed := func(arts []model.Article) {
		for _, a := range arts {
			if len(out) >= limit {
				return
			}
			if skip[a.ID] {
				continue
			}
			in := mapArticleInput(a)
			if a.FeedID != "" && s.lib != nil {
				if f, err := s.lib.Feeds.Get(ctx, a.FeedID); err == nil {
					in.FeedTitle = f.Title
				}
			}
			skip[a.ID] = true
			out = append(out, in)
		}
	}

	q := strings.TrimSpace(question)

	// 1) Named subscriptions first ("v2ex 最近热门") so the unread pile cannot crowd them out.
	if feeds, err := s.lib.ListFeeds(ctx); err == nil && q != "" {
		refs := make([]llm.FeedRef, 0, len(feeds))
		for _, f := range feeds {
			site := ""
			if f.SiteURL != nil {
				site = *f.SiteURL
			}
			refs = append(refs, llm.FeedRef{ID: f.ID, Title: f.Title, SiteURL: site, FeedURL: f.FeedURL})
		}
		matched := llm.MatchFeedsForQuery(q, refs, llm.MaxMatchedFeeds)
		perFeed := 12
		if n := len(matched); n > 1 {
			perFeed = (limit + n - 1) / n
			if perFeed < 6 {
				perFeed = 6
			}
		}
		for _, f := range matched {
			if len(out) >= limit {
				break
			}
			listed, lerr := s.lib.ListArticles(ctx, "feed:"+f.ID, perFeed, 0, exclude)
			if lerr == nil {
				addListed(listed)
			}
		}
	}

	// 2) Search the question itself (do not prepend the open article title).
	if s.search != nil && q != "" && len(out) < limit {
		res, err := s.search.Search(ctx, q, search.Options{
			Mode:        "",
			Limit:       limit,
			ExcludeNsfw: exclude,
		})
		if err == nil {
			for _, hit := range res.Hits {
				if len(out) >= limit {
					break
				}
				if skip[hit.ArticleID] {
					continue
				}
				in, aerr := s.articleInput(ctx, hit.ArticleID)
				if aerr != nil {
					continue
				}
				if strings.TrimSpace(in.Summary) == "" {
					in.Summary = hit.Snippet
				}
				skip[in.ID] = true
				out = append(out, in)
			}
		}
	}

	// 3) Current collection only as a last resort when retrieval is still thin.
	if len(out) < 4 {
		col := strings.TrimSpace(collectionID)
		if col == "" {
			col = "unread"
		}
		if listed, err := s.lib.ListArticles(ctx, col, 8, 0, exclude); err == nil {
			addListed(listed)
		}
	}
	return out
}
