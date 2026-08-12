package settings

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
)

// Key for web access JSON blob.
const KeyWebAccess = "app.web_access"

// DefaultWebPort is the default HTTP listen port for browser access.
const DefaultWebPort = 18765

// WebAccessConfig controls the optional local HTTP server for browser reading.
type WebAccessConfig struct {
	Enabled bool `json:"enabled"`
	// Bind is "localhost" (127.0.0.1 only) or "lan" (0.0.0.0).
	Bind string `json:"bind"`
	// Port is the listen port; default 18765; clamp [1024, 65535].
	Port int `json:"port"`
	// Token is an optional shared secret. Empty disables auth (OK for localhost).
	// When Bind=lan and Enabled, empty token is auto-generated on save/apply.
	Token string `json:"token"`
}

// DefaultWebAccessConfig is off, localhost-only, default port, no token.
func DefaultWebAccessConfig() WebAccessConfig {
	return WebAccessConfig{
		Enabled: false,
		Bind:    "localhost",
		Port:    DefaultWebPort,
		Token:   "",
	}
}

// Normalize clamps fields and fills defaults.
func (c WebAccessConfig) Normalize() WebAccessConfig {
	c.Bind = strings.ToLower(strings.TrimSpace(c.Bind))
	if c.Bind != "lan" && c.Bind != "localhost" {
		c.Bind = "localhost"
	}
	if c.Port < 1024 || c.Port > 65535 {
		c.Port = DefaultWebPort
	}
	c.Token = strings.TrimSpace(c.Token)
	if len(c.Token) > 128 {
		c.Token = c.Token[:128]
	}
	return c
}

// EnsureTokenForLAN generates a token when enabling LAN without one.
// Returns the config and whether a new token was created.
func (c WebAccessConfig) EnsureTokenForLAN() (WebAccessConfig, bool) {
	c = c.Normalize()
	if c.Enabled && c.Bind == "lan" && c.Token == "" {
		tok, err := GenerateWebAccessToken()
		if err == nil {
			c.Token = tok
			return c, true
		}
	}
	return c, false
}

// GenerateWebAccessToken returns 32 random bytes as hex (64 chars).
func GenerateWebAccessToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// LoadWebAccessConfig loads web access prefs with defaults when unset.
func (s *Store) LoadWebAccessConfig(ctx context.Context) (WebAccessConfig, error) {
	cfg := DefaultWebAccessConfig()
	err := s.GetJSON(ctx, KeyWebAccess, &cfg)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg.Normalize(), nil
	}
	if err != nil {
		return DefaultWebAccessConfig(), err
	}
	return cfg.Normalize(), nil
}

// SaveWebAccessConfig normalizes and persists web access config.
// When enabling LAN without a token, generates one before save.
func (s *Store) SaveWebAccessConfig(ctx context.Context, cfg WebAccessConfig) (WebAccessConfig, error) {
	cfg, _ = cfg.EnsureTokenForLAN()
	cfg = cfg.Normalize()
	if err := s.SetJSON(ctx, KeyWebAccess, cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// GetWebAccessConfig is an alias used by appsvc.
func (s *Store) GetWebAccessConfig(ctx context.Context) (WebAccessConfig, error) {
	return s.LoadWebAccessConfig(ctx)
}

// SetWebAccessConfig is an alias used by appsvc; returns the saved config.
func (s *Store) SetWebAccessConfig(ctx context.Context, cfg WebAccessConfig) (WebAccessConfig, error) {
	return s.SaveWebAccessConfig(ctx, cfg)
}
