package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lrss/internal/llm"
	"lrss/internal/settings"
)

func TestChat_OpenAICompatible(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "gpt-test" {
			t.Errorf("model = %v", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gpt-test",
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"role":    "assistant",
						"content": "OK",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{"prompt_tokens": 5, "completion_tokens": 1},
		})
	}))
	t.Cleanup(srv.Close)

	cli, err := llm.NewClient(settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  srv.URL + "/v1",
		APIKey:   "test-key",
		Model:    "gpt-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Inject test server client (surf not needed for loopback).
	// NewClient already has httpx client; point BaseURL at httptest — works.

	res, err := cli.Chat(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "OK" {
		t.Fatalf("content = %q", res.Content)
	}
	if res.PromptTokens != 5 {
		t.Fatalf("tokens = %d", res.PromptTokens)
	}
}

func TestTestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "OK"}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	cli, err := llm.NewClient(settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  srv.URL + "/v1",
		Model:    "m",
	})
	if err != nil {
		t.Fatal(err)
	}
	msg, err := cli.TestConnection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "OK") {
		t.Fatalf("msg = %q", msg)
	}
}

func TestNewClient_NotConfigured(t *testing.T) {
	_, err := llm.NewClient(settings.DefaultLLMConfig())
	if err == nil {
		t.Fatal("expected error")
	}
}
