import { computed, reactive, ref } from "vue";
import i18n from "@/i18n";
import { loadAppsvc, mapArticle, mapFeed, mapFolder } from "@/lib/backend";
import type {
  AppSettings,
  Article,
  CollectionId,
  Feed,
  FeedFolder,
  LibraryConfig,
  OPMLImportProgress,
  OPMLImportResult,
  SmartCollectionId,
  UIPrefs,
} from "@/types/rss";

/** Translate outside setup (module-level store). Depends on locale for reactivity. */
function t(key: string, params?: Record<string, unknown>): string {
  void i18n.global.locale.value;
  return i18n.global.t(key, params as any);
}

// Empty until backend bootstrap (or offline add-folder mock). No seed/demo library.
const folders = ref<FeedFolder[]>([]);
const feeds = ref<Feed[]>([]);
const articles = ref<Article[]>([]);

const collectionId = ref<CollectionId>("unread");
const selectedArticleId = ref<string | null>(null);
const searchQuery = ref("");
const addFeedOpen = ref(false);
const settingsOpen = ref(false);
const refreshing = ref(false);
const backendReady = ref(false);
const bootstrapError = ref("");

const settings = reactive<AppSettings>({
  autoRefresh: true,
  refreshIntervalMinutes: 30,
  markAsReadOnOpen: true,
  markAsReadOnScrollEnd: false,
  launchAtLogin: false,
  openOnStartup: "unread",
  hideReadOnStartup: false,
  theme: "light",
  accent: "purple",
  compactSidebar: false,
  fontSize: "md",
  showUnreadOnly: false,
  openLinksInBrowser: true,
  readerWidth: "medium",
  defaultFolderId: null,
  fetchFullContent: false,
  keepArticlesDays: 90,
  hideDuplicateTitles: true,
  blockKeywords: "",
  syncEnabled: false,
  syncProvider: "none",
  enableKeyboardShortcuts: true,
  notifyOnNewArticles: false,
  notifySound: false,
  hardwareAcceleration: true,
  clearCacheOnQuit: false,
  developerMode: false,
});

function isToday(iso: string): boolean {
  const d = new Date(iso);
  const now = new Date();
  return (
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
  );
}

const smartCounts = computed(() => {
  const list = articles.value;
  return {
    unread: list.filter((a) => !a.read).length,
    today: list.filter((a) => isToday(a.publishedAt)).length,
    starred: list.filter((a) => a.starred).length,
    all: list.length,
  } satisfies Record<SmartCollectionId, number>;
});

const collectionTitle = computed(() => {
  const id = collectionId.value;
  if (id === "unread") return t("nav.unread");
  if (id === "today") return t("nav.today");
  if (id === "starred") return t("nav.starred");
  if (id === "all") return t("nav.all");
  if (id.startsWith("feed:")) {
    return feeds.value.find((f) => f.id === id.slice(5))?.title ?? t("nav.feedFallback");
  }
  if (id.startsWith("folder:")) {
    return folders.value.find((f) => f.id === id.slice(7))?.name ?? t("nav.folderFallback");
  }
  return t("nav.library");
});

const filteredArticles = computed(() => {
  // When backend is ready, `articles` is already scoped by collection via reloadArticles.
  // Still apply client search + showUnreadOnly for responsiveness.
  let list = [...articles.value];
  if (!backendReady.value) {
    const id = collectionId.value;
    if (id === "unread") list = list.filter((a) => !a.read);
    else if (id === "today") list = list.filter((a) => isToday(a.publishedAt));
    else if (id === "starred") list = list.filter((a) => a.starred);
    else if (id.startsWith("feed:")) list = list.filter((a) => a.feedId === id.slice(5));
    else if (id.startsWith("folder:")) {
      const folder = folders.value.find((f) => f.id === id.slice(7));
      const ids = new Set(folder?.feedIds ?? []);
      list = list.filter((a) => ids.has(a.feedId));
    }
  }
  if (settings.showUnreadOnly && collectionId.value !== "starred") {
    list = list.filter((a) => !a.read);
  }
  const q = searchQuery.value.trim().toLowerCase();
  if (q) {
    list = list.filter(
      (a) =>
        a.title.toLowerCase().includes(q) ||
        a.summary.toLowerCase().includes(q) ||
        (a.author?.toLowerCase().includes(q) ?? false),
    );
  }
  return list.sort(
    (a, b) => new Date(b.publishedAt).getTime() - new Date(a.publishedAt).getTime(),
  );
});

