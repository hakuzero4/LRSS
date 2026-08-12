package appsvc

import (
	"context"
	"fmt"

	"lrss/internal/settings"
	"lrss/internal/web"
)

// WebAccessStatus is the desktop UI view of the browser HTTP server.
// Mirrors web.Status with json tags for Wails.
type WebAccessStatus = web.Status

// SetWebServer injects the optional browser-access HTTP server.
//
//wails:ignore
func (s *SettingsService) SetWebServer(srv *web.Server) {
	s.webServer = srv
}

// GetWebAccessConfig returns persisted web access settings.
func (s *SettingsService) GetWebAccessConfig() (settings.WebAccessConfig, error) {
	return s.store.GetWebAccessConfig(context.Background())
}

// SetWebAccessConfig saves settings and starts/stops/rebinds the HTTP server.
func (s *SettingsService) SetWebAccessConfig(cfg settings.WebAccessConfig) (WebAccessStatus, error) {
	ctx := context.Background()
	saved, err := s.store.SetWebAccessConfig(ctx, cfg)
	if err != nil {
		return WebAccessStatus{}, err
	}
	if s.webServer == nil {
		st := WebAccessStatus{
			Running:  false,
			Bind:     saved.Bind,
			Port:     saved.Port,
			HasToken: saved.Token != "",
			Error:    "web server not wired",
		}
		return st, fmt.Errorf("web server not wired")
	}
	return s.webServer.Apply(ctx, saved)
}

// GetWebAccessStatus returns whether the HTTP server is running and its URL.
func (s *SettingsService) GetWebAccessStatus() (WebAccessStatus, error) {
	if s.webServer == nil {
		cfg, err := s.store.GetWebAccessConfig(context.Background())
		if err != nil {
			return WebAccessStatus{}, err
		}
		return WebAccessStatus{
			Running:  false,
			Bind:     cfg.Bind,
			Port:     cfg.Port,
			HasToken: cfg.Token != "",
		}, nil
	}
	return s.webServer.Status(), nil
}

// RegenerateWebAccessToken creates a new token, saves, and restarts if enabled.
func (s *SettingsService) RegenerateWebAccessToken() (settings.WebAccessConfig, error) {
	ctx := context.Background()
	cfg, err := s.store.GetWebAccessConfig(ctx)
	if err != nil {
		return settings.WebAccessConfig{}, err
	}
	tok, err := settings.GenerateWebAccessToken()
	if err != nil {
		return settings.WebAccessConfig{}, err
	}
	cfg.Token = tok
	saved, err := s.store.SetWebAccessConfig(ctx, cfg)
	if err != nil {
		return settings.WebAccessConfig{}, err
	}
	if s.webServer != nil && saved.Enabled {
		if _, err := s.webServer.Apply(ctx, saved); err != nil {
			return saved, err
		}
	}
	return saved, nil
}
