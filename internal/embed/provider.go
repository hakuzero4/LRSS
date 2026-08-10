package embed

import (
	"context"
	"errors"
	"fmt"

	"lrss/internal/settings"
)

// Sentinel errors.
var (
	ErrEmbeddingDisabled = errors.New("embedding disabled")
	ErrDimensionMismatch = errors.New("embedding dimension mismatch")
)

// Provider generates float32 embeddings.
type Provider interface {
	Name() string
	Dimensions() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// NewProvider constructs a provider from config.
func NewProvider(cfg settings.EmbeddingConfig) (Provider, error) {
	cfg = cfg.Normalize()
	if !cfg.IsConfigured() {
		return NewNoop(), nil
	}
	switch cfg.Provider {
	case settings.ProviderOpenAICompatible:
		return NewOpenAICompat(cfg)
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider)
	}
}
