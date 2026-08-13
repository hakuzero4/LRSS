package settings

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Key for UI preferences JSON blob.
const KeyUIPrefs = "app.ui_prefs"

// ReaderToolbarButtons controls which icons appear in the article reader header.
// See DefaultReaderToolbarButtons. JSON keys match frontend settings.readerToolbar.
type ReaderToolbarButtons struct {
	Zen          bool `json:"zen"`
	Star         bool `json:"star"`
	Read         bool `json:"read"`
	Summarize    bool `json:"summarize"`
	Translate    bool `json:"translate"`
	AI           bool `json:"ai"`
	FetchFull    bool `json:"fetchFull"`
	Markdown     bool `json:"markdown"`
	OpenOriginal bool `json:"openOriginal"`
}

// DefaultReaderToolbarButtons is the out-of-box reader header set:
// read, summarize, fetch full, markdown, open original.
// Zen, star, translate, and AI menu stay available in settings but off by default.
func DefaultReaderToolbarButtons() ReaderToolbarButtons {
	return ReaderToolbarButtons{
		Zen:          false,
		Star:         false,
		Read:         true,
		Summarize:    true,
		Translate:    false,
		AI:           false,
		FetchFull:    true,
		Markdown:     true,
		OpenOriginal: true,
	}
}

// UIPrefs holds frontend UI / reading / retention preferences.
// JSON keys match the frontend AppSettings fields (camelCase).
type UIPrefs struct {
	MarkAsReadOnOpen      bool   `json:"markAsReadOnOpen"`
	MarkAsReadOnScrollEnd bool   `json:"markAsReadOnScrollEnd"`
	OpenOnStartup         string `json:"openOnStartup"` // unread|today|starred|all
	HideReadOnStartup     bool   `json:"hideReadOnStartup"`
	Theme                 string `json:"theme"`  // system|light|dark
	Accent                string `json:"accent"` // blue|purple|teal|orange
	CompactSidebar        bool   `json:"compactSidebar"`
	// MicaBackdrop: Windows 11 Mica window material. Ignored on other OSes.
	MicaBackdrop bool   `json:"micaBackdrop"`
	FontSize     string `json:"fontSize"` // sm|md|lg
	// ReaderFontFamily is a CSS font family name for article body/title.
	// Empty or "system" uses the app default sans stack.
	ReaderFontFamily        string `json:"readerFontFamily"`
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
	// SelectTranslate: when true, selecting text in the reader shows AI 划词翻译.
	SelectTranslate bool `json:"selectTranslate"`
	// AutoFetchFull: when true, opening an article asks the LLM if the body is
	// partial; if so, automatically fetch the original page full text.
	AutoFetchFull bool `json:"autoFetchFull"`
	// TranslateReplaceOriginal: when true, full-article translate overwrites content_html/text.
	// When false, only the bilingual overlay is shown (no body replace).
	TranslateReplaceOriginal bool `json:"translateReplaceOriginal"`
	// ReaderToolbar: which header icons are visible in the article reader.
	ReaderToolbar ReaderToolbarButtons `json:"readerToolbar"`
	// Locale is the UI language: "zh-CN" | "en-US". Empty → default zh-CN.
	// Shared with Web access so the browser UI matches the desktop app.
	Locale string `json:"locale"`
}

// DefaultUIPrefs matches frontend default settings in useRssStore.
func DefaultUIPrefs() UIPrefs {
	return UIPrefs{
		MarkAsReadOnOpen:         true,
		MarkAsReadOnScrollEnd:    false,
		OpenOnStartup:            "unread",
		HideReadOnStartup:        false,
		Theme:                    "light",
		Accent:                   "purple",
		CompactSidebar:           false,
		MicaBackdrop:             true, // Win11 desktop; no-op elsewhere / web access
		FontSize:                 "md",
		ReaderFontFamily:         "", // system / app default
		ShowUnreadOnly:           false,
		OpenLinksInBrowser:       true,
		ReaderWidth:              "medium",
		DefaultFolderId:          "",
		FetchFullContent:         false,
		KeepArticlesDays:         90,
		HideDuplicateTitles:      true,
		BlockKeywords:            "",
		EnableKeyboardShortcuts:  true,
		NotifyOnNewArticles:      false,
		NotifySound:              false,
		HardwareAcceleration:     true,
		ClearCacheOnQuit:         false,
		DeveloperMode:            false,
		NsfwMode:                 true, // show all until user enables office hide
		AutoSummarize:            false,
		SelectTranslate:          true,  // 划词翻译 on by default when LLM is configured
		AutoFetchFull:            false, // network + LLM; opt-in
		TranslateReplaceOriginal: false, // keep original body; bilingual overlay only
		ReaderToolbar:            DefaultReaderToolbarButtons(),
		// Locale empty until first save — frontend migrates from localStorage.
		Locale: "",
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
	// Normalize reader font: "system" / whitespace → empty (app default).
	c.ReaderFontFamily = strings.TrimSpace(c.ReaderFontFamily)
	if strings.EqualFold(c.ReaderFontFamily, "system") ||
		strings.EqualFold(c.ReaderFontFamily, "default") {
		c.ReaderFontFamily = ""
	}
	// Reject characters that break CSS font-family injection.
	if strings.ContainsAny(c.ReaderFontFamily, `/\<>|{};`) {
		c.ReaderFontFamily = ""
	}
	if len(c.ReaderFontFamily) > 80 {
		c.ReaderFontFamily = ""
	}
	if c.ReaderWidth == "" {
		c.ReaderWidth = "medium"
	}
	c.Locale = strings.TrimSpace(c.Locale)
	if c.Locale != "" {
		switch c.Locale {
		case "zh-CN", "en-US":
			// ok
		case "zh", "zh-cn", "zh_CN", "zh-Hans":
			c.Locale = "zh-CN"
		case "en", "en-us", "en_US", "en-GB", "en-gb":
			c.Locale = "en-US"
		default:
			low := strings.ToLower(c.Locale)
			if strings.HasPrefix(low, "zh") {
				c.Locale = "zh-CN"
			} else if strings.HasPrefix(low, "en") {
				c.Locale = "en-US"
			} else {
				c.Locale = "zh-CN"
			}
		}
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
