package settings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// Keys for embedding / search / LLM.
const (
	KeyEmbeddingProvider   = "embedding.provider"
	KeyEmbeddingBaseURL    = "embedding.base_url"
	KeyEmbeddingAPIKey     = "embedding.api_key"
	KeyEmbeddingModel      = "embedding.model"
	KeyEmbeddingDimensions = "embedding.dimensions"
	KeyEmbeddingBatchSize  = "embedding.batch_size"
	KeySearchMode          = "search.mode"
	KeySearchVectorTopK    = "search.vector_top_k"
	KeySearchFTSLimit      = "search.fts_limit"

	KeyLLMProvider     = "llm.provider"
	KeyLLMBaseURL      = "llm.base_url"
	KeyLLMAPIKey       = "llm.api_key"
	KeyLLMModel        = "llm.model"
	KeyLLMTemperature  = "llm.temperature"
	KeyLLMMaxTokens    = "llm.max_tokens"
	KeyLLMSystemPrompt = "llm.system_prompt"
)

// Store reads and writes the settings table.
type Store struct {
	db *sql.DB
}

// NewStore creates a settings store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Get returns a raw string value, or empty if missing.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("settings get %s: %w", key, err)
	}
	return v, nil
}

// Set writes a raw string value.
func (s *Store) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("settings set %s: %w", key, err)
	}
	return nil
}

// GetJSON unmarshals a JSON setting into dest.
func (s *Store) GetJSON(ctx context.Context, key string, dest any) error {
	raw, err := s.Get(ctx, key)
	if err != nil {
		return err
	}
	if raw == "" {
		return sql.ErrNoRows
	}
	if err := json.Unmarshal([]byte(raw), dest); err != nil {
		return fmt.Errorf("settings json %s: %w", key, err)
	}
	return nil
}

// SetJSON marshals and stores a value.
func (s *Store) SetJSON(ctx context.Context, key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Set(ctx, key, string(b))
}

// LoadEmbeddingConfig loads embedding settings with defaults.
func (s *Store) LoadEmbeddingConfig(ctx context.Context) (EmbeddingConfig, error) {
	cfg := DefaultEmbeddingConfig()
	if v, err := s.Get(ctx, KeyEmbeddingProvider); err != nil {
		return cfg, err
	} else if v != "" {
		cfg.Provider = v
	}
	if v, err := s.Get(ctx, KeyEmbeddingBaseURL); err != nil {
		return cfg, err
	} else {
		cfg.BaseURL = v
	}
	if v, err := s.Get(ctx, KeyEmbeddingAPIKey); err != nil {
		return cfg, err
	} else {
		cfg.APIKey = v
	}
	if v, err := s.Get(ctx, KeyEmbeddingModel); err != nil {
		return cfg, err
	} else {
		cfg.Model = v
	}
	if v, err := s.Get(ctx, KeyEmbeddingDimensions); err != nil {
		return cfg, err
	} else if v != "" {
		var d int
		if err := json.Unmarshal([]byte(v), &d); err == nil {
			cfg.Dimensions = d
		} else {
			fmt.Sscanf(v, "%d", &cfg.Dimensions)
		}
	}
	if v, err := s.Get(ctx, KeyEmbeddingBatchSize); err != nil {
		return cfg, err
	} else if v != "" {
		fmt.Sscanf(v, "%d", &cfg.BatchSize)
	}
	return cfg.Normalize(), nil
}

