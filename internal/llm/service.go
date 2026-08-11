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

// clientAdapter exposes ModelName + streaming on *Client.
type clientAdapter struct{ *Client }

func (a clientAdapter) ModelName() string {
	if a.Client == nil {
		return ""
	}
	return a.model
}

func (a clientAdapter) ChatStream(ctx context.Context, req ChatRequest, onChunk StreamHandler) (ChatResponse, error) {
	return a.Client.ChatStream(ctx, req, onChunk)
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

// Summarize generates an article summary. locale is the app UI language (e.g. zh-CN).
func (s *Service) Summarize(ctx context.Context, a ArticleInput, locale string) (FeatureResult, error) {
	return s.SummarizeStream(ctx, a, locale, nil)
}

// SummarizeStream is like Summarize but streams tokens via onChunk when not cache-hit.
func (s *Service) SummarizeStream(ctx context.Context, a ArticleInput, locale string, onChunk StreamHandler) (FeatureResult, error) {
	// Exclude Summary from fingerprint — we overwrite it with the AI result.
	hashInput := ArticleInput{Title: a.Title, Body: a.Body, URL: a.URL, Author: a.Author}
	// Prefer body for generation; still pass summary into the model as context only.
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(hashInput)
	loc := NormalizeUILocale(locale)
	extra := "loc=" + loc + "|deck=1"
	feature := FeatureSummarize
	system := SystemPromptFor(feature, locale)
	user := UserPromptSummarize(bundle, locale)

	cfg, err := s.loadCfg(ctx)
	if err != nil {
		return FeatureResult{}, err
	}
	key := CacheKey(a.ID, feature, cfg.Model, hash, extra)
	if s.Cache != nil {
		if hit, ok, err := s.Cache.Get(ctx, key); err != nil {
			return FeatureResult{}, err
		} else if ok && strings.TrimSpace(hit.ResultMD) != "" {
			if onChunk != nil {
				onChunk(hit.ResultMD, hit.ResultMD)
			}
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

	// Prefer streaming when underlying client supports it.
	var res ChatResponse
	if sc, ok := chat.(streamChatter); ok {
		res, err = sc.ChatStream(ctx, ChatRequest{
			Messages: []Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		}, onChunk)
	} else {
		res, err = chat.Chat(ctx, ChatRequest{
			Messages: []Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		})
		if err == nil && onChunk != nil && res.Content != "" {
			onChunk(res.Content, res.Content)
		}
	}
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
			Key: key, ArticleID: a.ID, Feature: feature,
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

// streamChatter is optional streaming capability.
type streamChatter interface {
	Chatter
	ChatStream(ctx context.Context, req ChatRequest, onChunk StreamHandler) (ChatResponse, error)
}

// Translate translates the article into targetLang (e.g. zh-CN, en).
func (s *Service) Translate(ctx context.Context, a ArticleInput, targetLang string) (FeatureResult, error) {
	return s.TranslateStream(ctx, a, targetLang, nil)
}

// TranslateStream streams bilingual marker output for interlinear UI.
func (s *Service) TranslateStream(ctx context.Context, a ArticleInput, targetLang string, onChunk StreamHandler) (FeatureResult, error) {
	targetLang = strings.TrimSpace(targetLang)
	if targetLang == "" {
		targetLang = "zh-CN"
	}
	// Fingerprint without summary noise; include target lang in extra.
	hashInput := ArticleInput{Title: a.Title, Body: a.Body, URL: a.URL, Author: a.Author}
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(hashInput)
	extra := "bilingual=1|lang=" + NormalizeUILocale(targetLang) + "|" + strings.TrimSpace(targetLang)
	feature := FeatureTranslate
	system := SystemPromptFor(feature, targetLang)
	user := UserPromptTranslate(bundle, targetLang)

	cfg, err := s.loadCfg(ctx)
	if err != nil {
		return FeatureResult{}, err
	}
	key := CacheKey(a.ID, feature, cfg.Model, hash, extra)
	if s.Cache != nil {
		if hit, ok, err := s.Cache.Get(ctx, key); err != nil {
			return FeatureResult{}, err
		} else if ok && strings.TrimSpace(hit.ResultMD) != "" {
			if onChunk != nil {
				onChunk(hit.ResultMD, hit.ResultMD)
			}
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
	var res ChatResponse
	if sc, ok := chat.(streamChatter); ok {
		res, err = sc.ChatStream(ctx, ChatRequest{
			Messages: []Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		}, onChunk)
	} else {
		res, err = chat.Chat(ctx, ChatRequest{
			Messages: []Message{
				{Role: "system", Content: system},
				{Role: "user", Content: user},
			},
		})
		if err == nil && onChunk != nil && res.Content != "" {
			onChunk(res.Content, res.Content)
		}
	}
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
			Key: key, ArticleID: a.ID, Feature: feature,
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

// Ask answers a question about the article.
func (s *Service) Ask(ctx context.Context, a ArticleInput, question, locale string) (FeatureResult, error) {
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(a)
	extra := ContentFingerprint(ArticleInput{Body: question}) + "|loc=" + NormalizeUILocale(locale)
	return s.runCached(ctx, a.ID, FeatureAsk, extra, hash, SystemPromptFor(FeatureAsk, locale), UserPromptAsk(bundle, question, locale))
}

// Digest builds a daily digest from items (already Top N).
func (s *Service) Digest(ctx context.Context, items []DigestItem, locale string) (FeatureResult, error) {
	if len(items) == 0 {
		return FeatureResult{}, fmt.Errorf("no unread articles for today")
	}
	// Cap body size per item via UserPromptDigest's BudgetText.
	hash := DigestFingerprint(items, len(items))
	loc := NormalizeUILocale(locale)
	return s.runCached(ctx, "", FeatureDigest, fmt.Sprintf("n=%d|loc=%s", len(items), loc), hash, SystemPromptFor(FeatureDigest, locale), UserPromptDigest(items, locale))
}

// FolderRef is a minimal folder for suggestions.
type FolderRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Suggest returns tags/folder suggestion Markdown.
// Local tags always available; LLM enhances when configured.
func (s *Service) Suggest(ctx context.Context, a ArticleInput, folders []FolderRef, locale string) (FeatureResult, error) {
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
	extra := strings.Join(names, ",") + "|loc=" + NormalizeUILocale(locale)
	res, err := s.runCached(ctx, a.ID, FeatureSuggest, extra, hash, SystemPromptFor(FeatureSuggest, locale), UserPromptSuggest(bundle, names, locale))
	if err != nil {
		return FeatureResult{}, err
	}
	// Attach best folder match from model text.
	id, name := MatchFolderName(res.Markdown, folders)
	res.FolderID = id
	res.FolderName = name
	// Prepend local tags section for transparency.
	if NormalizeUILocale(locale) == "zh" {
		res.Markdown = "### 本地标签\n" + strings.TrimPrefix(localMD, "## Tags\n") + "\n---\n\n" + res.Markdown
	} else {
		res.Markdown = "### Local tags\n" + strings.TrimPrefix(localMD, "## Tags\n") + "\n---\n\n" + res.Markdown
	}
	_ = cfg
	return res, nil
}

// ClassifyPromo classifies ads/soft-promo.
func (s *Service) ClassifyPromo(ctx context.Context, a ArticleInput, locale string) (FeatureResult, error) {
	bundle := BuildArticleBundle(a, DefaultMaxInputChars)
	hash := ContentFingerprint(a)
	loc := NormalizeUILocale(locale)
	res, err := s.runCached(ctx, a.ID, FeatureClassify, "loc="+loc, hash, SystemPromptFor(FeatureClassify, locale), UserPromptClassify(bundle, locale))
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
