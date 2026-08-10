export type SmartCollectionId = "unread" | "today" | "starred" | "all";

export type CollectionId = SmartCollectionId | `feed:${string}` | `folder:${string}`;

export type SettingsSectionId =
  | "general"
  | "appearance"
  | "reading"
  | "feeds"
  | "filters"
  | "search_ai"
  | "sync"
  | "shortcuts"
  | "notifications"
  | "advanced"
  | "about";

export interface FeedFolder {
  id: string;
  name: string;
  feedIds: string[];
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
  lastError?: string;
}

export interface Article {
  id: string;
  feedId: string;
  title: string;
  author?: string;
  summary: string;
  contentHtml: string;
  url: string;
  publishedAt: string;
  read: boolean;
  starred: boolean;
  imageUrl?: string;
}

export interface ReaderSelection {
  collectionId: CollectionId;
  articleId: string | null;
}

/** Result of FeedService.ImportOPML */
export interface OPMLImportResult {
  foldersCreated: number;
  feedsAdded: number;
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
 * UI prefs persisted via SettingsService.GetUIPrefs / SetUIPrefs.
 * autoRefresh / refreshIntervalMinutes stay on LibraryConfig and are merged on load.
 * launchAtLogin is UI-only (system API out of scope for S6).
 */
export interface UIPrefs {
  markAsReadOnOpen: boolean;
  markAsReadOnScrollEnd: boolean;
  openOnStartup: string; // unread|today|starred|all
  hideReadOnStartup: boolean;
  theme: string; // system|light|dark
  accent: string;
  compactSidebar: boolean;
  fontSize: string; // sm|md|lg
  showUnreadOnly: boolean;
  openLinksInBrowser: boolean;
  readerWidth: string; // narrow|medium|wide
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
}

export interface AppSettings {
  // 通用 · 刷新 (LibraryConfig)
  autoRefresh: boolean;
  refreshIntervalMinutes: number;

  // 通用 · 已读行为
  markAsReadOnOpen: boolean;
  markAsReadOnScrollEnd: boolean;

  // 通用 · 启动
  launchAtLogin: boolean;
  openOnStartup: SmartCollectionId;
  hideReadOnStartup: boolean;

  // 外观
  theme: "system" | "light" | "dark";
  /** Preset id (purple|blue|teal|orange) or custom #rrggbb */
  accent: string;
  compactSidebar: boolean;

  // 阅读
  fontSize: "sm" | "md" | "lg";
  showUnreadOnly: boolean;
  openLinksInBrowser: boolean;
  readerWidth: "narrow" | "medium" | "wide";

  // 订阅
  defaultFolderId: string | null;
  fetchFullContent: boolean;
  keepArticlesDays: number;

  // 过滤规则
  hideDuplicateTitles: boolean;
  blockKeywords: string;

  // 同步 (not in UIPrefs / S6 non-goal)
  syncEnabled: boolean;
  syncProvider: "none" | "icloud" | "webdav" | "custom";

  // 快捷键
  enableKeyboardShortcuts: boolean;

  // 通知
  notifyOnNewArticles: boolean;
  notifySound: boolean;

  // 高级
  hardwareAcceleration: boolean;
  clearCacheOnQuit: boolean;
  developerMode: boolean;
}