const selectedArticle = computed(
  () => articles.value.find((a) => a.id === selectedArticleId.value) ?? null,
);

const selectedFeed = computed(() => {
  const article = selectedArticle.value;
  if (!article) return null;
  return feeds.value.find((f) => f.id === article.feedId) ?? null;
});

async function reloadLibrary() {
  const api = await loadAppsvc();
  if (!api?.FeedService) {
    backendReady.value = false;
    return;
  }
  try {
    const [rawFeeds, rawFolders] = await Promise.all([
      api.FeedService.ListFeeds(),
      api.FeedService.ListFolders(),
    ]);
    const mappedFeeds = (rawFeeds ?? []).map(mapFeed) as Feed[];
    feeds.value = mappedFeeds;
    folders.value = (rawFolders ?? []).map((f: any) => mapFolder(f, mappedFeeds)) as FeedFolder[];
    backendReady.value = true;
    bootstrapError.value = "";
    await reloadArticles();
  } catch (e: any) {
    bootstrapError.value = e?.message || String(e);
    backendReady.value = false;
  }
}

async function reloadArticles() {
  const api = await loadAppsvc();
  if (!api?.ArticleService) return;
  const list = await api.ArticleService.List(collectionId.value, 200, 0);
  articles.value = (list ?? []).map(mapArticle) as Article[];
}

async function loadLibraryConfig() {
  const api = await loadAppsvc();
  const getCfg = api?.SettingsService?.GetLibraryConfig;
  if (typeof getCfg !== "function") return;
  try {
    const cfg = (await getCfg()) as LibraryConfig | null | undefined;
    if (!cfg || typeof cfg !== "object") return;
    if (typeof cfg.autoRefresh === "boolean") {
      settings.autoRefresh = cfg.autoRefresh;
    }
    if (
      typeof cfg.refreshIntervalMinutes === "number" &&
      Number.isFinite(cfg.refreshIntervalMinutes)
    ) {
      settings.refreshIntervalMinutes = cfg.refreshIntervalMinutes;
    }
  } catch (e) {
    console.warn("[lrss] GetLibraryConfig failed", e);
  }
}

/** Persist auto-refresh settings to backend when available. */
async function persistLibraryConfig(): Promise<void> {
  const api = await loadAppsvc();
  const setCfg = api?.SettingsService?.SetLibraryConfig;
  if (typeof setCfg !== "function") return;
  try {
    const cfg: LibraryConfig = {
      autoRefresh: settings.autoRefresh,
      refreshIntervalMinutes: settings.refreshIntervalMinutes,
    };
    await setCfg(cfg);
  } catch (e) {
    console.warn("[lrss] SetLibraryConfig failed", e);
  }
}

const SMART_COLLECTIONS: SmartCollectionId[] = ["unread", "today", "starred", "all"];
const THEMES = new Set(["system", "light", "dark"]);

const FONT_SIZES = new Set(["sm", "md", "lg"]);
const READER_WIDTHS = new Set(["narrow", "medium", "wide"]);

function isSmartCollection(v: unknown): v is SmartCollectionId {
  return typeof v === "string" && (SMART_COLLECTIONS as string[]).includes(v);
}

