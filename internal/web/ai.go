package web

// AIResult mirrors appsvc.AIResult JSON for browser clients.
type AIResult struct {
	Markdown        string `json:"markdown"`
	Feature         string `json:"feature"`
	Model           string `json:"model"`
	Cached          bool   `json:"cached"`
	FolderID        string `json:"folderId,omitempty"`
	FolderName      string `json:"folderName,omitempty"`
	Verdict         string `json:"verdict,omitempty"`
	KeptOriginal    bool   `json:"keptOriginal,omitempty"`
	TranslationRaw  string `json:"translationRaw,omitempty"`
	TranslationLang string `json:"translationLang,omitempty"`
}

// AI is the optional LLM facade used by web API handlers.
// Implemented by appsvc.WebAIBridge (avoids import cycle).
type AI interface {
	Summarize(articleId, locale string) (AIResult, error)
	Translate(articleId, targetLang string) (AIResult, error)
	TranslateSelection(text, targetLang string) (AIResult, error)
	Ask(articleId, question, locale string) (AIResult, error)
	SuggestFolders(articleId, locale string) (AIResult, error)
	ClassifyPromo(articleId, locale string) (AIResult, error)
	DetectContentFullness(articleId string) (AIResult, error)
	EnsureFullContent(articleId string) (AIResult, error)
	IsLLMConfigured() (bool, error)
	ClearTranslation(articleId string) error
	ApplySuggestedFolder(articleId, folderId string) error
}