// SaveEmbeddingConfig validates and persists embedding settings.
func (s *Store) SaveEmbeddingConfig(ctx context.Context, cfg EmbeddingConfig) error {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	pairs := []struct {
		k, v string
	}{
		{KeyEmbeddingProvider, cfg.Provider},
		{KeyEmbeddingBaseURL, cfg.BaseURL},
		{KeyEmbeddingAPIKey, cfg.APIKey},
		{KeyEmbeddingModel, cfg.Model},
		{KeyEmbeddingDimensions, fmt.Sprintf("%d", cfg.Dimensions)},
		{KeyEmbeddingBatchSize, fmt.Sprintf("%d", cfg.BatchSize)},
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range pairs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, p.k, p.v); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// LoadSearchConfig loads search settings with defaults.
func (s *Store) LoadSearchConfig(ctx context.Context) (SearchConfig, error) {
	cfg := DefaultSearchConfig()
	if v, err := s.Get(ctx, KeySearchMode); err != nil {
		return cfg, err
	} else if v != "" {
		cfg.Mode = v
	}
	if v, err := s.Get(ctx, KeySearchVectorTopK); err != nil {
		return cfg, err
	} else if v != "" {
		fmt.Sscanf(v, "%d", &cfg.VectorTopK)
	}
	if v, err := s.Get(ctx, KeySearchFTSLimit); err != nil {
		return cfg, err
	} else if v != "" {
		fmt.Sscanf(v, "%d", &cfg.FTSLimit)
	}
	return cfg.Normalize(), nil
}

// SaveSearchConfig persists search settings.
func (s *Store) SaveSearchConfig(ctx context.Context, cfg SearchConfig) error {
	cfg = cfg.Normalize()
	pairs := []struct{ k, v string }{
		{KeySearchMode, cfg.Mode},
		{KeySearchVectorTopK, fmt.Sprintf("%d", cfg.VectorTopK)},
		{KeySearchFTSLimit, fmt.Sprintf("%d", cfg.FTSLimit)},
	}
	for _, p := range pairs {
		if err := s.Set(ctx, p.k, p.v); err != nil {
			return err
		}
	}
	return nil
}

// LoadLLMConfig loads chat LLM settings with defaults.
func (s *Store) LoadLLMConfig(ctx context.Context) (LLMConfig, error) {
	cfg := DefaultLLMConfig()
	if v, err := s.Get(ctx, KeyLLMProvider); err != nil {
		return cfg, err
	} else if v != "" {
		cfg.Provider = v
	}
	if v, err := s.Get(ctx, KeyLLMBaseURL); err != nil {
		return cfg, err
	} else {
		cfg.BaseURL = v
	}
	if v, err := s.Get(ctx, KeyLLMAPIKey); err != nil {
		return cfg, err
	} else {
		cfg.APIKey = v
	}
	if v, err := s.Get(ctx, KeyLLMModel); err != nil {
		return cfg, err
	} else {
		cfg.Model = v
	}
	if v, err := s.Get(ctx, KeyLLMTemperature); err != nil {
		return cfg, err
	} else if v != "" {
		var t float64
		if _, err := fmt.Sscanf(v, "%f", &t); err == nil {
			cfg.Temperature = t
		}
	}
	if v, err := s.Get(ctx, KeyLLMMaxTokens); err != nil {
		return cfg, err
	} else if v != "" {
		fmt.Sscanf(v, "%d", &cfg.MaxTokens)
	}
	if v, err := s.Get(ctx, KeyLLMSystemPrompt); err != nil {
		return cfg, err
	} else {
		cfg.SystemPrompt = v
	}
	return cfg.Normalize(), nil
}

// SaveLLMConfig validates and persists LLM settings.
func (s *Store) SaveLLMConfig(ctx context.Context, cfg LLMConfig) error {
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	pairs := []struct {
		k, v string
	}{
		{KeyLLMProvider, cfg.Provider},
		{KeyLLMBaseURL, cfg.BaseURL},
		{KeyLLMAPIKey, cfg.APIKey},
		{KeyLLMModel, cfg.Model},
		{KeyLLMTemperature, fmt.Sprintf("%g", cfg.Temperature)},
		{KeyLLMMaxTokens, fmt.Sprintf("%d", cfg.MaxTokens)},
		{KeyLLMSystemPrompt, cfg.SystemPrompt},
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range pairs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value`, p.k, p.v); err != nil {
			return err
		}
	}
	return tx.Commit()
}