function buildUIPrefs(): UIPrefs {
  return {
    markAsReadOnOpen: settings.markAsReadOnOpen,
    markAsReadOnScrollEnd: settings.markAsReadOnScrollEnd,
    openOnStartup: settings.openOnStartup,
    hideReadOnStartup: settings.hideReadOnStartup,
    theme: settings.theme,
    accent: settings.accent,
    compactSidebar: settings.compactSidebar,
    fontSize: settings.fontSize,
    showUnreadOnly: settings.showUnreadOnly,
    openLinksInBrowser: settings.openLinksInBrowser,
    readerWidth: settings.readerWidth,
    defaultFolderId: settings.defaultFolderId ?? "",
    fetchFullContent: settings.fetchFullContent,
    keepArticlesDays: settings.keepArticlesDays,
    hideDuplicateTitles: settings.hideDuplicateTitles,
    blockKeywords: settings.blockKeywords,
    enableKeyboardShortcuts: settings.enableKeyboardShortcuts,
    notifyOnNewArticles: settings.notifyOnNewArticles,
    notifySound: settings.notifySound,
    hardwareAcceleration: settings.hardwareAcceleration,
    clearCacheOnQuit: settings.clearCacheOnQuit,
    developerMode: settings.developerMode,
  };
}

/** Coerce Wails / JSON quirks (boolean | "true" | 0/1) into real booleans. */
function coerceBool(v: unknown): boolean | undefined {
  if (typeof v === "boolean") return v;
  if (v === 1 || v === "1" || v === "true" || v === "TRUE" || v === "True") return true;
  if (v === 0 || v === "0" || v === "false" || v === "FALSE" || v === "False") return false;
  return undefined;
}

function pickBool(obj: Record<string, unknown>, ...keys: string[]): boolean | undefined {
  for (const k of keys) {
    if (k in obj) {
      const b = coerceBool(obj[k]);
      if (b !== undefined) return b;
    }
  }
  return undefined;
}

