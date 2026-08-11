package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"lrss/internal/settings"
)

// Chatter is the chat surface used by features (real Client or test double).
type Chatter interface {
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
	ModelName() string
}

// clientAdapter exposes ModelName on *Client.
type clientAdapter struct{ *Client }

func (a clientAdapter) ModelName() string {
	if a.Client == nil {
		return ""
	}
	return a.model
}

// NewChatter builds a Chatter from config.
func NewChatter(cfg settings.LLMConfig) (Chatter, error) {
	c, err := NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return clientAdapter{c}, nil
}

// FeatureResult is returned to appsvc / UI.
type FeatureResult struct {
	Markdown  string `json:"markdown"`
	Feature   string `json:"feature"`
	Model     string `json:"model"`
	Cached    bool   `json:"cached"`
	FolderID  string `json:"folderId,omitempty"`
	FolderName string `json:"folderName,omitempty"`
	// Verdict for classify: organic|promo|unclear
	Verdict string `json:"verdict,omitempty"`
}

// Service runs user-triggered LLM features with cache.
type Service struct {
	Store   *settings.Store
	Cache   *Cache
	// NewChatter defaults to NewChatter; inject in tests.
	NewChatter func(cfg settings.LLMConfig) (Chatter, error)
}

func (s *Service) chatter(cfg settings.LLMConfig) (Chatter, error) {
	fn := s.NewChatter
	if fn == nil {
		fn = NewChatter
	}
	return fn(cfg)
}

func (s *Service) loadCfg(ctx context.Context) (settings.LLMConfig, error) {
	if s == nil || s.Store == nil {
		return settings.LLMConfig{}, fmt.Errorf("llm not available")
	}
	cfg, err := s.Store.LoadLLMConfig(ctx)
	if err != nil {
		return settings.LLMConfig{}, err
	}
	if !cfg.IsConfigured() {
		return settings.LLMConfig{}, fmt.Errorf("llm not configured: enable a chat model in Settings → Search/AI")
	}
	return cfg, nil
}

func (s *Service) runCached(
	ctx context.Context,
	articleID, feature, extra string,
	hash string,
	system string,
	user string,
) (FeatureResult, error) {
	cfg, err := s.loadCfg(ctx)
	if err != nil {
		return FeatureResult{}, err
	}
	key := CacheKey(articleID, feature, cfg.Model, hash, extra)
	if s.Cache != nil {
		if hit, ok, err := s.Cache.Get(ctx, key); err != nil {
			return FeatureResult{}, err
		} else if ok && strings.TrimSpace(hit.ResultMD) != "" {
			return FeatureResult{
				Markdown: hit.ResultMD,
				Feature:  feature,
				Model:    hit.Model,
				Cached:   true,
			}, nil
		}
	}

	chat, err := s.chatter(cfg)
	if err != nil {
		return FeatureResult{}, err
	}
	res, err := chat.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return FeatureResult{}, err
	}
	md := strings.TrimSpace(res.Content)
	if md == "" {
		return FeatureResult{}, fmt.Errorf("llm returned empty content")
	}
	model := res.Model
	if model == "" {
		model = chat.ModelName()
	}
	if s.Cache != nil {
		_ = s.Cache.Put(ctx, CachedResult{
			Key: key, ArticleID: articleID, Feature: feature,
			Model: model, ContentHash: hash, ResultMD: md,
		})
	}
	return FeatureResult{
		Markdown: md,
		Feature:  feature,
		Model:    model,
		Cached:   false,
	}, nil
}

// Summarize generates an article summary.
func (s *Service) Summarize(ctx context.Context, a ArticleInput) (FeatureResult, error) {
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(a)
	return s.runCached(ctx, a.ID, FeatureSummarize, "", hash, SystemPromptFor(FeatureSummarize), UserPromptSummarize(bundle))
}

// Translate translates the article into targetLang (e.g. zh-CN, en).
func (s *Service) Translate(ctx context.Context, a ArticleInput, targetLang string) (FeatureResult, error) {
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = "zh-CN"
	}
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(a)
	return s.runCached(ctx, a.ID, FeatureTranslate, targetLang, hash, SystemPromptFor(FeatureTranslate), UserPromptTranslate(bundle, targetLang))
}

