package embed

import "context"

// NoopProvider is used when embedding is not configured.
type NoopProvider struct{}

// NewNoop returns a disabled provider.
func NewNoop() *NoopProvider { return &NoopProvider{} }

func (NoopProvider) Name() string      { return "noop" }
func (NoopProvider) Dimensions() int   { return 0 }

func (NoopProvider) Embed(context.Context, []string) ([][]float32, error) {
	return nil, ErrEmbeddingDisabled
}