function applyUIPrefs(prefs: Partial<UIPrefs> | Record<string, unknown> | null | undefined) {
  if (!prefs || typeof prefs !== "object") return;
  const p = prefs as Record<string, unknown>;

  const markOpen = pickBool(p, "markAsReadOnOpen", "MarkAsReadOnOpen");
  if (markOpen !== undefined) settings.markAsReadOnOpen = markOpen;

  const markScroll = pickBool(p, "markAsReadOnScrollEnd", "MarkAsReadOnScrollEnd");
  if (markScroll !== undefined) settings.markAsReadOnScrollEnd = markScroll;

  const openStartup = p.openOnStartup ?? p.OpenOnStartup;
  if (isSmartCollection(openStartup)) {
    settings.openOnStartup = openStartup;
  }

  const hideRead = pickBool(p, "hideReadOnStartup", "HideReadOnStartup");
  if (hideRead !== undefined) settings.hideReadOnStartup = hideRead;

  const theme = p.theme ?? p.Theme;
  if (typeof theme === "string" && THEMES.has(theme)) {
    settings.theme = theme as AppSettings["theme"];
  }
  const accent = p.accent ?? p.Accent;
  if (typeof accent === "string") {
    // Preset id or #rrggbb custom color
    const a = accent.trim();
    if (
      a === "blue" ||
      a === "purple" ||
      a === "teal" ||
      a === "orange" ||
      /^#[0-9a-fA-F]{6}$/.test(a) ||
      /^#[0-9a-fA-F]{3}$/.test(a)
    ) {
      settings.accent = a.length === 4 && a.startsWith("#")
        ? `#${a[1]}${a[1]}${a[2]}${a[2]}${a[3]}${a[3]}`.toLowerCase()
        : a.startsWith("#")
          ? a.toLowerCase()
          : a;
    }
  }

  const compact = pickBool(p, "compactSidebar", "CompactSidebar");
  if (compact !== undefined) settings.compactSidebar = compact;

  const fontSize = p.fontSize ?? p.FontSize;
  if (typeof fontSize === "string" && FONT_SIZES.has(fontSize)) {
    settings.fontSize = fontSize as AppSettings["fontSize"];
  }

  const showUnread = pickBool(p, "showUnreadOnly", "ShowUnreadOnly");
  if (showUnread !== undefined) settings.showUnreadOnly = showUnread;

  const openLinks = pickBool(p, "openLinksInBrowser", "OpenLinksInBrowser");
  if (openLinks !== undefined) settings.openLinksInBrowser = openLinks;

  const readerWidth = p.readerWidth ?? p.ReaderWidth;
  if (typeof readerWidth === "string" && READER_WIDTHS.has(readerWidth)) {
    settings.readerWidth = readerWidth as AppSettings["readerWidth"];
  }

  const folderId = p.defaultFolderId ?? p.DefaultFolderId;
  if (typeof folderId === "string") {
    settings.defaultFolderId = folderId.trim() ? folderId : null;
  }

  const fetchFull = pickBool(p, "fetchFullContent", "FetchFullContent");
  if (fetchFull !== undefined) settings.fetchFullContent = fetchFull;

  const keepDays = p.keepArticlesDays ?? p.KeepArticlesDays;
  if (typeof keepDays === "number" && Number.isFinite(keepDays)) {
    const d = Math.round(keepDays);
    settings.keepArticlesDays = Math.min(365, Math.max(7, d));
  }

  const hideDup = pickBool(p, "hideDuplicateTitles", "HideDuplicateTitles");
  if (hideDup !== undefined) settings.hideDuplicateTitles = hideDup;

  const keywords = p.blockKeywords ?? p.BlockKeywords;
  if (typeof keywords === "string") {
    settings.blockKeywords = keywords;
  }

  const kb = pickBool(p, "enableKeyboardShortcuts", "EnableKeyboardShortcuts");
  if (kb !== undefined) settings.enableKeyboardShortcuts = kb;

  const notifyNew = pickBool(p, "notifyOnNewArticles", "NotifyOnNewArticles");
  if (notifyNew !== undefined) settings.notifyOnNewArticles = notifyNew;

  const notifySnd = pickBool(p, "notifySound", "NotifySound");
  if (notifySnd !== undefined) settings.notifySound = notifySnd;

  const hw = pickBool(p, "hardwareAcceleration", "HardwareAcceleration");
  if (hw !== undefined) settings.hardwareAcceleration = hw;

  const clearCache = pickBool(p, "clearCacheOnQuit", "ClearCacheOnQuit");
  if (clearCache !== undefined) settings.clearCacheOnQuit = clearCache;

  const dev = pickBool(p, "developerMode", "DeveloperMode");
  if (dev !== undefined) settings.developerMode = dev;
}

async function loadUIPrefs() {
  const api = await loadAppsvc();
  const getPrefs = api?.SettingsService?.GetUIPrefs;
  if (typeof getPrefs !== "function") return;
  try {
    const prefs = (await getPrefs()) as UIPrefs | null | undefined;
    applyUIPrefs(prefs ?? undefined);
  } catch (e) {
    console.warn("[lrss] GetUIPrefs failed", e);
  }
}

let persistUIPrefsTimer: ReturnType<typeof setTimeout> | null = null;

/** Serialize saves so concurrent callers don't interleave. */
let persistUIPrefsChain: Promise<void> = Promise.resolve();

/**
 * Persist UI prefs to SQLite via SetUIPrefs.
 * @param immediate skip debounce (use for switches the user expects to stick)
 */
function persistUIPrefs(immediate = false): void | Promise<void> {
  if (persistUIPrefsTimer) {
    clearTimeout(persistUIPrefsTimer);
    persistUIPrefsTimer = null;
  }
  if (immediate) {
    return persistUIPrefsNow();
  }
  persistUIPrefsTimer = setTimeout(() => {
    persistUIPrefsTimer = null;
    void persistUIPrefsNow();
  }, 300);
}

