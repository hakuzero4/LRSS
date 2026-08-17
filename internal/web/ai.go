package web

import "lrss/internal/model"

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

// ChatSendRequest mirrors appsvc.ChatSendRequest for the browser API.
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

// ChatHistoryResult is the persisted transcript for an article (or the library session).
type ChatHistoryResult struct {
	SessionID string              `json:"sessionId"`
	ArticleID string              `json:"articleId"`
	Messages  []model.ChatMessage `json:"messages"`
}

// AIStreamEvent mirrors appsvc.LLMStreamEvent for browser SSE clients.
type AIStreamEvent struct {
	ArticleID string `json:"articleId"`
	SessionID string `json:"sessionId,omitempty"`
	Feature   string `json:"feature"`
	Delta     string `json:"delta"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
	Model     string `json:"model,omitempty"`
	Cached    bool   `json:"cached,omitempty"`
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
	ChatHistory(articleId string) (ChatHistoryResult, error)
	ChatClear(articleId string) error
	ChatCancel(sessionId string) error
	ChatSend(req ChatSendRequest) (ChatSendResult, error)
	SubscribeStream(fn func(AIStreamEvent)) (unsubscribe func())
}
