export type SmartCollectionId = "unread" | "today" | "starred" | "all" | "recent";

export type CollectionId = SmartCollectionId | `feed:${string}` | `folder:${string}`;

export type SettingsSectionId =
  | "general"
  | "appearance"
  | "reading"
  | "feeds"
  | "filters"
  | "search_ai" // models: LLM + embedding connection
  | "ai_features" // feature toggles (auto summarize, select translate, …)
  | "sync"
  | "shortcuts"
  | "notifications"
  | "advanced"
  | "about";

export type FolderDisplayMode = "list" | "cards";

export interface FeedFolder {
  id: string;
  name: string;
  feedIds: string[];
  /** Sensitive folder; hidden (with its feeds) when nsfwMode is false. */
  isNsfw?: boolean;
  /** Article list layout for this folder (and feeds inside it). */
  displayMode?: FolderDisplayMode;
}

export interface Feed {
  id: string;
  title: string;
  siteUrl: string;
  feedUrl: string;
  favicon?: string;
  folderId?: string;
  unreadCount: number;
  lastFetchedAt: string;
  /** true when auto-refresh is paused for this feed */
  isPaused?: boolean;
  /**
   * Per-feed auto-refresh interval in minutes.
   * 0 (or undefined) = follow global LibraryConfig default.
   */
  refreshIntervalMinutes?: number;
  /** 0 = follow global keepArticlesDays; else [7, 365]. */
  keepArticlesDays?: number;
  lastError?: string;
  /** Sensitive feed; hidden from smart lists/sidebar when nsfwMode is false. */
  isNsfw?: boolean;
}

export interface Article {
  id: string;
  feedId: string;
  title: string;
  author?: string;
  summary: string;
  contentHtml: string;
  /** Bilingual <<o>>/<<t>> text; original contentHtml is always kept. */
  translationRaw?: string;
  translationLang?: string;
  url: string;
  publishedAt: string;
  read: boolean;
  starred: boolean;
  imageUrl?: string;
  /** True after full-page fetch replaced the body; skip auto re-fetch. */
  fullContentFetched?: boolean;
}

export interface ReaderSelection {
  collectionId: CollectionId;
  articleId: string | null;
}

/** Result of FeedService.ImportOPML */
export interface OPMLImportResult {
  foldersCreated: number;
  feedsAdded: number;
  /** Existing URL: folder/title/site synced from OPML */
  feedsUpdated: number;
  feedsSkipped: number;
  feedsFailed: number;
  errors?: string[];
  /** Newly inserted feed IDs (for progressive RefreshFeed) */
  addedFeedIds?: string[];
}

/** Progress updates during OPML import + optional article fetch. */
export type OPMLImportProgress = {
  phase: "parse" | "write" | "fetch" | "done";
  message: string;
  current?: number;
  total?: number;
};

export interface LibraryConfig {
  autoRefresh: boolean;
  refreshIntervalMinutes: number;
}

/**
 * Which icons appear in the article reader header (Settings → Appearance).
 * Defaults: read, summarize, fetchFull, markdown, openOriginal on; rest off.
 */
export interface ReaderToolbarButtons {
  zen: boolean;
  star: boolean;
  read: boolean;
  summarize: boolean;
  translate: boolean;
  ai: boolean;
  fetchFull: boolean;
  markdown: boolean;
  openOriginal: boolean;
}

/** Default visible set: read, summarize, fetch full, markdown, open original. */
export const DEFAULT_READER_TOOLBAR: ReaderToolbarButtons = {
  zen: false,
  star: false,
  read: true,
  summarize: true,
  translate: false,
  ai: false,
  fetchFull: true,
  markdown: true,
  openOriginal: true,
};

export const READER_TOOLBAR_KEYS = [
  "zen",
  "star",
  "read",
  "summarize",
  "translate",
  "ai",
  "fetchFull",
  "markdown",
  "openOriginal",
] as const satisfies readonly (keyof ReaderToolbarButtons)[];

/**
 * UI prefs persisted via SettingsService.GetUIPrefs / SetUIPrefs.
 * autoRefresh / refreshIntervalMinutes stay on LibraryConfig and are merged on load.
 * Launch at login is an OS setting (SettingsService.Get/SetLaunchAtLogin), not UIPrefs.
 */