function persistUIPrefsNow(): Promise<void> {
  const run = async () => {
    try {
      const api = await loadAppsvc();
      const setPrefs = api?.SettingsService?.SetUIPrefs;
      if (typeof setPrefs !== "function") {
        console.warn("[lrss] SetUIPrefs not available — UI prefs will not persist");
        return;
      }
      // Plain JSON object — avoids reactive Proxy issues with Wails IPC.
      const payload = JSON.parse(JSON.stringify(buildUIPrefs())) as UIPrefs;
      await setPrefs(payload);
    } catch (e) {
      console.warn("[lrss] SetUIPrefs failed", e);
    }
  };
  // Always continue the chain even if a previous save failed.
  persistUIPrefsChain = persistUIPrefsChain.then(run, run);
  return persistUIPrefsChain;
}

/**
 * Purge old non-starred articles using current keepArticlesDays.
 * Flushes pending UI prefs first so the backend uses the latest keep days.
 * Returns deleted count (0 if API missing).
 */
async function purgeOldArticles(): Promise<number> {
  if (persistUIPrefsTimer) {
    clearTimeout(persistUIPrefsTimer);
    persistUIPrefsTimer = null;
  }
  await persistUIPrefsNow();

  const api = await loadAppsvc();
  const purgeFn = api?.SettingsService?.PurgeOldArticles;
  if (typeof purgeFn !== "function") return 0;
  try {
    const raw = await purgeFn();
    let deleted = 0;
    if (typeof raw === "number" && Number.isFinite(raw)) {
      deleted = Math.max(0, Math.floor(raw));
    } else if (raw && typeof raw === "object") {
      const o = raw as Record<string, unknown>;
      deleted = Math.max(
        0,
        Math.floor(Number(o.deleted ?? o.purged ?? o.Deleted ?? o.Purged ?? 0) || 0),
      );
    }
    await reloadLibrary();
    return deleted;
  } catch (e) {
    console.warn("[lrss] PurgeOldArticles failed", e);
    throw e;
  }
}

/** Apply openOnStartup / hideReadOnStartup after prefs are loaded. */
function applyStartupPrefs() {
  if (isSmartCollection(settings.openOnStartup)) {
    collectionId.value = settings.openOnStartup;
  }
  if (settings.hideReadOnStartup) {
    settings.showUnreadOnly = true;
  }
}

/**
 * Import OPML: write subscriptions quickly (fetch=false), then refresh new feeds
 * one-by-one so the UI can show progress. Never blocks for N×30s with a single status.
 */
