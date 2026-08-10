package embed

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
)

// FakeProvider produces deterministic unit vectors for tests (no network).
type FakeProvider struct {
	Dim int
}

// NewFake returns a fake provider with the given dimension.
func NewFake(dim int) *FakeProvider {
	if dim <= 0 {
		dim = 8
	}
	return &FakeProvider{Dim: dim}
}

func (f *FakeProvider) Name() string    { return "fake" }
func (f *FakeProvider) Dimensions() int { return f.Dim }

func (f *FakeProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = fakeVector(t, f.Dim)
	}
	return out, nil
}

func fakeVector(text string, dim int) []float32 {
	sum := sha256.Sum256([]byte(text))
	v := make([]float32, dim)
	var norm float64
	for i := 0; i < dim; i++ {
		// Expand hash stream
		b := sum[i%len(sum)] ^ byte(i*17)
		f := float64(b)/127.5 - 1 // [-1,1]
		v[i] = float32(f)
		norm += f * f
	}
	norm = math.Sqrt(norm)
	if norm < 1e-9 {
		v[0] = 1
		return v
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return v
}

// MustEmbed is a helper for tests.
func (f *FakeProvider) MustEmbed(text string) []float32 {
	vs, err := f.Embed(context.Background(), []string{text})
	if err != nil || len(vs) != 1 {
		panic(fmt.Sprintf("fake embed: %v", err))
	}
	return vs[0]
}
