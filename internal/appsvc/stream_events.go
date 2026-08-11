package appsvc

import (
	"github.com/wailsapp/wails/v3/pkg/application"
)

// LLMStreamEvent is emitted to the frontend during streaming AI features.
type LLMStreamEvent struct {
	ArticleID string `json:"articleId"`
	Feature   string `json:"feature"`
	Delta     string `json:"delta"`
	Text      string `json:"text"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
	Model     string `json:"model,omitempty"`
	Cached    bool   `json:"cached,omitempty"`
}

const EventLLMStream = "llm:stream"

func init() {
	application.RegisterEvent[LLMStreamEvent](EventLLMStream)
}

// emitLLMStream best-effort emits a stream event (no-op if app not ready / tests).
func emitLLMStream(ev LLMStreamEvent) {
	app := application.Get()
	if app == nil {
		return
	}
	app.Event.Emit(EventLLMStream, ev)
}
