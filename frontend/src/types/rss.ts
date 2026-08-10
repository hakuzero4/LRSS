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

export interface AppSettings {
  // 通用 · 刷新
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
  accent: "blue" | "purple" | "teal" | "orange";
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

  // 同步
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
