package appsvc

import (
	"context"
	"fmt"
	"log"
	"strings"

	"lrss/internal/db"
	"lrss/internal/job"
	"lrss/internal/llm"
	"lrss/internal/notify"
	"lrss/internal/search"
	"lrss/internal/service"
	"lrss/internal/settings"
	"lrss/internal/sysfont"
	"lrss/internal/vector"
	"lrss/internal/web"
)

// SettingsService is exposed to the Wails frontend.
type SettingsService struct {
	store     *settings.Store
	search    *search.Service
	embed     *job.EmbedWorker
	index     *vector.Index
	library   *service.Library // optional; required for PurgeOldArticles
	notify    *notify.Sender   // optional; test + permission helpers
	webServer *web.Server      // optional; browser access HTTP server
	onUIPrefs func(settings.UIPrefs)
}

// NewSettings constructs the service from shared deps.
// Call SetLibrary after constructing the library to enable article purge.
func NewSettings(store *settings.Store, searchSvc *search.Service, worker *job.EmbedWorker) *SettingsService {
	return &SettingsService{
		store:  store,
		search: searchSvc,
		embed:  worker,
		index:  vector.NewIndex(searchSvc.SQL),
	}
}

// SetLibrary injects the library used for retention purge.
//
//wails:ignore
func (s *SettingsService) SetLibrary(lib *service.Library) {
	s.library = lib
}

// SetNotifier injects the desktop notification sender (test notification UI).
//
//wails:ignore
func (s *SettingsService) SetNotifier(n *notify.Sender) {
	s.notify = n
}

// SetOnUIPrefs is called after a successful SetUIPrefs (desktop Mica toggle, etc).
//
//wails:ignore
func (s *SettingsService) SetOnUIPrefs(fn func(settings.UIPrefs)) {
	s.onUIPrefs = fn
}

// EnsureNotificationPermission requests OS notification permission when needed.
func (s *SettingsService) EnsureNotificationPermission() (bool, error) {
	if s.notify == nil {
		return false, fmt.Errorf("notifications unavailable")
	}
	return s.notify.EnsureAuthorized(context.Background())
}

