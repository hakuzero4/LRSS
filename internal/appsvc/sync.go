package appsvc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lrss/internal/cloudsync"
	"lrss/internal/service"
	"lrss/internal/settings"
)

// SyncService exposes subscription-only remote sync (OPML via WebDAV / S3).
type SyncService struct {
	store *settings.Store
	lib   *service.Library
}

// NewSync constructs the sync façade.
func NewSync(store *settings.Store, lib *service.Library) *SyncService {
	return &SyncService{store: store, lib: lib}
}

// SetLibrary injects library (optional late bind).
//
//wails:ignore
func (s *SyncService) SetLibrary(lib *service.Library) {
	s.lib = lib
}

// GetSyncConfig returns masked sync settings.
func (s *SyncService) GetSyncConfig() (settings.SyncConfig, error) {
	if s == nil || s.store == nil {
		return settings.DefaultSyncConfig(), fmt.Errorf("sync unavailable")
	}
	cfg, err := s.store.LoadSyncConfig(context.Background())
	if err != nil {
		return settings.SyncConfig{}, err
	}
	return cfg.Masked(), nil
}

// SetSyncConfig saves sync settings. Masked secrets keep previous values.
func (s *SyncService) SetSyncConfig(cfg settings.SyncConfig) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("sync unavailable")
	}
	ctx := context.Background()
	old, err := s.store.LoadSyncConfig(ctx)
	if err != nil {
		return err
	}
	cfg = cfg.Normalize()
	// Preserve secrets when UI sends masks / empty placeholders.
	if isMaskedSecret(cfg.WebDAVPassword) {
		cfg.WebDAVPassword = old.WebDAVPassword
	}
	if isMaskedSecret(cfg.S3SecretKey) {
		cfg.S3SecretKey = old.S3SecretKey
	}
	if isMaskedSecret(cfg.S3AccessKey) || looksMaskedAccessKey(cfg.S3AccessKey) {
		cfg.S3AccessKey = old.S3AccessKey
	}
	// Keep last sync timestamps from disk unless caller cleared them.
	if cfg.LastPushAt == "" {
		cfg.LastPushAt = old.LastPushAt
	}
	if cfg.LastPullAt == "" {
		cfg.LastPullAt = old.LastPullAt
	}
	return s.store.SaveSyncConfig(ctx, cfg)
}

func isMaskedSecret(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || s == "***" {
		return true
	}
	return strings.Contains(s, "***")
}

func looksMaskedAccessKey(s string) bool {
	return strings.Contains(s, "***")
}

// SyncTestResult is returned by TestSyncConnection.
type SyncTestResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// TestSyncConnection verifies the saved (or provided) remote is reachable.
// When cfg fields are partial / secrets masked, merges with saved config.
func (s *SyncService) TestSyncConnection(cfg settings.SyncConfig) (SyncTestResult, error) {
	merged, err := s.mergeConfig(cfg)
	if err != nil {
		return SyncTestResult{}, err
	}
	if !merged.Enabled {
		return SyncTestResult{OK: false, Message: "sync is disabled"}, nil
	}
	store, err := cloudsync.NewStore(merged)
	if err != nil {
		return SyncTestResult{OK: false, Message: err.Error()}, nil
	}
	if err := store.Ping(context.Background()); err != nil {
		return SyncTestResult{OK: false, Message: err.Error()}, nil
	}
	return SyncTestResult{OK: true, Message: "ok"}, nil
}

// SyncPushResult is returned after uploading OPML.
type SyncPushResult struct {
	Bytes    int    `json:"bytes"`
	PushedAt string `json:"pushedAt"`
}

