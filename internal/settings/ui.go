package settings

import (
	"context"
	"database/sql"
	"errors"
)

// Key for UI preferences JSON blob.
const KeyUIPrefs = "app.ui_prefs"

// UIPrefs holds frontend UI / reading / retention preferences.
// JSON keys match the frontend AppSettings fields (camelCase).
type UIPrefs struct {
	MarkAsReadOnOpen        bool   `json:"markAsReadOnOpen"`
	MarkAsReadOnScrollEnd   bool   `json:"markAsReadOnScrollEnd"`
	OpenOnStartup           string `json:"openOnStartup"` // unread|today|starred|all
	HideReadOnStartup       bool   `json:"hideReadOnStartup"`
	Theme                   string `json:"theme"`  // system|light|dark
	Accent                  string `json:"accent"` // blue|purple|teal|orange
	CompactSidebar          bool   `json:"compactSidebar"`
	FontSize                string `json:"fontSize"` // sm|md|lg
	ShowUnreadOnly          bool   `json:"showUnreadOnly"`
	OpenLinksInBrowser      bool   `json:"openLinksInBrowser"`
	ReaderWidth             string `json:"readerWidth"`     // narrow|medium|wide|fill
	DefaultFolderId         string `json:"defaultFolderId"` // empty = none
	FetchFullContent        bool   `json:"fetchFullContent"`
	KeepArticlesDays        int    `json:"keepArticlesDays"` // 7–365, default 90
	HideDuplicateTitles     bool   `json:"hideDuplicateTitles"`
	BlockKeywords           string `json:"blockKeywords"`
	EnableKeyboardShortcuts bool   `json:"enableKeyboardShortcuts"`
	NotifyOnNewArticles     bool   `json:"notifyOnNewArticles"`
	NotifySound             bool   `json:"notifySound"`
	HardwareAcceleration    bool   `json:"hardwareAcceleration"`
	ClearCacheOnQuit        bool   `json:"clearCacheOnQuit"`
	DeveloperMode           bool   `json:"developerMode"`
	// NsfwMode: true = show NSFW feeds (default); false = office mode hide isNsfw feeds.
	NsfwMode bool `json:"nsfwMode"`
	// AutoSummarize: when true, opening an article requests an LLM summary (if LLM configured).
	AutoSummarize bool `json:"autoSummarize"`
}

// DefaultUIPrefs matches frontend default settings in useRssStore.
func DefaultUIPrefs() UIPrefs {
	return UIPrefs{
		MarkAsReadOnOpen:        true,
		MarkAsReadOnScrollEnd:   false,
		OpenOnStartup:           "unread",
		HideReadOnStartup:       false,
		Theme:                   "light",
		Accent:                  "purple",
		CompactSidebar:          false,
		FontSize:                "md",
		ShowUnreadOnly:          false,
		OpenLinksInBrowser:      true,
		ReaderWidth:             "medium",
		DefaultFolderId:         "",
		FetchFullContent:        false,
		KeepArticlesDays:        90,
		HideDuplicateTitles:     true,
		BlockKeywords:           "",
		EnableKeyboardShortcuts: true,
		NotifyOnNewArticles:     false,
		NotifySound:             false,
		HardwareAcceleration:    true,
		ClearCacheOnQuit:        false,
		DeveloperMode:           false,
		NsfwMode:                true, // show all until user enables office hide
		AutoSummarize:           false,
	}
}

// Normalize clamps KeepArticlesDays to [7, 365] and fills empty string enums
// with frontend defaults when blank.
func (c UIPrefs) Normalize() UIPrefs {
	if c.KeepArticlesDays < 7 {
		c.KeepArticlesDays = 7
	}
	if c.KeepArticlesDays > 365 {
		c.KeepArticlesDays = 365
	}
	if c.OpenOnStartup == "" {
		c.OpenOnStartup = "unread"
	}
	if c.Theme == "" {
		c.Theme = "light"
	}
	if c.Accent == "" {
		c.Accent = "purple"
	}
	if c.FontSize == "" {
		c.FontSize = "md"
	}
	if c.ReaderWidth == "" {
		c.ReaderWidth = "medium"
	}
	return c
}

// LoadUIPrefs loads UI prefs with defaults when unset or partial.
func (s *Store) LoadUIPrefs(ctx context.Context) (UIPrefs, error) {
	cfg := DefaultUIPrefs()
	err := s.GetJSON(ctx, KeyUIPrefs, &cfg)
	if errors.Is(err, sql.ErrNoRows) {
		return cfg.Normalize(), nil
	}
	if err != nil {
		return DefaultUIPrefs(), err
	}
	return cfg.Normalize(), nil
}

// SaveUIPrefs normalizes and persists UI prefs as JSON.
func (s *Store) SaveUIPrefs(ctx context.Context, cfg UIPrefs) error {
	cfg = cfg.Normalize()
	return s.SetJSON(ctx, KeyUIPrefs, cfg)
}

// GetUIPrefs is an alias used by appsvc.
func (s *Store) GetUIPrefs(ctx context.Context) (UIPrefs, error) {
	return s.LoadUIPrefs(ctx)
}

// SetUIPrefs is an alias used by appsvc.
func (s *Store) SetUIPrefs(ctx context.Context, cfg UIPrefs) error {
	return s.SaveUIPrefs(ctx, cfg)
}
