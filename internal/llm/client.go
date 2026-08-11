// Package llm provides OpenAI-compatible chat completions via httpx (surf).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lrss/internal/httpx"
	"lrss/internal/settings"
)

const (
	defaultTimeout = 60 * time.Second
	defaultUA      = "LRSS/0.1 (+https://local; llm)"
)

// Message is a chat message.
type Message struct {
	Role    string `json:"role"` // system|user|assistant
	Content string `json:"content"`
}

// ChatRequest is a high-level chat call.
type ChatRequest struct {
	Messages    []Message
	Temperature *float64
	MaxTokens   *int
}

// ChatResponse is a simplified completion result.
type ChatResponse struct {
	Content      string
	Model        string
	FinishReason string
	// Raw usage if the provider returned it.
	PromptTokens     int
	CompletionTokens int
}

// Client talks to an OpenAI-compatible /chat/completions endpoint.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	temperature float64
	maxTokens  int
	system     string
	http       *http.Client
}

// NewClient builds a client from LLMConfig. Config must be configured.
func NewClient(cfg settings.LLMConfig) (*Client, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.IsConfigured() {
		return nil, fmt.Errorf("llm: not configured")
	}
	return &Client{
		baseURL:     cfg.BaseURL,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
		system:      cfg.SystemPrompt,
		http: httpx.Std(httpx.Options{
			Timeout:   defaultTimeout,
			UserAgent: defaultUA,
		}),
	}, nil
}

// Chat runs a non-streaming chat completion.
func (c *Client) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	if c == nil {
		return ChatResponse{}, fmt.Errorf("llm: nil client")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	msgs := make([]Message, 0, len(req.Messages)+1)
	if c.system != "" {
		// Only inject default system if caller did not already send one.
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
		"stream":      false,
	}
	if maxTok > 0 {
		body["max_tokens"] = maxTok
	}

	raw, err := c.postJSON(ctx, "/chat/completions", body)
	if err != nil {
		return ChatResponse{}, err
	}

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
	if content == "" && choice.Text != "" {
		content = strings.TrimSpace(choice.Text)
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

// TestConnection sends a minimal completion to verify credentials / model.
func (c *Client) TestConnection(ctx context.Context) (string, error) {
	maxTok := 16
	temp := 0.0
	res, err := c.Chat(ctx, ChatRequest{
		Messages: []Message{
			{Role: "user", Content: "Reply with exactly: OK"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTok,
	})
	if err != nil {
		return "", err
	}
	msg := res.Content
	if msg == "" {
		msg = "ok"
	}
	// Keep short for UI toast.
	if len(msg) > 120 {
		msg = msg[:117] + "…"
	}
	return msg, nil
}

func (c *Client) postJSON(ctx context.Context, path string, payload any) ([]byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := joinURL(c.baseURL, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("llm: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUA)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm: fetch: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("llm: read: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("llm: http %d: %s", resp.StatusCode, msg)
	}
	return body, nil
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	// If base already ends with /v1, append /chat/completions; if user put full path, still join.
	return base + path
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

type chatCompletionsResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Index        int    `json:"index"`
		FinishReason string `json:"finish_reason"`
		// Chat style
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		// Some proxies use text
		Text string `json:"text"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}
