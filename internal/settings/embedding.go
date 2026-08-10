package settings

import (
	"fmt"
	"net/url"
	"strings"
)

// Provider kinds for embedding.
const (
	ProviderDisabled        = "disabled"
	ProviderOpenAICompatible = "openai_compatible"
)

// Search modes.
const (
	SearchModeAuto   = "auto"
	SearchModeFTS    = "fts"
	SearchModeVector = "vector"
	SearchModeHybrid = "hybrid"
)

// EmbeddingConfig is stored under settings keys embedding.*.
type EmbeddingConfig struct {
	Provider   string `json:"provider"`
	BaseURL    string `json:"baseUrl"`
	APIKey     string `json:"apiKey"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
	BatchSize  int    `json:"batchSize"`
}

// SearchConfig controls search mode defaults.
type SearchConfig struct {
	Mode        string `json:"mode"`
	VectorTopK  int    `json:"vectorTopK"`
	FTSLimit    int    `json:"ftsLimit"`
}

// DefaultEmbeddingConfig returns disabled embedding.
func DefaultEmbeddingConfig() EmbeddingConfig {
	return EmbeddingConfig{
		Provider:   ProviderDisabled,
		BatchSize:  16,
		Dimensions: 0,
	}
}

// DefaultSearchConfig returns auto mode (FTS until vector ready).
func DefaultSearchConfig() SearchConfig {
	return SearchConfig{
		Mode:       SearchModeAuto,
		VectorTopK: 30,
		FTSLimit:   50,
	}
}

// IsConfigured reports whether embedding can be used (ignores extension load).
func (c EmbeddingConfig) IsConfigured() bool {
	if c.Provider == "" || c.Provider == ProviderDisabled {
		return false
	}
	if c.Provider != ProviderOpenAICompatible {
		return false
	}
	if strings.TrimSpace(c.Model) == "" {
		return false
	}
	if c.Dimensions < 32 || c.Dimensions > 4096 {
		return false
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return false
	}
	return true
}

// Validate checks config fields when enabling embedding.
func (c EmbeddingConfig) Validate() error {
	if c.Provider == "" || c.Provider == ProviderDisabled {
		return nil
	}
	if c.Provider != ProviderOpenAICompatible {
		return fmt.Errorf("unsupported embedding provider %q", c.Provider)
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("embedding model is required")
	}
	if c.Dimensions < 32 || c.Dimensions > 4096 {
		return fmt.Errorf("dimensions must be between 32 and 4096")
	}
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		return fmt.Errorf("base URL is required")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid base URL")
	}
	if c.BatchSize < 0 || c.BatchSize > 256 {
		return fmt.Errorf("batch size out of range")
	}
	return nil
}

// Normalize fills defaults.
func (c EmbeddingConfig) Normalize() EmbeddingConfig {
	if c.Provider == "" {
		c.Provider = ProviderDisabled
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 16
	}
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model = strings.TrimSpace(c.Model)
	c.APIKey = strings.TrimSpace(c.APIKey)
	return c
}

// Normalize search config defaults.
func (c SearchConfig) Normalize() SearchConfig {
	switch c.Mode {
	case SearchModeAuto, SearchModeFTS, SearchModeVector, SearchModeHybrid:
	default:
		c.Mode = SearchModeAuto
	}
	if c.VectorTopK <= 0 {
		c.VectorTopK = 30
	}
	if c.FTSLimit <= 0 {
		c.FTSLimit = 50
	}
	return c
}

// Masked returns a copy safe for UI (api key redacted).
func (c EmbeddingConfig) Masked() EmbeddingConfig {
	out := c
	if out.APIKey != "" {
		if len(out.APIKey) <= 8 {
			out.APIKey = "***"
		} else {
			out.APIKey = out.APIKey[:3] + "***" + out.APIKey[len(out.APIKey)-2:]
		}
	}
	return out
}