export interface UIPrefs {
  markAsReadOnOpen: boolean;
  markAsReadOnScrollEnd: boolean;
  openOnStartup: string; // unread|today|starred|recent|all
  hideReadOnStartup: boolean;
  /** Recently-read rows to keep. 10–200, default 50. */
  recentReadLimit: number;
  theme: string; // system|light|dark
  accent: string;
  compactSidebar: boolean;
  /** Windows 11 Mica window material. No-op on other platforms / web access. */
  micaBackdrop: boolean;
  fontSize: string; // sm|md|lg
  /** CSS font family for article body; empty = app default. */
  readerFontFamily: string;
  showUnreadOnly: boolean;
  openLinksInBrowser: boolean;
  readerWidth: string; // narrow|medium|wide|fill
  defaultFolderId: string; // empty = null
  fetchFullContent: boolean;
  keepArticlesDays: number; // 7–365
  hideDuplicateTitles: boolean;
  blockKeywords: string;
  enableKeyboardShortcuts: boolean;
  notifyOnNewArticles: boolean;
  notifySound: boolean;
  hardwareAcceleration: boolean;
  clearCacheOnQuit: boolean;
  developerMode: boolean;
  /** true = show NSFW feeds; false = office mode hide isNsfw feeds */
  nsfwMode: boolean;
  /** When true, open article triggers LLM summarize (if LLM configured). */
  autoSummarize: boolean;
  /** When true, selecting text in the reader shows AI 划词翻译. */
  selectTranslate: boolean;
  /**
   * When true, opening an article asks the LLM if the body is partial;
   * if so, auto-fetch the original page full text.
   */
  autoFetchFull: boolean;
  /**
   * @deprecated Prefer always-kept original + translationRaw.
   * Kept for prefs compatibility; no longer overwrites content_html.
   */
  translateReplaceOriginal?: boolean;
  /** Reader header toolbar button visibility. */
  readerToolbar: ReaderToolbarButtons;
  /** UI language: zh-CN | en-US (synced to Web access via backend). */
  locale?: string;
}

export interface AppSettings {
  // 通用 · 刷新 (LibraryConfig)
  autoRefresh: boolean;
  refreshIntervalMinutes: number;

  // 通用 · 已读行为
  markAsReadOnOpen: boolean;
  markAsReadOnScrollEnd: boolean;

  // 通用 · 启动
  /** Unused local leftover; OS launch-at-login lives in Settings → Advanced (not UIPrefs). */
  launchAtLogin: boolean;
  openOnStartup: SmartCollectionId;
  hideReadOnStartup: boolean;
  /** Recently-read rows to keep. 10–200, default 50. */
  recentReadLimit: number;

  // 外观
  theme: "system" | "light" | "dark";
  /** Preset id (purple|blue|teal|orange) or custom #rrggbb */
  accent: string;
  compactSidebar: boolean;
  /** Windows 11 Mica window material. */
  micaBackdrop: boolean;

  // 阅读
  fontSize: "sm" | "md" | "lg";
  /** CSS font family for article body/title; empty = app default. */
  readerFontFamily: string;
  showUnreadOnly: boolean;
  openLinksInBrowser: boolean;
  readerWidth: "narrow" | "medium" | "wide" | "fill";

  // 订阅
  defaultFolderId: string | null;
  fetchFullContent: boolean;
  keepArticlesDays: number;

  // 过滤规则
  hideDuplicateTitles: boolean;
  blockKeywords: string;

  // 同步 (not in UIPrefs / S6 non-goal)
  /** UI-only mirror; real sync config lives in SyncService / app.sync_config. */
  syncEnabled: boolean;
  syncProvider: "none" | "webdav" | "s3";

  // 快捷键
  enableKeyboardShortcuts: boolean;

  // 通知
  notifyOnNewArticles: boolean;
  notifySound: boolean;

  // 高级
  hardwareAcceleration: boolean;
  clearCacheOnQuit: boolean;
  developerMode: boolean;

  /**
   * NSFW mode: true shows sensitive feeds; false (office) hides them
   * from smart lists and the sidebar (settings feed list still shows all).
   */
  nsfwMode: boolean;
  /** Auto-run LLM summarize when opening an article (if LLM configured). */
  autoSummarize: boolean;
  /** Select text in reader to AI-translate (划词翻译). */
  selectTranslate: boolean;
  /** AI judges partial body on open and auto-fetches full page when needed. */
  autoFetchFull: boolean;
  /** @deprecated unused for overwrite; original body is always kept. */
  translateReplaceOriginal?: boolean;
  /** Which icons show in the article reader top bar. */
  readerToolbar: ReaderToolbarButtons;
}
