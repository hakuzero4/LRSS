package appsvc

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"lrss/internal/llm"
	"lrss/internal/model"
	"lrss/internal/service"
	"lrss/internal/settings"
)

// AIService exposes user-triggered LLM features to the frontend.
type AIService struct {
	store *settings.Store
	lib   *service.Library
	feat  *llm.Service
}

// NewAI constructs the Wails-facing AI API.
func NewAI(store *settings.Store, lib *service.Library, db *sql.DB) *AIService {
	var cache *llm.Cache
	if db != nil {
		cache = &llm.Cache{DB: db}
	}
	return &AIService{
		store: store,
		lib:   lib,
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
	return mapArticleInput(a), nil
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
	return llm.ArticleInput{
		ID:      a.ID,
		Title:   a.Title,
		Summary: summary,
		Body:    llm.PlainBody(bodyText, bodyHTML),
		URL:     a.URL,
		Author:  author,
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

// DailyDigest builds a Markdown digest of today's unread articles (Top N, default 12).
// locale controls output language (app UI language).
func (s *AIService) DailyDigest(limit int, locale string) (AIResult, error) {
	ctx := context.Background()
	if s.lib == nil {
		return AIResult{}, fmt.Errorf("library unavailable")
	}
	if limit <= 0 {
		limit = 12
	}
	if limit > 40 {
		limit = 40
	}
	// "today" collection already scopes to the current day; prefer unread among them.
	list, err := s.lib.ListArticles(ctx, "today", limit*3, 0, false)
	if err != nil {
		return AIResult{}, err
	}
	items := make([]llm.DigestItem, 0, limit)
	for _, a := range list {
		if a.IsRead {
			continue
		}
		sum := ""
		if a.Summary != nil {
			sum = *a.Summary
		}
		items = append(items, llm.DigestItem{Title: a.Title, Summary: sum, URL: a.URL})
		if len(items) >= limit {
			break
		}
	}
	// Fallback: if all of today's are read, use any today articles.
	if len(items) == 0 {
		for _, a := range list {
			sum := ""
			if a.Summary != nil {
				sum = *a.Summary
			}
			items = append(items, llm.DigestItem{Title: a.Title, Summary: sum, URL: a.URL})
			if len(items) >= limit {
				break
			}
		}
	}
	r, err := s.feat.Digest(ctx, items, locale)
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
