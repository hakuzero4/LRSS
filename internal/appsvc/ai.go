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

// Summarize generates a Markdown summary for an article.
func (s *AIService) Summarize(articleId string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	r, err := s.feat.Summarize(ctx, in)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// Translate translates an article into targetLang (e.g. zh-CN, en).
func (s *AIService) Translate(articleId, targetLang string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	r, err := s.feat.Translate(ctx, in, targetLang)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// Ask answers a question about the article (empty question → default overview).
func (s *AIService) Ask(articleId, question string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	r, err := s.feat.Ask(ctx, in, question)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// DailyDigest builds a Markdown digest of today's unread articles (Top N, default 12).
func (s *AIService) DailyDigest(limit int) (AIResult, error) {
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
	r, err := s.feat.Digest(ctx, items)
	if err != nil {
		return AIResult{}, err
	}
	return toAIResult(r), nil
}

// SuggestFolders returns tag/folder suggestions for an article.
// When LLM is off, returns local-rule tags only.
func (s *AIService) SuggestFolders(articleId string) (AIResult, error) {
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
	r, err := s.feat.Suggest(ctx, in, folders)
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
func (s *AIService) ClassifyPromo(articleId string) (AIResult, error) {
	ctx := context.Background()
	in, err := s.articleInput(ctx, articleId)
	if err != nil {
		return AIResult{}, err
	}
	r, err := s.feat.ClassifyPromo(ctx, in)
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
