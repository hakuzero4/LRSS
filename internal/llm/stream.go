package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// StreamHandler receives incremental chat tokens.
// delta is the new piece; full is the cumulative text so far.
type StreamHandler func(delta, full string)

// ChatStream runs a streaming chat completion (SSE).
// Falls back to non-stream Chat if the provider rejects stream.
func (c *Client) ChatStream(ctx context.Context, req ChatRequest, onChunk StreamHandler) (ChatResponse, error) {
	if c == nil {
		return ChatResponse{}, fmt.Errorf("llm: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	msgs := make([]Message, 0, len(req.Messages)+1)
	if c.system != "" {
		hasSystem := false
		for _, m := range req.Messages {
			if m.Role == "system" {
				hasSystem = true
				break
			}
		}
		if !hasSystem {
			msgs = append(msgs, Message{Role: "system", Content: c.system})
		}
	}
	for _, m := range req.Messages {
		role := strings.TrimSpace(m.Role)
		if role == "" {
			role = "user"
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		msgs = append(msgs, Message{Role: role, Content: content})
	}
	if len(msgs) == 0 {
		return ChatResponse{}, fmt.Errorf("llm: empty messages")
	}

	temp := c.temperature
	if req.Temperature != nil {
		temp = *req.Temperature
	}
	maxTok := c.maxTokens
	if req.MaxTokens != nil {
		maxTok = *req.MaxTokens
	}

	body := map[string]any{
		"model":       c.model,
		"messages":    msgs,
		"temperature": temp,
		"stream":      true,
	}
	if maxTok > 0 {
		body["max_tokens"] = maxTok
	}

	rawBody, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, err
	}
	endpoint := joinURL(c.baseURL, "/chat/completions")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return ChatResponse{}, fmt.Errorf("llm: request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("User-Agent", defaultUA)
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("llm: fetch: %w", err)
	}
	defer resp.Body.Close()

	// Some proxies return JSON error with 200 or reject stream — fallback.
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		// Fallback to non-stream on failure.
		if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound {
			return c.chatStreamFallback(ctx, req, onChunk)
		}
		msg := strings.TrimSpace(string(b))
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		return ChatResponse{}, fmt.Errorf("llm: http %d: %s", resp.StatusCode, msg)
	}

	// Non-SSE JSON body (some providers ignore stream:true, or omit content-type).
	if strings.Contains(ct, "application/json") && !strings.Contains(ct, "event-stream") {
		b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			return ChatResponse{}, err
		}
		return c.parseAndEmitFull(b, onChunk)
	}

	// Peek first non-empty line: JSON object ⇒ full response; "data:" ⇒ SSE.
	br := bufio.NewReader(resp.Body)
	first, err := peekFirstLine(br)
	if err != nil {
		return ChatResponse{}, fmt.Errorf("llm: stream peek: %w", err)
	}
	trimmed := strings.TrimSpace(first)
	if trimmed != "" && !strings.HasPrefix(trimmed, "data:") && strings.HasPrefix(trimmed, "{") {
		rest, _ := io.ReadAll(io.LimitReader(br, 4<<20))
		fullBody := append([]byte(first), rest...)
		// first line may not include rest of JSON if multi-line
		if !json.Valid(fullBody) {
			// Read was partial — prepend and continue
			fullBody = append([]byte(first+"\n"), rest...)
		}
		return c.parseAndEmitFull(fullBody, onChunk)
	}

	// Reconstruct stream: first line already read.
	streamReader := io.MultiReader(strings.NewReader(first+"\n"), br)

	var full strings.Builder
	var model, finish string
	sc := bufio.NewScanner(streamReader)
	// Increase buffer for long lines.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Model != "" {
			model = chunk.Model
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return ChatResponse{}, fmt.Errorf("llm: %s", chunk.Error.Message)
		}
		for _, ch := range chunk.Choices {
			if ch.FinishReason != "" {
				finish = ch.FinishReason
			}
			delta := ch.Delta.Content
			if delta == "" && ch.Text != "" {
				delta = ch.Text
			}
			if delta == "" {
				continue
			}
			full.WriteString(delta)
			if onChunk != nil {
				onChunk(delta, full.String())
			}
		}
	}
	if err := sc.Err(); err != nil {
		return ChatResponse{}, fmt.Errorf("llm: stream read: %w", err)
	}

	text := strings.TrimSpace(full.String())
	if text == "" {
		// Empty stream — try non-stream fallback once.
		return c.chatStreamFallback(ctx, req, onChunk)
	}
	return ChatResponse{
		Content:      text,
		Model:        firstNonEmpty(model, c.model),
		FinishReason: finish,
	}, nil
}

func (c *Client) chatStreamFallback(ctx context.Context, req ChatRequest, onChunk StreamHandler) (ChatResponse, error) {
	res, err := c.Chat(ctx, req)
	if err != nil {
		return ChatResponse{}, err
	}
	if onChunk != nil && res.Content != "" {
		onChunk(res.Content, res.Content)
	}
	return res, nil
}

func (c *Client) parseAndEmitFull(raw []byte, onChunk StreamHandler) (ChatResponse, error) {
	var parsed chatCompletionsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ChatResponse{}, fmt.Errorf("llm: decode: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return ChatResponse{}, fmt.Errorf("llm: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return ChatResponse{}, fmt.Errorf("llm: empty choices")
	}
	choice := parsed.Choices[0]
	content := strings.TrimSpace(choice.Message.Content)
	if content == "" {
		content = strings.TrimSpace(choice.Text)
	}
	if onChunk != nil && content != "" {
		onChunk(content, content)
	}
	out := ChatResponse{
		Content:      content,
		Model:        firstNonEmpty(parsed.Model, c.model),
		FinishReason: choice.FinishReason,
	}
	if parsed.Usage != nil {
		out.PromptTokens = parsed.Usage.PromptTokens
		out.CompletionTokens = parsed.Usage.CompletionTokens
	}
	return out, nil
}

func peekFirstLine(br *bufio.Reader) (string, error) {
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		// Keep newline stripped for inspection; empty lines skip.
		s := strings.TrimRight(line, "\r\n")
		if s != "" || err == io.EOF {
			return s, nil
		}
		if err == io.EOF {
			return "", io.EOF
		}
	}
}

type streamChunk struct {
	Model   string `json:"model"`
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
			Role    string `json:"role"`
		} `json:"delta"`
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}
