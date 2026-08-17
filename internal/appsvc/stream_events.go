package appsvc

import (
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// LLMStreamEvent is emitted to the frontend during streaming AI features.
type LLMStreamEvent struct {
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

const EventLLMStream = "llm:stream"

// StreamListener receives llm:stream events (desktop Wails + web SSE).
type StreamListener func(LLMStreamEvent)

var (
	streamMu        sync.RWMutex
	streamListeners = map[int]StreamListener{}
	streamNextID    int
)

func init() {
	application.RegisterEvent[LLMStreamEvent](EventLLMStream)
}

// SubscribeLLMStream registers a listener for in-process stream events.
// Used by web access SSE so the browser can follow the same tokens as the desktop WebView.
func SubscribeLLMStream(fn StreamListener) (unsubscribe func()) {
	if fn == nil {
		return func() {}
	}
	streamMu.Lock()
	streamNextID++
	id := streamNextID
	streamListeners[id] = fn
	streamMu.Unlock()
	return func() {
		streamMu.Lock()
		delete(streamListeners, id)
		streamMu.Unlock()
	}
}

// emitLLMStream best-effort emits a stream event (Wails no-op if app not ready / tests).
func emitLLMStream(ev LLMStreamEvent) {
	if app := application.Get(); app != nil {
		app.Event.Emit(EventLLMStream, ev)
	}
	streamMu.RLock()
	fns := make([]StreamListener, 0, len(streamListeners))
	for _, fn := range streamListeners {
		fns = append(fns, fn)
	}
	streamMu.RUnlock()
	for _, fn := range fns {
		fn(ev)
	}
}
