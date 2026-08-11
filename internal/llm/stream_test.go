package llm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lrss/internal/llm"
	"lrss/internal/settings"
)

func TestChatStream_SSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{"Hello", " ", "world"}
		for _, c := range chunks {
			payload, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{
					{"delta": map[string]string{"content": c}},
				},
			})
			fmt.Fprintf(w, "data: %s\n\n", payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)

	cli, err := llm.NewClient(settings.LLMConfig{
		Provider: settings.LLMProviderOpenAICompatible,
		BaseURL:  srv.URL + "/v1",
		Model:    "m",
		APIKey:   "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	res, err := cli.ChatStream(context.Background(), llm.ChatRequest{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(delta, full string) {
		got = append(got, delta)
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "Hello world" {
		t.Fatalf("content = %q", res.Content)
	}
	if strings.Join(got, "") != "Hello world" {
		t.Fatalf("deltas = %v", got)
	}
}