// PushSubscriptions exports local feeds as OPML and uploads to the remote.
// Does not include reading state.
func (s *SyncService) PushSubscriptions() (SyncPushResult, error) {
	if s.lib == nil {
		return SyncPushResult{}, fmt.Errorf("library not configured")
	}
	ctx := context.Background()
	cfg, err := s.store.LoadSyncConfig(ctx)
	if err != nil {
		return SyncPushResult{}, err
	}
	if !cfg.IsConfigured() {
		return SyncPushResult{}, fmt.Errorf("sync is not configured")
	}
	xml, err := s.lib.ExportOPML(ctx)
	if err != nil {
		return SyncPushResult{}, err
	}
	body := []byte(xml)
	store, err := cloudsync.NewStore(cfg)
	if err != nil {
		return SyncPushResult{}, err
	}
	if err := store.Put(ctx, body); err != nil {
		_ = s.recordError(ctx, cfg, err)
		return SyncPushResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cfg.LastPushAt = now
	cfg.LastError = ""
	_ = s.store.SaveSyncConfig(ctx, cfg)
	return SyncPushResult{Bytes: len(body), PushedAt: now}, nil
}

// SyncPullResult is returned after downloading and importing OPML.
type SyncPullResult struct {
	Bytes      int    `json:"bytes"`
	Imported   int    `json:"imported"`
	Skipped    int    `json:"skipped"`
	Folders    int    `json:"folders"`
	PulledAt   string `json:"pulledAt"`
	FetchAfter bool   `json:"fetchAfter"`
}

// PullSubscriptions downloads remote OPML and imports subscriptions.
// fetchAfter controls whether new feeds are refreshed immediately.
// Does not merge reading state.
func (s *SyncService) PullSubscriptions(fetchAfter bool) (SyncPullResult, error) {
	if s.lib == nil {
		return SyncPullResult{}, fmt.Errorf("library not configured")
	}
	ctx := context.Background()
	cfg, err := s.store.LoadSyncConfig(ctx)
	if err != nil {
		return SyncPullResult{}, err
	}
	if !cfg.IsConfigured() {
		return SyncPullResult{}, fmt.Errorf("sync is not configured")
	}
	store, err := cloudsync.NewStore(cfg)
	if err != nil {
		return SyncPullResult{}, err
	}
	body, err := store.Get(ctx)
	if err != nil {
		_ = s.recordError(ctx, cfg, err)
		return SyncPullResult{}, err
	}
	res, err := s.lib.ImportOPML(ctx, string(body), fetchAfter)
	if err != nil {
		_ = s.recordError(ctx, cfg, err)
		return SyncPullResult{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	cfg.LastPullAt = now
	cfg.LastError = ""
	_ = s.store.SaveSyncConfig(ctx, cfg)
	return SyncPullResult{
		Bytes:      len(body),
		Imported:   res.FeedsAdded,
		Skipped:    res.FeedsSkipped,
		Folders:    res.FoldersCreated,
		PulledAt:   now,
		FetchAfter: fetchAfter,
	}, nil
}

func (s *SyncService) recordError(ctx context.Context, cfg settings.SyncConfig, err error) error {
	if err == nil {
		return nil
	}
	cfg.LastError = err.Error()
	if len(cfg.LastError) > 500 {
		cfg.LastError = cfg.LastError[:500] + "…"
	}
	return s.store.SaveSyncConfig(ctx, cfg)
}

func (s *SyncService) mergeConfig(cfg settings.SyncConfig) (settings.SyncConfig, error) {
	ctx := context.Background()
	saved, err := s.store.LoadSyncConfig(ctx)
	if err != nil {
		return settings.SyncConfig{}, err
	}
	cfg = cfg.Normalize()
	// Start from form; fill secrets from saved when masked.
	if isMaskedSecret(cfg.WebDAVPassword) {
		cfg.WebDAVPassword = saved.WebDAVPassword
	}
	if isMaskedSecret(cfg.S3SecretKey) {
		cfg.S3SecretKey = saved.S3SecretKey
	}
	if isMaskedSecret(cfg.S3AccessKey) || looksMaskedAccessKey(cfg.S3AccessKey) {
		cfg.S3AccessKey = saved.S3AccessKey
	}
	// Empty non-secret fields: keep form values; for test, allow using saved endpoint if blank.
	if cfg.Provider == settings.SyncProviderNone && saved.Provider != settings.SyncProviderNone {
		cfg.Provider = saved.Provider
	}
	if cfg.WebDAVURL == "" {
		cfg.WebDAVURL = saved.WebDAVURL
	}
	if cfg.S3Endpoint == "" {
		cfg.S3Endpoint = saved.S3Endpoint
	}
	if cfg.S3Bucket == "" {
		cfg.S3Bucket = saved.S3Bucket
	}
	if !cfg.Enabled && saved.Enabled {
		// Allow testing with form toggle on.
	}
	// For connection test, force enabled if provider configured.
	if cfg.Provider != settings.SyncProviderNone {
		cfg.Enabled = true
	}
	return cfg.Normalize(), nil
}
