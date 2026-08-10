package embed

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

// OpenAICompatProvider calls OpenAI-compatible /embeddings endpoints.
type OpenAICompatProvider struct {
	baseURL    string
	apiKey     string
	model      string
	dimensions int
	batchSize  int
	client     *http.Client
}

// NewOpenAICompat creates a provider from config.
func NewOpenAICompat(cfg settings.EmbeddingConfig) (*OpenAICompatProvider, error) {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &OpenAICompatProvider{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		batchSize:  cfg.BatchSize,
		client: httpx.Std(httpx.Options{
			Timeout: 30 * time.Second,
		}),
	}, nil
}

func (p *OpenAICompatProvider) Name() string    { return "openai_compatible" }
func (p *OpenAICompatProvider) Dimensions() int { return p.dimensions }

func (p *OpenAICompatProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float32, 0, len(texts))
	bs := p.batchSize
	if bs <= 0 {
		bs = 16
	}
	for i := 0; i < len(texts); i += bs {
		end := i + bs
		if end > len(texts) {
			end = len(texts)
		}
		batch, err := p.embedBatch(ctx, texts[i:end])
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
	}
	return out, nil
}

type embeddingsRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingsResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *OpenAICompatProvider) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embeddingsRequest{Model: p.model, Input: texts})
	if err != nil {
		return nil, err
	}
	url := p.baseURL + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	res, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings request: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))

	var parsed embeddingsResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("embeddings decode: %w (status %d)", err, res.StatusCode)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return nil, fmt.Errorf("embeddings api: %s", parsed.Error.Message)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("embeddings status %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings count mismatch: got %d want %d", len(parsed.Data), len(texts))
	}

	// Order by index if present.
	byIdx := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			return nil, fmt.Errorf("invalid embedding index %d", d.Index)
		}
		if len(d.Embedding) != p.dimensions {
			return nil, fmt.Errorf("%w: got %d want %d", ErrDimensionMismatch, len(d.Embedding), p.dimensions)
		}
		vec := make([]float32, len(d.Embedding))
		for i, f := range d.Embedding {
			vec[i] = float32(f)
		}
		byIdx[d.Index] = vec
	}
	for i, v := range byIdx {
		if v == nil {
			return nil, fmt.Errorf("missing embedding for index %d", i)
		}
	}
	return byIdx, nil
}