async function importOPMLFile(
  file: File,
  onProgress?: (p: OPMLImportProgress) => void,
): Promise<OPMLImportResult> {
  const empty: OPMLImportResult = {
    foldersCreated: 0,
    feedsAdded: 0,
    feedsSkipped: 0,
    feedsFailed: 0,
    addedFeedIds: [],
  };
  const report = (p: OPMLImportProgress) => {
    try {
      onProgress?.(p);
    } catch {
      /* ignore UI callback errors */
    }
  };

  const api = await loadAppsvc();
  const importFn = api?.FeedService?.ImportOPML;
  if (typeof importFn !== "function") {
    console.warn("[lrss] ImportOPML not available");
    return {
      ...empty,
      feedsFailed: 1,
      errors: [t("opml.backendUnavailable")],
    };
  }

  report({ phase: "parse", message: t("opml.reading") });
  const xml = await file.text();
  if (!xml.trim()) {
    return { ...empty, feedsFailed: 1, errors: [t("opml.emptyFile")] };
  }

  report({ phase: "write", message: t("opml.writing") });
  // fetch=false: return after DB write so large OPML does not freeze the dialog.
  const raw = await importFn(xml, false);
  let result = normalizeImportResult(raw);
  await reloadLibrary();

  const ids = result.addedFeedIds ?? [];
  // Fallback if backend has not returned IDs yet: refresh all after a bulk add.
  if (ids.length === 0 && result.feedsAdded > 0) {
    report({
      phase: "fetch",
      message: t("opml.writtenRefreshing", { n: result.feedsAdded }),
    });
    try {
      await api?.FeedService?.RefreshAll?.();
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      result = {
        ...result,
        feedsFailed: result.feedsFailed + 1,
        errors: [...(result.errors ?? []), msg],
      };
    }
    await reloadLibrary();
    report({
      phase: "done",
      message: t("opml.complete", {
        added: result.feedsAdded,
        skipped: result.feedsSkipped,
        failed: result.feedsFailed,
      }),
    });
    return result;
  }

  const total = ids.length;
  if (total === 0) {
    report({
      phase: "done",
      message:
        result.feedsSkipped > 0
          ? t("opml.noNewSkipped", { n: result.feedsSkipped })
          : t("opml.noNew"),
      current: 0,
      total: 0,
    });
    return result;
  }

  const refreshFn = api?.FeedService?.RefreshFeed;
  if (typeof refreshFn !== "function") {
    report({
      phase: "done",
      message: t("opml.addedNoRefresh", { n: total }),
      current: 0,
      total,
    });
    return result;
  }

  const errors = [...(result.errors ?? [])];
  let failed = result.feedsFailed;

  for (let i = 0; i < ids.length; i++) {
    const id = ids[i];
    const feed = feeds.value.find((f) => f.id === id);
    const label = feed?.title || feed?.feedUrl || id.slice(0, 8);
    report({
      phase: "fetch",
      message: t("opml.fetching", { current: i + 1, total, label }),
      current: i + 1,
      total,
    });
    try {
      await refreshFn(id);
    } catch (e: unknown) {
      failed++;
      const msg = e instanceof Error ? e.message : String(e);
      if (errors.length < 20) {
        errors.push(t("opml.refreshError", { label, msg }));
      }
    }
    // Refresh sidebar counts periodically without thrashing every single feed.
    if (i === ids.length - 1 || (i + 1) % 5 === 0) {
      await reloadLibrary();
    }
  }

  await reloadLibrary();
  result = {
    ...result,
    feedsFailed: failed,
    errors: errors.length ? errors : undefined,
  };
  report({
    phase: "done",
    message: t("opml.completeRefresh", {
      added: result.feedsAdded,
      skipped: result.feedsSkipped,
      failed,
    }),
    current: total,
    total,
  });
  return result;
}

function normalizeImportResult(raw: unknown): OPMLImportResult {
  const r = (raw ?? {}) as Record<string, unknown>;
  const idsRaw = r.addedFeedIds ?? r.AddedFeedIDs;
  const addedFeedIds = Array.isArray(idsRaw)
    ? (idsRaw as unknown[]).map(String).filter(Boolean)
    : [];
  return {
    foldersCreated: Number(r.foldersCreated ?? 0) || 0,
    feedsAdded: Number(r.feedsAdded ?? 0) || 0,
    feedsSkipped: Number(r.feedsSkipped ?? 0) || 0,
    feedsFailed: Number(r.feedsFailed ?? 0) || 0,
    errors: Array.isArray(r.errors)
      ? (r.errors as unknown[]).map(String)
      : undefined,
    addedFeedIds,
  };
}