// TestNotification sends a sample system notification (settings panel).
func (s *SettingsService) TestNotification() error {
	if s.notify == nil {
		return fmt.Errorf("notifications unavailable")
	}
	return s.notify.Test(context.Background())
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

// GetLLMConfig returns masked chat LLM settings.
func (s *SettingsService) GetLLMConfig() (settings.LLMConfig, error) {
	cfg, err := s.store.LoadLLMConfig(context.Background())
	if err != nil {
		return settings.LLMConfig{}, err
	}
	return cfg.Masked(), nil
}

// SetLLMConfig validates and saves chat LLM settings.
// When the UI sends a masked API key, the previous key is kept.
func (s *SettingsService) SetLLMConfig(cfg settings.LLMConfig) error {
	ctx := context.Background()
	old, err := s.store.LoadLLMConfig(ctx)
	if err != nil {
		return err
	}
	cfg = cfg.Normalize()
	if cfg.APIKey == "" || cfg.APIKey == "***" || containsMask(cfg.APIKey) {
		if old.APIKey != "" && (cfg.APIKey == "" || containsMask(cfg.APIKey)) {
			cfg.APIKey = old.APIKey
		}
	}
	return s.store.SaveLLMConfig(ctx, cfg)
}

// TestLLMConfig tries a minimal chat completion with the given config
// (or saved config if fields are empty / key is masked). Returns a short reply snippet.
func (s *SettingsService) TestLLMConfig(cfg settings.LLMConfig) (string, error) {
	ctx := context.Background()
	saved, err := s.store.LoadLLMConfig(ctx)
	if err != nil {
		return "", err
	}
	cfg = cfg.Normalize()
	// Merge blanks from saved so the UI can test without re-typing the key.
	if cfg.Provider == "" || cfg.Provider == settings.LLMProviderDisabled {
		if saved.IsConfigured() {
			cfg = saved
		}
	}
	if cfg.APIKey == "" || cfg.APIKey == "***" || containsMask(cfg.APIKey) {
		cfg.APIKey = saved.APIKey
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = saved.BaseURL
	}
	if strings.TrimSpace(cfg.Model) == "" {
		cfg.Model = saved.Model
	}
	if cfg.Provider == "" || cfg.Provider == settings.LLMProviderDisabled {
		cfg.Provider = settings.LLMProviderOpenAICompatible
	}
	cfg = cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	if !cfg.IsConfigured() {
		return "", fmt.Errorf("llm not configured")
	}
	cli, err := llm.NewClient(cfg)
	if err != nil {
		return "", err
	}
	return cli.TestConnection(ctx)
}

// GetSearchConfig returns search mode settings.
func (s *SettingsService) GetSearchConfig() (settings.SearchConfig, error) {
	return s.store.LoadSearchConfig(context.Background())
}

// SetSearchConfig saves search mode settings.
func (s *SettingsService) SetSearchConfig(cfg settings.SearchConfig) error {
	return s.store.SaveSearchConfig(context.Background(), cfg)
}

// GetLibraryConfig returns auto-refresh settings.
func (s *SettingsService) GetLibraryConfig() (settings.LibraryConfig, error) {
	return s.store.GetLibraryConfig(context.Background())
}

// SetLibraryConfig validates (clamps interval) and saves library settings.
func (s *SettingsService) SetLibraryConfig(cfg settings.LibraryConfig) error {
	return s.store.SetLibraryConfig(context.Background(), cfg)
}

// ListSystemFonts returns installed / common font family names for the reading
// typography picker (sorted, unique). Safe for CSS font-family selection.
func (s *SettingsService) ListSystemFonts() []string {
	return sysfont.List()
}

// GetUIPrefs returns UI / reading / retention preferences (defaults when unset).
func (s *SettingsService) GetUIPrefs() (settings.UIPrefs, error) {
	return s.store.GetUIPrefs(context.Background())
}

// SetUIPrefs normalizes and persists UI preferences.
// If keepArticlesDays changed, a background purge is scheduled.
func (s *SettingsService) SetUIPrefs(cfg settings.UIPrefs) error {
	ctx := context.Background()
	old, err := s.store.GetUIPrefs(ctx)
	if err != nil {
		return err
	}
	cfg = cfg.Normalize()
	if err := s.store.SetUIPrefs(ctx, cfg); err != nil {
		return err
	}
	if s.onUIPrefs != nil {
		s.onUIPrefs(cfg)
	}
	if s.library != nil && old.KeepArticlesDays != cfg.KeepArticlesDays {
		days := cfg.KeepArticlesDays
		lib := s.library
		go func() {
			n, err := lib.PurgeOldArticles(context.Background(), days)
			if err != nil {
				log.Printf("purge after SetUIPrefs: %v", err)
				return
			}
			if n > 0 {
				log.Printf("purge after SetUIPrefs: deleted %d articles (keep=%d days)", n, days)
			}
		}()
	}
	if s.library != nil && old.RecentReadLimit != cfg.RecentReadLimit {
		if err := s.library.PruneOpened(ctx, cfg.RecentReadLimit); err != nil {
			log.Printf("prune recent-read after SetUIPrefs: %v", err)
		}
	}
	return nil
}

// PurgeResult is returned by PurgeOldArticles for the frontend.
type PurgeResult struct {
	Deleted int `json:"deleted"`
}

// CacheClearResult is returned by ClearLLMCache.
type CacheClearResult struct {
	Deleted int64 `json:"deleted"`
}

// ClearLLMCache wipes AI feature result cache (summarize/translate/etc.).
func (s *SettingsService) ClearLLMCache() (CacheClearResult, error) {
	if s.search == nil || s.search.SQL == nil {
		return CacheClearResult{}, fmt.Errorf("database unavailable")
	}
	cache := &llm.Cache{DB: s.search.SQL}
	n, err := cache.Clear(context.Background())
	if err != nil {
		return CacheClearResult{}, err
	}
	return CacheClearResult{Deleted: n}, nil
}

// LLMCacheCount returns how many rows are in llm_feature_cache (diagnostics).
func (s *SettingsService) LLMCacheCount() (int64, error) {
	if s.search == nil || s.search.SQL == nil {
		return 0, fmt.Errorf("database unavailable")
	}
	var n int64
	err := s.search.SQL.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM llm_feature_cache`).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// PurgeOldArticles deletes non-starred articles older than the current keepArticlesDays.
func (s *SettingsService) PurgeOldArticles() (PurgeResult, error) {
	if s.library == nil {
		return PurgeResult{}, fmt.Errorf("library not configured")
	}
	ctx := context.Background()
	prefs, err := s.store.GetUIPrefs(ctx)
	if err != nil {
		return PurgeResult{}, err
	}
	n, err := s.library.PurgeOldArticles(ctx, prefs.KeepArticlesDays)
	if err != nil {
		return PurgeResult{}, err
	}
	return PurgeResult{Deleted: n}, nil
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

// SearchService is the Wails-facing search API.
type SearchService struct {
	inner *search.Service
}

// NewSearch wraps the domain search service.
func NewSearch(inner *search.Service) *SearchService {
	return &SearchService{inner: inner}
}

// Search runs library search. Respects UIPrefs.nsfwMode (office hide).
func (s *SearchService) Search(query string, mode string, limit int) (search.Result, error) {
	exclude := false
	if s != nil && s.inner != nil && s.inner.Settings != nil {
		prefs, err := s.inner.Settings.LoadUIPrefs(context.Background())
		if err == nil {
			exclude = !prefs.NsfwMode
		}
	}
	return s.inner.Search(context.Background(), query, search.Options{
		Mode: mode, Limit: limit, ExcludeNsfw: exclude,
	})
}
