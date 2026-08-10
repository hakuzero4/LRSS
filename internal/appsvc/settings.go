package appsvc

import (
	"context"
	"fmt"

	"lrss/internal/db"
	"lrss/internal/job"
	"lrss/internal/search"
	"lrss/internal/settings"
	"lrss/internal/vector"
)

// SettingsService is exposed to the Wails frontend.
type SettingsService struct {
	store  *settings.Store
	search *search.Service
	embed  *job.EmbedWorker
	index  *vector.Index
}

// NewSettings constructs the service from shared deps.
func NewSettings(store *settings.Store, searchSvc *search.Service, worker *job.EmbedWorker) *SettingsService {
	return &SettingsService{
		store:  store,
		search: searchSvc,
		embed:  worker,
		index:  vector.NewIndex(searchSvc.SQL),
	}
}

// GetEmbeddingConfig returns masked embedding settings.
func (s *SettingsService) GetEmbeddingConfig() (settings.EmbeddingConfig, error) {
	cfg, err := s.store.LoadEmbeddingConfig(context.Background())
	if err != nil {
		return settings.EmbeddingConfig{}, err
	}
	return cfg.Masked(), nil
}

// SetEmbeddingConfig validates and saves embedding settings.
// If model/dimensions change, embeddings are invalidated and re-queued.
func (s *SettingsService) SetEmbeddingConfig(cfg settings.EmbeddingConfig) error {
	ctx := context.Background()
	old, err := s.store.LoadEmbeddingConfig(ctx)
	if err != nil {
		return err
	}
	cfg = cfg.Normalize()
	// Allow partial updates keeping existing API key when masked placeholder sent.
	if cfg.APIKey == "" || cfg.APIKey == "***" || (len(cfg.APIKey) > 3 && cfg.APIKey[3:6] == "***") {
		if old.APIKey != "" && (cfg.APIKey == "" || containsMask(cfg.APIKey)) {
			cfg.APIKey = old.APIKey
		}
	}
	if err := s.store.SaveEmbeddingConfig(ctx, cfg); err != nil {
		return err
	}
	if !cfg.IsConfigured() {
		return nil
	}
	changed := old.Model != cfg.Model || old.Dimensions != cfg.Dimensions || old.Provider != cfg.Provider
	if changed {
		_ = s.index.InvalidateAll(ctx)
		_ = s.embed.EnqueueAllPending(ctx)
	}
	if db.VectorInfo().Loaded {
		_ = s.index.Ensure(ctx, cfg.Dimensions)
	}
	return nil
}

func containsMask(s string) bool {
	return len(s) >= 3 && (s == "***" || (len(s) > 6 && s[3:6] == "***"))
}

// GetSearchConfig returns search mode settings.
func (s *SettingsService) GetSearchConfig() (settings.SearchConfig, error) {
	return s.store.LoadSearchConfig(context.Background())
}

// SetSearchConfig saves search mode settings.
func (s *SettingsService) SetSearchConfig(cfg settings.SearchConfig) error {
	return s.store.SaveSearchConfig(context.Background(), cfg)
}

// GetSearchCapabilities reports FTS/vector availability.
func (s *SettingsService) GetSearchCapabilities() (search.Capabilities, error) {
	return s.search.Capabilities(context.Background())
}

// GetVectorStatus returns sqlite-vector load status.
func (s *SettingsService) GetVectorStatus() db.VectorStatus {
	return db.VectorInfo()
}

// RunEmbedOnce processes a batch of pending embeddings (manual trigger).
func (s *SettingsService) RunEmbedOnce(limit int) (int, error) {
	return s.embed.RunOnce(context.Background(), limit)
}

// RebuildAllEmbeddings invalidates all vectors, re-queues every article, and
// processes batches until idle (or max rounds). Requires embedding configured.
// Returns processed article count and worker rounds.
func (s *SettingsService) RebuildAllEmbeddings() (map[string]int, error) {
	ctx := context.Background()
	cfg, err := s.store.LoadEmbeddingConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.IsConfigured() {
		return nil, fmt.Errorf("请先配置向量模型后再重新生成")
	}
	if err := s.index.InvalidateAll(ctx); err != nil {
		return nil, err
	}
	if err := s.embed.EnqueueAllPending(ctx); err != nil {
		return nil, err
	}
	total := 0
	rounds := 0
	batch := cfg.BatchSize
	if batch <= 0 {
		batch = 16
	}
	for rounds < 500 {
		n, err := s.embed.RunOnce(ctx, batch)
		if err != nil {
			return map[string]int{"processed": total, "rounds": rounds}, err
		}
		if n == 0 {
			break
		}
		total += n
		rounds++
	}
	return map[string]int{"processed": total, "rounds": rounds}, nil
}

// SearchService is exposed to the frontend.
type SearchService struct {
	inner *search.Service
}

// NewSearch wraps the domain search service.
func NewSearch(inner *search.Service) *SearchService {
	return &SearchService{inner: inner}
}

// Search runs library search.
func (s *SearchService) Search(query string, mode string, limit int) (search.Result, error) {
	return s.inner.Search(context.Background(), query, search.Options{Mode: mode, Limit: limit})
}