async function exportOPMLDownload(): Promise<void> {
  const api = await loadAppsvc();
  const exportFn = api?.FeedService?.ExportOPML;
  if (typeof exportFn !== "function") {
    console.warn("[lrss] ExportOPML not available");
    throw new Error(t("opml.exportUnavailable"));
  }
  const xml = await exportFn();
  if (typeof xml !== "string" || !xml) {
    throw new Error(t("opml.exportEmpty"));
  }
  const blob = new Blob([xml], { type: "text/xml;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  try {
    const a = document.createElement("a");
    a.href = url;
    a.download = "lrss-subscriptions.opml";
    a.rel = "noopener";
    document.body.appendChild(a);
    a.click();
    a.remove();
  } finally {
    URL.revokeObjectURL(url);
  }
}

/** Wipe every feed, article, and folder. Irreversible — caller must confirm in UI. */
async function clearAllSubscriptions(): Promise<{ feedsDeleted: number; foldersDeleted: number }> {
  const api = await loadAppsvc();
  const clearFn = api?.FeedService?.ClearAllSubscriptions;
  if (typeof clearFn !== "function") {
    // Mock fallback: clear local state only
    const nFeeds = feeds.value.length;
    const nFolders = folders.value.length;
    feeds.value = [];
    folders.value = [];
    articles.value = [];
    selectedArticleId.value = null;
    collectionId.value = "unread";
    return { feedsDeleted: nFeeds, foldersDeleted: nFolders };
  }
  const raw = await clearFn();
  const r = (raw ?? {}) as Record<string, unknown>;
  await reloadLibrary();
  selectedArticleId.value = null;
  collectionId.value = "unread";
  await reloadArticles();
  return {
    feedsDeleted: Number(r.feedsDeleted ?? 0) || 0,
    foldersDeleted: Number(r.foldersDeleted ?? 0) || 0,
  };
}

async function createFolder(name: string): Promise<void> {
  const trimmed = name.trim();
  if (!trimmed) return;
  const api = await loadAppsvc();
  const createFn = api?.FeedService?.CreateFolder;
  if (typeof createFn !== "function") {
    // Mock fallback: local-only folder
    const id = `folder-${Date.now()}`;
    folders.value = [
      ...folders.value,
      { id, name: trimmed, feedIds: [] },
    ];
    return;
  }
  await createFn(trimmed, "");
  await reloadLibrary();
}

async function bootstrap() {
  await reloadLibrary();
  if (backendReady.value) {
    await Promise.all([loadUIPrefs(), loadLibraryConfig()]);
    applyStartupPrefs();
    // Re-fetch articles if startup collection differs from default "unread" load.
    await reloadArticles();
  }
}

function selectCollection(id: CollectionId) {
  collectionId.value = id;
  selectedArticleId.value = null;
  searchQuery.value = "";
  if (backendReady.value) {
    void reloadArticles();
  }
}

async function selectArticle(id: string | null) {
  selectedArticleId.value = id;
  if (!id) return;
  const article = articles.value.find((a) => a.id === id);
  if (article && settings.markAsReadOnOpen && !article.read) {
    article.read = true;
    recomputeFeedUnread(article.feedId);
    if (backendReady.value) {
      const api = await loadAppsvc();
      await api?.ArticleService?.SetRead(id, true);
      await reloadLibraryFeedsOnly();
    }
  }
  // Prefer full content from Get when backend available
  if (backendReady.value) {
    try {
      const api = await loadAppsvc();
      const full = await api?.ArticleService?.Get(id);
      if (full) {
        const mapped = mapArticle(full) as Article;
        const idx = articles.value.findIndex((a) => a.id === id);
        if (idx >= 0) articles.value[idx] = { ...articles.value[idx], ...mapped };
      }
    } catch {
      /* ignore */
    }
  }
}

async function reloadLibraryFeedsOnly() {
  const api = await loadAppsvc();
  if (!api?.FeedService) return;
  const rawFeeds = await api.FeedService.ListFeeds();
  feeds.value = (rawFeeds ?? []).map(mapFeed) as Feed[];
}

async function toggleStar(id: string) {
  const article = articles.value.find((a) => a.id === id);
  if (!article) return;
  article.starred = !article.starred;
  if (backendReady.value) {
    const api = await loadAppsvc();
    await api?.ArticleService?.SetStarred(id, article.starred);
  }
}

async function toggleRead(id: string) {
  const article = articles.value.find((a) => a.id === id);
  if (!article) return;
  article.read = !article.read;
  recomputeFeedUnread(article.feedId);
  if (backendReady.value) {
    const api = await loadAppsvc();
    await api?.ArticleService?.SetRead(id, article.read);
    await reloadLibraryFeedsOnly();
  }
}

async function markAllRead() {
  if (backendReady.value) {
    const api = await loadAppsvc();
    await api?.ArticleService?.MarkAllRead(collectionId.value);
    await reloadLibrary();
    return;
  }
  const ids = new Set(filteredArticles.value.map((a) => a.id));
  const feedIds = new Set<string>();
  for (const article of articles.value) {
    if (ids.has(article.id) && !article.read) {
      article.read = true;
      feedIds.add(article.feedId);
    }
  }
  feedIds.forEach(recomputeFeedUnread);
}

function recomputeFeedUnread(feedId: string) {
  const feed = feeds.value.find((f) => f.id === feedId);
  if (!feed) return;
  feed.unreadCount = articles.value.filter((a) => a.feedId === feedId && !a.read).length;
}

function openAddFeed() {
  addFeedOpen.value = true;
}
function closeAddFeed() {
  addFeedOpen.value = false;
}
function openSettings() {
  settingsOpen.value = true;
}
function closeSettings() {
  settingsOpen.value = false;
}

/** Mock-only local add (fallback). Prefer addFeedFromURL with backend. */
function addFeed(input: { title: string; feedUrl: string; siteUrl?: string }) {
  const id = `feed-${Date.now()}`;
  const feed: Feed = {
    id,
    title: input.title || "New Feed",
    feedUrl: input.feedUrl,
    siteUrl: input.siteUrl || input.feedUrl,
    unreadCount: 0,
    lastFetchedAt: new Date().toISOString(),
  };
  feeds.value = [feed, ...feeds.value];
  collectionId.value = `feed:${id}`;
  selectedArticleId.value = null;
  addFeedOpen.value = false;
}

async function addFeedFromURL(url: string, _title?: string): Promise<void> {
  const api = await loadAppsvc();
  if (!api?.FeedService?.AddFeed) {
    addFeed({ title: _title || new URL(url).hostname, feedUrl: url, siteUrl: url });
    return;
  }
  const feed = await api.FeedService.AddFeed(url, "");
  await api.FeedService.RefreshFeed(feed.id);
  await reloadLibrary();
  collectionId.value = `feed:${feed.id}`;
  selectedArticleId.value = null;
  addFeedOpen.value = false;
  await reloadArticles();
}

async function refreshFeeds() {
  if (refreshing.value) return;
  refreshing.value = true;
  try {
    const api = await loadAppsvc();
    if (api?.FeedService?.RefreshAll) {
      await api.FeedService.RefreshAll();
      await reloadLibrary();
    } else {
      await new Promise((r) => setTimeout(r, 400));
      for (const feed of feeds.value) {
        feed.lastFetchedAt = new Date().toISOString();
      }
    }
  } finally {
    refreshing.value = false;
  }
}

// Kick off backend bootstrap once (browser-safe if bindings missing).
void bootstrap();

export function useRssStore() {
  return {
    folders,
    feeds,
    articles,
    collectionId,
    selectedArticleId,
    searchQuery,
    addFeedOpen,
    settingsOpen,
    refreshing,
    backendReady,
    bootstrapError,
    settings,
    smartCounts,
    collectionTitle,
    filteredArticles,
    selectedArticle,
    selectedFeed,
    selectCollection,
    selectArticle,
    toggleStar,
    toggleRead,
    markAllRead,
    openAddFeed,
    closeAddFeed,
    openSettings,
    closeSettings,
    addFeed,
    addFeedFromURL,
    refreshFeeds,
    reloadLibrary,
    persistLibraryConfig,
    persistUIPrefs,
    loadUIPrefs,
    purgeOldArticles,
    importOPMLFile,
    exportOPMLDownload,
    clearAllSubscriptions,
    createFolder,
  };
}
