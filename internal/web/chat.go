package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"lrss/internal/model"
)

func (s *Server) handleAIChatHistory(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	articleID := strings.TrimSpace(r.URL.Query().Get("articleId"))
	res, err := ai.ChatHistory(articleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if res.Messages == nil {
		res.Messages = []model.ChatMessage{}
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAIChatSend(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body ChatSendRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	res, err := ai.ChatSend(body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleAIChatClear(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		ArticleID string `json:"articleId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := ai.ChatClear(body.ArticleID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAIChatCancel(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	var body struct {
		SessionID string `json:"sessionId"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if err := ai.ChatCancel(body.SessionID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAIStream(w http.ResponseWriter, r *http.Request) {
	ai := s.requireAI(w)
	if ai == nil {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "stream unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, ": ok\n\n")
	flusher.Flush()

	ch := make(chan AIStreamEvent, 32)
	unsub := ai.SubscribeStream(func(ev AIStreamEvent) {
		select {
		case ch <- ev:
		default:
		}
	})
	defer unsub()

	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-tick.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}