// Ask answers a question about the article.
func (s *Service) Ask(ctx context.Context, a ArticleInput, question string) (FeatureResult, error) {
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(a)
	extra := ContentFingerprint(ArticleInput{Body: question}) // reuse hash helper for question
	return s.runCached(ctx, a.ID, FeatureAsk, extra, hash, SystemPromptFor(FeatureAsk), UserPromptAsk(bundle, question))
}

// Digest builds a daily digest from items (already Top N).
func (s *Service) Digest(ctx context.Context, items []DigestItem) (FeatureResult, error) {
	if len(items) == 0 {
		return FeatureResult{}, fmt.Errorf("no unread articles for today")
	}
	// Cap body size per item via UserPromptDigest's BudgetText.
	hash := DigestFingerprint(items, len(items))
	return s.runCached(ctx, "", FeatureDigest, fmt.Sprintf("n=%d", len(items)), hash, SystemPromptFor(FeatureDigest), UserPromptDigest(items))
}

// FolderRef is a minimal folder for suggestions.
type FolderRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Suggest returns tags/folder suggestion Markdown.
// When LLM is configured it calls the model; local tags are always prepended as a note if LLM fails... 
// Actually plan says LLM and/or local-rule path. We always run local tags; if LLM configured, enhance.
func (s *Service) Suggest(ctx context.Context, a ArticleInput, folders []FolderRef) (FeatureResult, error) {
	localTags := LocalSuggestTags(a.Title, a.Summary, 6)
	localMD := FormatLocalSuggestMarkdown(localTags, "Unfiled")

	cfg, err := s.loadCfg(ctx)
	if err != nil {
		// Local-only path when LLM off.
		return FeatureResult{
			Markdown: localMD,
			Feature:  FeatureSuggest,
			Model:    "local",
			Cached:   false,
		}, nil
	}

	names := make([]string, 0, len(folders))
	for _, f := range folders {
		if strings.TrimSpace(f.Name) != "" {
			names = append(names, f.Name)
		}
	}
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(a)
	extra := strings.Join(names, ",")
	res, err := s.runCached(ctx, a.ID, FeatureSuggest, extra, hash, SystemPromptFor(FeatureSuggest), UserPromptSuggest(bundle, names))
	if err != nil {
		return FeatureResult{}, err
	}
	// Attach best folder match from model text.
	id, name := MatchFolderName(res.Markdown, folders)
	res.FolderID = id
	res.FolderName = name
	// Prepend local tags section for transparency.
	res.Markdown = "### Local tags\n" + strings.TrimPrefix(localMD, "## Tags\n") + "\n---\n\n" + res.Markdown
	_ = cfg
	return res, nil
}

// ClassifyPromo classifies ads/soft-promo.
func (s *Service) ClassifyPromo(ctx context.Context, a ArticleInput) (FeatureResult, error) {
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(a)
	res, err := s.runCached(ctx, a.ID, FeatureClassify, "", hash, SystemPromptFor(FeatureClassify), UserPromptClassify(bundle))
	if err != nil {
		return FeatureResult{}, err
	}
	res.Verdict = parseVerdict(res.Markdown)
	meta, _ := json.Marshal(map[string]string{"verdict": res.Verdict})
	if s.Cache != nil && !res.Cached {
		// refresh meta if we just wrote — best effort
		key := CacheKey(a.ID, FeatureClassify, res.Model, hash, "")
		_ = s.Cache.Put(ctx, CachedResult{
			Key: key, ArticleID: a.ID, Feature: FeatureClassify,
			Model: res.Model, ContentHash: hash, ResultMD: res.Markdown, MetaJSON: string(meta),
		})
	}
	return res, nil
}

func parseVerdict(md string) string {
	low := strings.ToLower(md)
	switch {
	case strings.Contains(low, "verdict:** promo") || strings.Contains(low, "verdict: promo") ||
		strings.Contains(low, "**verdict:** promo"):
		return "promo"
	case strings.Contains(low, "verdict:** organic") || strings.Contains(low, "verdict: organic"):
		return "organic"
	case strings.Contains(low, "promo") && strings.Contains(low, "verdict"):
		if strings.Contains(low, "organic") && strings.Index(low, "organic") < strings.Index(low, "promo") {
			return "organic"
		}
		return "promo"
	case strings.Contains(low, "organic") && strings.Contains(low, "verdict"):
		return "organic"
	default:
		return "unclear"
	}
}
