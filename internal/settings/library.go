package settings

import (
	"context"
	"fmt"
)

// Settings keys for library / auto-refresh.
const (
	KeyLibraryAutoRefresh            = "library.auto_refresh"
	KeyLibraryRefreshIntervalMinutes = "library.refresh_interval_minutes"
)

// LibraryConfig controls background refresh behaviour.
type LibraryConfig struct {
	AutoRefresh            bool `json:"autoRefresh"`
	RefreshIntervalMinutes int  `json:"refreshIntervalMinutes"`
}

// DefaultLibraryConfig returns auto-refresh on every 30 minutes.
func DefaultLibraryConfig() LibraryConfig {
	return LibraryConfig{
		AutoRefresh:            true,
		RefreshIntervalMinutes: 30,
	}
}

// Normalize clamps interval to [5, 180] and fills defaults.
func (c LibraryConfig) Normalize() LibraryConfig {
	if c.RefreshIntervalMinutes < 5 {
		c.RefreshIntervalMinutes = 5
	}
	if c.RefreshIntervalMinutes > 180 {
		c.RefreshIntervalMinutes = 180
	}
	return c
}

// LoadLibraryConfig loads library settings with defaults.
func (s *Store) LoadLibraryConfig(ctx context.Context) (LibraryConfig, error) {
	cfg := DefaultLibraryConfig()

	if v, err := s.Get(ctx, KeyLibraryAutoRefresh); err != nil {
		return cfg, err
	} else if v != "" {
		switch v {
		case "1", "true", "TRUE", "True":
			cfg.AutoRefresh = true
		case "0", "false", "FALSE", "False":
			cfg.AutoRefresh = false
		default:
			// Keep default on unexpected values.
		}
	}

	if v, err := s.Get(ctx, KeyLibraryRefreshIntervalMinutes); err != nil {
		return cfg, err
	} else if v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			cfg.RefreshIntervalMinutes = n
		}
	}

	return cfg.Normalize(), nil
}

// SaveLibraryConfig validates, clamps, and persists library settings.
func (s *Store) SaveLibraryConfig(ctx context.Context, cfg LibraryConfig) error {
	cfg = cfg.Normalize()
	auto := "0"
	if cfg.AutoRefresh {
		auto = "1"
	}
	pairs := []struct{ k, v string }{
		{KeyLibraryAutoRefresh, auto},
		{KeyLibraryRefreshIntervalMinutes, fmt.Sprintf("%d", cfg.RefreshIntervalMinutes)},
	}
	for _, p := range pairs {
		if err := s.Set(ctx, p.k, p.v); err != nil {
			return err
		}
	}
	return nil
}

// GetLibraryConfig is an alias used by appsvc (same as LoadLibraryConfig).
func (s *Store) GetLibraryConfig(ctx context.Context) (LibraryConfig, error) {
	return s.LoadLibraryConfig(ctx)
}

// SetLibraryConfig is an alias used by appsvc (same as SaveLibraryConfig).
func (s *Store) SetLibraryConfig(ctx context.Context, cfg LibraryConfig) error {
	return s.SaveLibraryConfig(ctx, cfg)
}
