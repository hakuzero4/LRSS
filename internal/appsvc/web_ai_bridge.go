package appsvc

import "lrss/internal/web"

// WebAIBridge adapts AIService to web.AI (no import cycle with handler package).
type WebAIBridge struct {
	S *AIService
}

// NewWebAI returns a web.AI from the Wails AI service (nil-safe).
func NewWebAI(s *AIService) web.AI {
	if s == nil {
		return nil
	}
	return WebAIBridge{S: s}
}

func mapAIResult(r AIResult) web.AIResult {
	return web.AIResult{
		Markdown:        r.Markdown,
		Feature:         r.Feature,
		Model:           r.Model,
		Cached:          r.Cached,
		FolderID:        r.FolderID,
		FolderName:      r.FolderName,
		Verdict:         r.Verdict,
		KeptOriginal:    r.KeptOriginal,
		TranslationRaw:  r.TranslationRaw,
		TranslationLang: r.TranslationLang,
	}
}

func (b WebAIBridge) Summarize(articleId, locale string) (web.AIResult, error) {
	r, err := b.S.Summarize(articleId, locale)
	return mapAIResult(r), err
}

func (b WebAIBridge) Translate(articleId, targetLang string) (web.AIResult, error) {
	r, err := b.S.Translate(articleId, targetLang)
	return mapAIResult(r), err
}

func (b WebAIBridge) TranslateSelection(text, targetLang string) (web.AIResult, error) {
	r, err := b.S.TranslateSelection(text, targetLang)
	return mapAIResult(r), err
}

func (b WebAIBridge) Ask(articleId, question, locale string) (web.AIResult, error) {
	r, err := b.S.Ask(articleId, question, locale)
	return mapAIResult(r), err
}

func (b WebAIBridge) SuggestFolders(articleId, locale string) (web.AIResult, error) {
	r, err := b.S.SuggestFolders(articleId, locale)
	return mapAIResult(r), err
}

func (b WebAIBridge) ClassifyPromo(articleId, locale string) (web.AIResult, error) {
	r, err := b.S.ClassifyPromo(articleId, locale)
	return mapAIResult(r), err
}

func (b WebAIBridge) DetectContentFullness(articleId string) (web.AIResult, error) {
	r, err := b.S.DetectContentFullness(articleId)
	return mapAIResult(r), err
}

func (b WebAIBridge) EnsureFullContent(articleId string) (web.AIResult, error) {
	r, err := b.S.EnsureFullContent(articleId)
	return mapAIResult(r), err
}

func (b WebAIBridge) IsLLMConfigured() (bool, error) {
	return b.S.IsLLMConfigured()
}

func (b WebAIBridge) ClearTranslation(articleId string) error {
	return b.S.ClearTranslation(articleId)
}

func (b WebAIBridge) ApplySuggestedFolder(articleId, folderId string) error {
	return b.S.ApplySuggestedFolder(articleId, folderId)
}
