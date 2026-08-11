import { computed, reactive, ref, watch } from "vue";
import i18n from "@/i18n";
import { applyArticleFilters } from "@/lib/articleFilters";
import { loadAppsvc, mapArticle, mapFeed, mapFolder } from "@/lib/backend";
import { feedIdsInFolder, folderCollectionId } from "@/lib/folderMenu";
import { applyShowUnreadOnly } from "@/lib/readingSettings";
import { filterArticlesByNsfwMode, filterFeedsForSidebar } from "@/lib/nsfw";
import {
  mergeArticleIntoPools,
  resolveAddFeedFolderId,
  resolveEmptyListReason,
  resolveSelectedArticle,
  shouldUseBackendSearch,
} from "@/lib/uiGaps";
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
/** Articles from SearchService (null = not in backend-search mode). */
const searchArticles = ref<Article[] | null>(null);
const searchBusy = ref(false);
const searchSource = ref<"none" | "backend" | "local">("none");
let searchTimer: ReturnType<typeof setTimeout> | null = null;
let searchSeq = 0;

const addFeedOpen = ref(false);
/** When set, AddFeed uses this folder id (from folder context menu). */
const addFeedTargetFolderId = ref<string | null>(null);
const settingsOpen = ref(false);
/** Zen mode: hide feed sidebar + article list (session-only). */
const zenMode = ref(false);
const refreshing = ref(false);
const backendReady = ref(false);
const bootstrapError = ref("");

/** Right-side AI result panel (summarize / translate / ask / digest / …). */
export type AIPanelState = {
  open: boolean;
  busy: boolean;
  title: string;
  markdown: string;
  feature: string;
  model: string;
  cached: boolean;
  folderId: string;
  folderName: string;
  verdict: string;
};

const aiPanel = reactive<AIPanelState>({
  open: false,
  busy: false,
  title: "",
  markdown: "",
  feature: "",
  model: "",
  cached: false,
  folderId: "",
  folderName: "",
  verdict: "",
});

function closeAIPanel() {
  aiPanel.open = false;
  aiPanel.busy = false;
}

function openAIPanelShell(title: string, feature: string) {
  aiPanel.open = true;
  aiPanel.busy = true;
  aiPanel.title = title;
  aiPanel.feature = feature;
  aiPanel.markdown = "";
  aiPanel.model = "";
  aiPanel.cached = false;
  aiPanel.folderId = "";
  aiPanel.folderName = "";
  aiPanel.verdict = "";
}

function applyAIResult(raw: any, title: string, feature: string) {
  const r = raw ?? {};
  aiPanel.open = true;
  aiPanel.busy = false;
  aiPanel.title = title;
  aiPanel.feature = feature;
  aiPanel.markdown = String(r.markdown ?? r.Markdown ?? "");
  aiPanel.model = String(r.model ?? r.Model ?? "");
  aiPanel.cached = !!(r.cached ?? r.Cached);
  aiPanel.folderId = String(r.folderId ?? r.FolderID ?? "");
  aiPanel.folderName = String(r.folderName ?? r.FolderName ?? "");
  aiPanel.verdict = String(r.verdict ?? r.Verdict ?? "");
}

async function runAIFeature(
  title: string,
  feature: string,
  call: (api: any) => Promise<any>,
): Promise<void> {
  openAIPanelShell(title, feature);
  try {
    if (!backendReady.value) {
      throw new Error(t("ai.backendUnavailable"));
    }
    const api = await loadAppsvc();
    if (!api?.AIService) {
      throw new Error(t("ai.unavailable"));
    }
    const raw = await call(api);
    applyAIResult(raw, title, feature);
  } catch (e) {
    aiPanel.busy = false;
    aiPanel.markdown = "";
    const msg = e instanceof Error ? e.message : String(e);
    throw new Error(msg);
  }
}

async function aiSummarize(articleId: string) {
  await runAIFeature(t("ai.summarize"), "summarize", (api) =>
    api.AIService.Summarize(articleId),
  );
}

async function aiTranslate(articleId: string, targetLang: string) {
  const lang = (targetLang || "zh-CN").trim();
  await runAIFeature(t("ai.translateTo", { lang }), "translate", (api) =>
    api.AIService.Translate(articleId, lang),
  );
}

async function aiAsk(articleId: string, question: string) {
  await runAIFeature(t("ai.ask"), "ask", (api) =>
    api.AIService.Ask(articleId, question ?? ""),
  );
}

async function aiDailyDigest(limit = 12) {
  await runAIFeature(t("ai.dailyDigest"), "digest", (api) =>
    api.AIService.DailyDigest(limit),
  );
}

async function aiSuggest(articleId: string) {
  await runAIFeature(t("ai.suggest"), "suggest", (api) =>
    api.AIService.SuggestFolders(articleId),
  );
}

async function aiClassify(articleId: string) {
  await runAIFeature(t("ai.classify"), "classify", (api) =>
    api.AIService.ClassifyPromo(articleId),
  );
}

async function aiApplyFolder(articleId: string, folderId: string) {
  if (!backendReady.value) throw new Error(t("ai.backendUnavailable"));
  const api = await loadAppsvc();
  const fn = api?.AIService?.ApplySuggestedFolder;
  if (typeof fn !== "function") throw new Error(t("ai.unavailable"));
  await fn(articleId, folderId);
  await reloadLibrary();
}

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
  nsfwMode: true,
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

/**
 * Sidebar smart-list totals. Loaded from backend SmartCounts (full DB counts).
 * Must NOT be derived from the current article page (limit 200) — switching
 * collections used to make these badges "refresh"/jump incorrectly.
 */
const smartCounts = reactive({
  unread: 0,
  today: 0,
  starred: 0,
  all: 0,
} satisfies Record<SmartCollectionId, number>);

/** Offline / mock fallback: derive from currently loaded articles only. */
function applySmartCountsFromArticles() {
  const list = articles.value;
  smartCounts.unread = list.filter((a) => !a.read).length;
  smartCounts.today = list.filter((a) => isToday(a.publishedAt)).length;
  smartCounts.starred = list.filter((a) => a.starred).length;
  smartCounts.all = list.length;
}

function parseCountField(v: unknown): number {
  if (typeof v === "number" && Number.isFinite(v)) return Math.max(0, Math.floor(v));
  if (typeof v === "string" && v.trim() !== "" && Number.isFinite(Number(v))) {
    return Math.max(0, Math.floor(Number(v)));
  }
  return 0;
}

/** Apply SmartCounts payload (plain object or Wails class instance). */
function applySmartCountsPayload(raw: unknown) {
  if (!raw || typeof raw !== "object") return;
  const o = raw as Record<string, unknown>;
  // Prefer JSON tags; also accept PascalCase / getter-style.
  smartCounts.unread = parseCountField(o.unread ?? o.Unread);
  smartCounts.today = parseCountField(o.today ?? o.Today);
  smartCounts.starred = parseCountField(o.starred ?? o.Starred);
  smartCounts.all = parseCountField(o.all ?? o.All);
}

async function reloadSmartCounts() {
  const api = await loadAppsvc();
  const fn = api?.ArticleService?.SmartCounts;
  if (typeof fn !== "function") {
    if (!backendReady.value) applySmartCountsFromArticles();
    return;
  }
  try {
    const raw = await fn();
    applySmartCountsPayload(raw);
  } catch (e) {
    console.warn("[lrss] SmartCounts failed", e);
  }
}

/** Optimistic local adjust when marking one article read/unread. */
function adjustSmartUnread(delta: number) {
  smartCounts.unread = Math.max(0, smartCounts.unread + delta);
}

/**
 * After read-state mutations: refresh smart-list badges + feed unread counts.
 * Unread smart list view filters via `filteredArticles` (client-side !read);
 * we keep the selected article in `articles` so the reader pane stays open.
 */
async function syncAfterReadChange() {
  if (backendReady.value) {
    await Promise.all([reloadSmartCounts(), reloadLibraryFeedsOnly()]);
  } else {
    applySmartCountsFromArticles();
  }
}

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
  // Backend search results replace collection list while a query is active.
  const usingBackendHits = searchArticles.value != null && searchQuery.value.trim().length > 0;
  let list = usingBackendHits ? [...searchArticles.value!] : [...articles.value];

  if (!usingBackendHits) {
    // When backend is ready, `articles` is already scoped by collection via reloadArticles.
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
    // Always drop read items from the Unread smart list (even when list came from backend).
    if (collectionId.value === "unread") {
      list = list.filter((a) => !a.read);
    }
  }

  // Settings → Reading: show unread only (starred collection exempt).
  list = applyShowUnreadOnly(list, settings.showUnreadOnly, collectionId.value);

  const q = searchQuery.value.trim().toLowerCase();
  // Local filter only when not showing backend search hits.
  if (q && !usingBackendHits) {
    list = list.filter(
      (a) =>
        a.title.toLowerCase().includes(q) ||
        a.summary.toLowerCase().includes(q) ||
        (a.author?.toLowerCase().includes(q) ?? false),
    );
  }

  // Newest first so hide-duplicate keeps the latest article per title.
  list.sort(
    (a, b) => new Date(b.publishedAt).getTime() - new Date(a.publishedAt).getTime(),
  );
  // Office mode: hide NSFW only on smart lists. Explicit feed:/folder: keeps content
  // visible (e.g. just-added sensitive feed after subscribe).
  {
    const col = collectionId.value;
    const smart =
      col === "unread" || col === "today" || col === "starred" || col === "all";
    if (smart) {
      list = filterArticlesByNsfwMode(list, feeds.value, settings.nsfwMode, folders.value);
    }
  }
  // Settings → Filters: block keywords + optional duplicate titles.
  list = applyArticleFilters(list, {
    hideDuplicateTitles: settings.hideDuplicateTitles,
    blockKeywords: settings.blockKeywords,
  });
  return list;
});

/** Folders visible in the sidebar (office mode hides isNsfw folders). */
const sidebarFolders = computed(() =>
  settings.nsfwMode ? folders.value : folders.value.filter((f) => !f.isNsfw),
);

/** Feeds visible in the sidebar (respects feed + folder nsfw). Settings list uses raw `feeds`. */
const sidebarFeeds = computed(() =>
  filterFeedsForSidebar(feeds.value, settings.nsfwMode, folders.value),
);

const emptyListReason = computed(() =>
  resolveEmptyListReason({
    feedCount: feeds.value.length,
    articleCountInView: filteredArticles.value.length,
    hasSearchQuery: searchQuery.value.trim().length > 0,
    // Only user-driven filters that can empty the list — not default-on hideDuplicateTitles.
    hasActiveFilters:
      !!settings.blockKeywords.trim() ||
      (settings.showUnreadOnly && collectionId.value !== "starred"),
  }),
);

/** Prefer collection page, then backend search hits (may not be in current page). */
const selectedArticle = computed(() =>
  resolveSelectedArticle(selectedArticleId.value, articles.value, searchArticles.value),
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
    await Promise.all([reloadArticles(), reloadSmartCounts()]);
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
  // Do not recompute smartCounts here — list is paginated; counts stay stable
  // across collection switches until an explicit reloadSmartCounts().
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
const READER_WIDTHS = new Set(["narrow", "medium", "wide", "fill"]);

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
    nsfwMode: settings.nsfwMode,
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

  const nsfw = pickBool(p, "nsfwMode", "NsfwMode");
  if (nsfw !== undefined) settings.nsfwMode = nsfw;
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

/** Create a folder; returns its id (empty if name blank). */
async function createFolder(name: string): Promise<string> {
  const trimmed = name.trim();
  if (!trimmed) return "";
  const api = await loadAppsvc();
  const createFn = api?.FeedService?.CreateFolder;
  if (typeof createFn !== "function") {
    // Mock fallback: local-only folder
    const id = `folder-${Date.now()}`;
    folders.value = [
      ...folders.value,
      { id, name: trimmed, feedIds: [] },
    ];
    return id;
  }
  const created = await createFn(trimmed, "");
  await reloadLibrary();
  const id =
    (created && typeof created === "object" && (created as { id?: string }).id) ||
    folders.value.find((f) => f.name === trimmed)?.id ||
    "";
  return String(id || "");
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
  searchArticles.value = null;
  searchSource.value = "none";
  if (backendReady.value) {
    // Only reload the article list — smartCounts must stay put.
    void reloadArticles();
  } else {
    applySmartCountsFromArticles();
  }
}

async function selectArticle(id: string | null) {
  selectedArticleId.value = id;
  if (!id) return;
  // Search hits may not be on the current collection page.
  let article =
    resolveSelectedArticle(id, articles.value, searchArticles.value) ?? undefined;
  let markedRead = false;
  if (article && settings.markAsReadOnOpen && !article.read) {
    article.read = true;
    // Mirror into both pools so list + reader stay consistent.
    mergeArticleIntoPools(article, articles.value, searchArticles.value);
    recomputeFeedUnread(article.feedId);
    adjustSmartUnread(-1);
    markedRead = true;
    if (backendReady.value) {
      const api = await loadAppsvc();
      try {
        await api?.ArticleService?.SetRead(id, true);
      } catch (e) {
        article.read = false;
        mergeArticleIntoPools(article, articles.value, searchArticles.value);
        recomputeFeedUnread(article.feedId);
        adjustSmartUnread(1);
        markedRead = false;
        console.warn("[lrss] SetRead failed", e);
      }
    }
  }
  // Prefer full content from Get when backend available
  if (backendReady.value) {
    try {
      const api = await loadAppsvc();
      const full = await api?.ArticleService?.Get(id);
      if (full) {
        const mapped = mapArticle(full) as Article;
        // Preserve local read=true if we just marked it (avoids flash if Get races).
        if (markedRead) mapped.read = true;
        const updated = mergeArticleIntoPools(
          mapped,
          articles.value,
          searchArticles.value,
        );
        // Hit only in search results and Get succeeded — ensure search pool has full body.
        if (!updated && searchArticles.value) {
          const sidx = searchArticles.value.findIndex((a) => a.id === id);
          if (sidx >= 0) {
            searchArticles.value[sidx] = {
              ...searchArticles.value[sidx],
              ...mapped,
            };
          } else {
            searchArticles.value = [...searchArticles.value, mapped];
          }
        } else if (!updated) {
          // Neither pool had it (edge): drop onto search or articles so reader can show it.
          if (searchArticles.value) {
            searchArticles.value = [...searchArticles.value, mapped];
          } else {
            articles.value = [...articles.value, mapped];
          }
        }
      }
    } catch {
      /* ignore */
    }
  }
  if (markedRead) {
    await syncAfterReadChange();
  }
}

async function reloadLibraryFeedsOnly() {
  const api = await loadAppsvc();
  if (!api?.FeedService) return;
  const rawFeeds = await api.FeedService.ListFeeds();
  feeds.value = (rawFeeds ?? []).map(mapFeed) as Feed[];
}

async function toggleStar(id: string) {
  const article = resolveSelectedArticle(id, articles.value, searchArticles.value);
  if (!article) return;
  const next = !article.starred;
  article.starred = next;
  mergeArticleIntoPools(article, articles.value, searchArticles.value);
  smartCounts.starred = Math.max(0, smartCounts.starred + (next ? 1 : -1));
  if (backendReady.value) {
    const api = await loadAppsvc();
    try {
      await api?.ArticleService?.SetStarred(id, next);
    } catch (e) {
      article.starred = !next;
      mergeArticleIntoPools(article, articles.value, searchArticles.value);
      smartCounts.starred = Math.max(0, smartCounts.starred + (next ? -1 : 1));
      console.warn("[lrss] SetStarred failed", e);
    }
  }
}

async function toggleRead(id: string) {
  const article = resolveSelectedArticle(id, articles.value, searchArticles.value);
  if (!article) return;
  const next = !article.read;
  article.read = next;
  mergeArticleIntoPools(article, articles.value, searchArticles.value);
  recomputeFeedUnread(article.feedId);
  adjustSmartUnread(next ? -1 : 1);
  if (backendReady.value) {
    const api = await loadAppsvc();
    try {
      await api?.ArticleService?.SetRead(id, next);
    } catch (e) {
      article.read = !next;
      mergeArticleIntoPools(article, articles.value, searchArticles.value);
      recomputeFeedUnread(article.feedId);
      adjustSmartUnread(next ? 1 : -1);
      console.warn("[lrss] SetRead failed", e);
      return;
    }
  }
  await syncAfterReadChange();
}

/**
 * Fetch original page HTML for an article (when feed only has partial content).
 * Uses backend ArticleService.FetchFullContent (surf fingerprint HTTP + readability).
 * Returns the updated mapped article, or throws with a message.
 */
async function fetchFullContent(id: string): Promise<Article> {
  if (!backendReady.value) {
    throw new Error("backend unavailable");
  }
  const api = await loadAppsvc();
  const fn = api?.ArticleService?.FetchFullContent;
  if (typeof fn !== "function") {
    throw new Error("FetchFullContent unavailable");
  }
  const full = await fn(id);
  if (!full) {
    throw new Error("empty response");
  }
  const mapped = mapArticle(full) as Article;
  const updated = mergeArticleIntoPools(mapped, articles.value, searchArticles.value);
  if (!updated && searchArticles.value) {
    const sidx = searchArticles.value.findIndex((a) => a.id === id);
    if (sidx >= 0) {
      searchArticles.value[sidx] = { ...searchArticles.value[sidx], ...mapped };
    } else {
      searchArticles.value = [...searchArticles.value, mapped];
    }
  } else if (!updated) {
    articles.value = [...articles.value, mapped];
  }
  return mapped;
}

async function markAllRead() {
  if (backendReady.value) {
    const api = await loadAppsvc();
    await api?.ArticleService?.MarkAllRead(collectionId.value);
    // Full reload: articles page + feeds + smart counts
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
  applySmartCountsFromArticles();
}

function recomputeFeedUnread(feedId: string) {
  const feed = feeds.value.find((f) => f.id === feedId);
  if (!feed) return;
  feed.unreadCount = articles.value.filter((a) => a.feedId === feedId && !a.read).length;
}

function openAddFeed() {
  // Prefer explicit default folder from settings when opening from the chrome.
  const def = (settings.defaultFolderId ?? "").trim();
  addFeedTargetFolderId.value = def || null;
  addFeedOpen.value = true;
}

/** Open add-feed dialog with a default folder (folder context menu). */
function openAddFeedInFolder(folderId: string) {
  const id = folderId.trim();
  addFeedTargetFolderId.value = id || null;
  addFeedOpen.value = true;
}

function setAddFeedFolderId(folderId: string | null) {
  const id = (folderId ?? "").trim();
  addFeedTargetFolderId.value = id || null;
}

function closeAddFeed() {
  addFeedOpen.value = false;
  addFeedTargetFolderId.value = null;
}

/** Debounced search: backend SearchService when ready, else local filter only. */
function scheduleSearch(q: string) {
  if (searchTimer) clearTimeout(searchTimer);
  const trimmed = q.trim();
  if (!trimmed) {
    searchArticles.value = null;
    searchSource.value = "none";
    searchBusy.value = false;
    return;
  }
  if (!shouldUseBackendSearch(backendReady.value, trimmed)) {
    searchArticles.value = null;
    searchSource.value = "local";
    searchBusy.value = false;
    return;
  }
  searchBusy.value = true;
  searchSource.value = "backend";
  const seq = ++searchSeq;
  searchTimer = setTimeout(() => {
    void runBackendSearch(trimmed, seq);
  }, 280);
}

function setSearchQuery(q: string) {
  searchQuery.value = q;
  // watch also fires; keep for programmatic callers that expect immediate scheduling
  scheduleSearch(q);
}

watch(searchQuery, (q) => {
  scheduleSearch(q);
});

async function runBackendSearch(query: string, seq: number) {
  const api = await loadAppsvc();
  const fn = api?.SearchService?.Search;
  if (typeof fn !== "function") {
    if (seq === searchSeq) {
      searchArticles.value = null;
      searchSource.value = "local";
      searchBusy.value = false;
    }
    return;
  }
  try {
    const raw = await fn(query, "auto", 80);
    if (seq !== searchSeq) return;
    const hits = (raw?.hits ?? raw?.Hits ?? []) as Array<{
      articleId?: string;
      ArticleId?: string;
    }>;
    const ids = hits
      .map((h) => h.articleId ?? h.ArticleId ?? "")
      .filter(Boolean) as string[];
    const getFn = api?.ArticleService?.Get;
    const out: Article[] = [];
    if (typeof getFn === "function") {
      for (const id of ids) {
        try {
          const full = await getFn(id);
          if (full) out.push(mapArticle(full) as Article);
        } catch {
          /* skip missing */
        }
      }
    }
    searchArticles.value = out;
    searchSource.value = "backend";
  } catch (e) {
    console.warn("[lrss] Search failed", e);
    if (seq === searchSeq) {
      searchArticles.value = null;
      searchSource.value = "local";
    }
  } finally {
    if (seq === searchSeq) searchBusy.value = false;
  }
}
function openSettings() {
  settingsOpen.value = true;
}
function closeSettings() {
  settingsOpen.value = false;
}

/** Toggle zen mode (hide sidebar + article list). Session-only. */
function toggleZenMode() {
  zenMode.value = !zenMode.value;
}

function setZenMode(on: boolean) {
  zenMode.value = !!on;
}

/** Options for Add Feed advanced settings (applied after subscribe). */
export type AddFeedOptions = {
  title?: string;
  isNsfw?: boolean;
  /** 0 = follow global default. */
  refreshIntervalMinutes?: number;
  /**
   * Folder for this add. `undefined` → resolve from dialog target / default.
   * `null` or `""` → unfiled.
   */
  folderId?: string | null;
  /** Jump to the new feed after add (default true). */
  selectAfter?: boolean;
  /** Close the chrome add-feed dialog (default true). */
  closeDialog?: boolean;
  /** Reload library + articles after add (default true). Batch uses false until end. */
  reload?: boolean;
};

export type AddFeedsBatchResult = {
  added: number;
  failed: { url: string; message: string }[];
  lastFeedId?: string;
};

/** Mock-only local add (fallback). Prefer addFeedFromURL with backend. */
function addFeed(input: {
  title: string;
  feedUrl: string;
  siteUrl?: string;
  folderId?: string | null;
  isNsfw?: boolean;
  refreshIntervalMinutes?: number;
  selectAfter?: boolean;
  closeDialog?: boolean;
}): string {
  const id = `feed-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;
  const folderId = input.folderId?.trim() || undefined;
  const feed: Feed = {
    id,
    title: input.title || "New Feed",
    feedUrl: input.feedUrl,
    siteUrl: input.siteUrl || input.feedUrl,
    folderId,
    unreadCount: 0,
    lastFetchedAt: new Date().toISOString(),
    isPaused: false,
    refreshIntervalMinutes: input.refreshIntervalMinutes ?? 0,
    isNsfw: !!input.isNsfw,
  };
  feeds.value = [feed, ...feeds.value];
  if (folderId) {
    const folder = folders.value.find((f) => f.id === folderId);
    if (folder && !folder.feedIds.includes(id)) {
      folder.feedIds = [...folder.feedIds, id];
    }
  }
  if (input.selectAfter !== false) {
    collectionId.value = `feed:${id}`;
    selectedArticleId.value = null;
  }
  if (input.closeDialog !== false) {
    closeAddFeed();
  }
  return id;
}

/**
 * Subscribe to one feed URL. Returns the feed id.
 * Does a single backend fetch (no extra Refresh) unless batch opts disable reloads.
 */
async function addFeedFromURL(url: string, opts?: string | AddFeedOptions): Promise<string> {
  // Back-compat: second arg may be a plain title string.
  const options: AddFeedOptions =
    typeof opts === "string" ? { title: opts } : opts ?? {};
  const customTitle = options.title?.trim() || "";
  const isNsfw = !!options.isNsfw;
  let interval = Math.max(0, Math.floor(Number(options.refreshIntervalMinutes) || 0));
  if (interval > 0 && interval < 5) interval = 5;
  if (interval > 180) interval = 180;
  const selectAfter = options.selectAfter !== false;
  const closeDialog = options.closeDialog !== false;
  const doReload = options.reload !== false;

  const folderId =
    options.folderId !== undefined
      ? (options.folderId?.trim() || "")
      : resolveAddFeedFolderId(addFeedTargetFolderId.value, settings.defaultFolderId);

  const api = await loadAppsvc();
  if (!api?.FeedService?.AddFeed) {
    return addFeed({
      title: customTitle || new URL(url).hostname,
      feedUrl: url,
      siteUrl: url,
      folderId: folderId || null,
      isNsfw,
      refreshIntervalMinutes: interval,
      selectAfter,
      closeDialog,
    });
  }
  // Empty folderId → unfiled (backend contract).
  const feed = await api.FeedService.AddFeed(url, folderId);
  const feedId = String(feed.id);

  // Apply advanced options after the feed exists (best-effort; subscribe already succeeded).
  if (customTitle && typeof api.FeedService.RenameFeed === "function") {
    try {
      await api.FeedService.RenameFeed(feedId, customTitle);
    } catch (e) {
      console.warn("[lrss] RenameFeed after add failed", e);
    }
  }
  if (isNsfw && typeof api.FeedService.SetFeedNsfw === "function") {
    try {
      await api.FeedService.SetFeedNsfw(feedId, true);
    } catch (e) {
      console.warn("[lrss] SetFeedNsfw after add failed", e);
    }
  }
  if (interval > 0 && typeof api.FeedService.SetFeedRefreshInterval === "function") {
    try {
      await api.FeedService.SetFeedRefreshInterval(feedId, interval);
    } catch (e) {
      console.warn("[lrss] SetFeedRefreshInterval after add failed", e);
    }
  }

  if (doReload) {
    await reloadLibrary();
  }
  if (selectAfter) {
    collectionId.value = `feed:${feedId}`;
    selectedArticleId.value = null;
  }
  if (closeDialog) {
    closeAddFeed();
  }
  if (doReload) {
    await reloadArticles();
    if (isNsfw && !settings.nsfwMode) {
      await reloadSmartCounts();
    }
  }
  return feedId;
}

/**
 * Subscribe to many feed URLs sequentially (one network fetch each).
 * Reloads the library once at the end. Does not close the chrome add dialog.
 */
async function addFeedsFromURLs(
  urls: string[],
  opts?: Omit<AddFeedOptions, "title" | "selectAfter" | "closeDialog" | "reload"> & {
    selectLast?: boolean;
    /** 1-based progress after each URL attempt. */
    onProgress?: (current: number, total: number, url: string) => void;
  },
): Promise<AddFeedsBatchResult> {
  const list = urls.map((u) => u.trim()).filter(Boolean);
  const result: AddFeedsBatchResult = { added: 0, failed: [] };
  if (list.length === 0) return result;

  for (let i = 0; i < list.length; i++) {
    const url = list[i]!;
    opts?.onProgress?.(i + 1, list.length, url);
    try {
      // eslint-disable-next-line no-new
      new URL(url);
      const id = await addFeedFromURL(url, {
        isNsfw: opts?.isNsfw,
        refreshIntervalMinutes: opts?.refreshIntervalMinutes,
        folderId: opts?.folderId,
        selectAfter: false,
        closeDialog: false,
        reload: false,
      });
      result.added++;
      result.lastFeedId = id;
    } catch (e: unknown) {
      result.failed.push({
        url,
        message: e instanceof Error ? e.message : String(e),
      });
    }
  }

  await reloadLibrary();
  if (opts?.selectLast !== false && result.lastFeedId) {
    collectionId.value = `feed:${result.lastFeedId}`;
    selectedArticleId.value = null;
  }
  await reloadArticles();
  if (opts?.isNsfw && !settings.nsfwMode) {
    await reloadSmartCounts();
  }
  return result;
}

async function refreshOneFeed(feedId: string): Promise<number> {
  const api = await loadAppsvc();
  const fn = api?.FeedService?.RefreshFeed;
  if (typeof fn !== "function") return 0;
  const res = await fn(feedId);
  await reloadLibrary();
  let added = 0;
  if (res && typeof res === "object") {
    added = Math.max(0, Math.floor(Number((res as { added?: number }).added ?? 0) || 0));
  }
  return added;
}

async function markFeedRead(feedId: string): Promise<void> {
  const api = await loadAppsvc();
  const fn = api?.ArticleService?.MarkAllRead;
  if (typeof fn === "function") {
    await fn(`feed:${feedId}`);
    await reloadLibrary();
    return;
  }
  for (const a of articles.value) {
    if (a.feedId === feedId) a.read = true;
  }
  recomputeFeedUnread(feedId);
  applySmartCountsFromArticles();
}

async function deleteFeed(feedId: string): Promise<void> {
  const api = await loadAppsvc();
  const fn = api?.FeedService?.DeleteFeed;
  if (typeof fn === "function") {
    await fn(feedId);
  }
  feeds.value = feeds.value.filter((f) => f.id !== feedId);
  articles.value = articles.value.filter((a) => a.feedId !== feedId);
  if (collectionId.value === `feed:${feedId}`) {
    collectionId.value = "unread";
    selectedArticleId.value = null;
  }
  await reloadLibrary();
}

async function moveFeedToFolder(feedId: string, folderId: string | null): Promise<void> {
  const api = await loadAppsvc();
  const fn = api?.FeedService?.MoveFeed;
  const folder = (folderId ?? "").trim();
  if (typeof fn === "function") {
    await fn(feedId, folder);
  }
  const feed = feeds.value.find((f) => f.id === feedId);
  if (feed) feed.folderId = folder || undefined;
  // Rebuild folder.feedIds
  for (const f of folders.value) {
    f.feedIds = feeds.value.filter((x) => x.folderId === f.id).map((x) => x.id);
  }
}

/** Rename a folder (display name). */
async function renameFolder(id: string, name: string): Promise<void> {
  const trimmed = name.trim();
  if (!trimmed) throw new Error(t("folderMenu.renameEmpty"));
  const api = await loadAppsvc();
  const fn = api?.FeedService?.RenameFolder;
  if (typeof fn === "function") {
    await fn(id, trimmed);
    const folder = folders.value.find((f) => f.id === id);
    if (folder) folder.name = trimmed;
    return;
  }
  const folder = folders.value.find((f) => f.id === id);
  if (folder) folder.name = trimmed;
}

/**
 * Delete a folder. Feeds become unfiled (not deleted).
 * Caller must confirm in UI first.
 */
async function deleteFolder(id: string): Promise<void> {
  const api = await loadAppsvc();
  const fn = api?.FeedService?.DeleteFolder;
  if (typeof fn === "function") {
    await fn(id);
    // Unfile feeds locally then drop folder; keep badges consistent.
    for (const feed of feeds.value) {
      if (feed.folderId === id) feed.folderId = undefined;
    }
    folders.value = folders.value.filter((f) => f.id !== id);
    if (collectionId.value === folderCollectionId(id)) {
      collectionId.value = "unread";
      selectedArticleId.value = null;
      await reloadArticles();
    }
    await reloadLibraryFeedsOnly();
    // Refresh folder list from backend when available.
    try {
      const rawFolders = await api.FeedService.ListFolders();
      const mappedFeeds = feeds.value;
      folders.value = (rawFolders ?? []).map((f: any) => mapFolder(f, mappedFeeds)) as FeedFolder[];
    } catch {
      /* local state already updated */
    }
    return;
  }
  for (const feed of feeds.value) {
    if (feed.folderId === id) feed.folderId = undefined;
  }
  folders.value = folders.value.filter((f) => f.id !== id);
  if (collectionId.value === folderCollectionId(id)) {
    collectionId.value = "unread";
    selectedArticleId.value = null;
  }
}

/** Mark all articles in a folder as read via ArticleService.MarkAllRead. */
async function markFolderRead(folderId: string): Promise<void> {
  const collection = folderCollectionId(folderId);
  const api = await loadAppsvc();
  const fn = api?.ArticleService?.MarkAllRead;
  if (typeof fn === "function") {
    await fn(collection);
    await reloadLibrary();
    return;
  }
  const ids = new Set(feedIdsInFolder(folderId, feeds.value));
  for (const article of articles.value) {
    if (ids.has(article.feedId) && !article.read) {
      article.read = true;
    }
  }
  for (const feedId of ids) recomputeFeedUnread(feedId);
  applySmartCountsFromArticles();
}

/**
 * Refresh every feed in a folder (RefreshFeed each), then reload library.
 * Returns total articles added when backend reports them.
 */
async function refreshFolderFeeds(folderId: string): Promise<{ refreshed: number; added: number }> {
  const ids = feedIdsInFolder(folderId, feeds.value);
  if (ids.length === 0) return { refreshed: 0, added: 0 };

  const api = await loadAppsvc();
  const refreshFn = api?.FeedService?.RefreshFeed;
  if (typeof refreshFn !== "function") {
    const now = new Date().toISOString();
    for (const feed of feeds.value) {
      if (ids.includes(feed.id)) feed.lastFetchedAt = now;
    }
    return { refreshed: ids.length, added: 0 };
  }

  let added = 0;
  let refreshed = 0;
  for (const id of ids) {
    try {
      const res = await refreshFn(id);
      refreshed++;
      if (res && typeof res === "object") {
        const n = Number((res as { added?: number }).added ?? 0);
        if (Number.isFinite(n)) added += Math.max(0, Math.floor(n));
      }
    } catch (e) {
      console.warn("[lrss] RefreshFeed failed", id, e);
    }
  }
  await reloadLibrary();
  return { refreshed, added };
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

/** Patch one feed in place so sidebar badges / list keys do not thrash. */
function patchFeedLocal(
  id: string,
  patch: Partial<Pick<Feed, "title" | "refreshIntervalMinutes" | "isPaused" | "isNsfw">>,
) {
  const feed = feeds.value.find((f) => f.id === id);
  if (!feed) return;
  if (patch.title !== undefined) feed.title = patch.title;
  if (patch.refreshIntervalMinutes !== undefined) {
    feed.refreshIntervalMinutes = patch.refreshIntervalMinutes;
  }
  if (patch.isPaused !== undefined) feed.isPaused = patch.isPaused;
  if (patch.isNsfw !== undefined) feed.isNsfw = patch.isNsfw;
}

/** Rename a subscription (locks title against remote overwrites). */
async function renameFeed(id: string, title: string): Promise<void> {
  const trimmed = title.trim();
  if (!trimmed) throw new Error(t("settings.feeds.renameEmpty"));
  const api = await loadAppsvc();
  const fn = api?.FeedService?.RenameFeed;
  if (typeof fn === "function") {
    await fn(id, trimmed);
    patchFeedLocal(id, { title: trimmed });
    return;
  }
  patchFeedLocal(id, { title: trimmed });
}

/**
 * Set per-feed auto-refresh interval in minutes.
 * 0 = follow global default; otherwise clamped by backend to [5, 180].
 */
async function setFeedRefreshInterval(id: string, minutes: number): Promise<void> {
  const n = Math.max(0, Math.floor(Number(minutes) || 0));
  const api = await loadAppsvc();
  const fn = api?.FeedService?.SetFeedRefreshInterval;
  if (typeof fn === "function") {
    await fn(id, n);
    // Apply clamped client-side mirror (backend clamps 0 | [5,180]).
    let applied = n;
    if (applied > 0 && applied < 5) applied = 5;
    if (applied > 180) applied = 180;
    patchFeedLocal(id, { refreshIntervalMinutes: applied });
    return;
  }
  patchFeedLocal(id, { refreshIntervalMinutes: n });
}

async function setFeedPaused(id: string, paused: boolean): Promise<void> {
  const api = await loadAppsvc();
  const fn = api?.FeedService?.SetFeedPaused;
  if (typeof fn === "function") {
    await fn(id, paused);
    patchFeedLocal(id, { isPaused: paused });
    return;
  }
  patchFeedLocal(id, { isPaused: paused });
}

async function setFeedNsfw(id: string, nsfw: boolean): Promise<void> {
  const api = await loadAppsvc();
  const fn = api?.FeedService?.SetFeedNsfw;
  if (typeof fn === "function") {
    await fn(id, nsfw);
  }
  const feed = feeds.value.find((f) => f.id === id);
  if (feed) feed.isNsfw = nsfw;
  // Smart lists/counts depend on office mode — refresh after mark.
  if (!settings.nsfwMode) {
    await Promise.all([reloadArticles(), reloadSmartCounts()]);
  }
}

async function setFolderNsfw(id: string, nsfw: boolean): Promise<void> {
  const api = await loadAppsvc();
  const fn = api?.FeedService?.SetFolderNsfw;
  if (typeof fn === "function") {
    await fn(id, nsfw);
  }
  const folder = folders.value.find((f) => f.id === id);
  if (folder) folder.isNsfw = nsfw;
  if (!settings.nsfwMode) {
    // Leaving a NSFW folder while office is on — jump away if it disappears.
    if (nsfw && collectionId.value === `folder:${id}`) {
      collectionId.value = "unread";
      selectedArticleId.value = null;
    }
    await Promise.all([reloadArticles(), reloadSmartCounts()]);
  }
}

/** Toggle show-sensitive / office hide; persists UIPrefs and reloads lists. */
async function setNsfwMode(enabled: boolean): Promise<void> {
  if (settings.nsfwMode === enabled) return;
  settings.nsfwMode = enabled;
  await persistUIPrefs(true);
  await reloadLibrary();
}

/** Reset UI prefs to defaults (does not delete feeds/articles). */
async function resetUIPrefsToDefaults(): Promise<void> {
  const defaults: UIPrefs = {
    markAsReadOnOpen: true,
    markAsReadOnScrollEnd: false,
    openOnStartup: "unread",
    hideReadOnStartup: false,
    theme: "light",
    accent: "purple",
    compactSidebar: false,
    fontSize: "md",
    showUnreadOnly: false,
    openLinksInBrowser: true,
    readerWidth: "medium",
    defaultFolderId: "",
    fetchFullContent: false,
    keepArticlesDays: 90,
    hideDuplicateTitles: true,
    blockKeywords: "",
    enableKeyboardShortcuts: true,
    notifyOnNewArticles: false,
    notifySound: false,
    hardwareAcceleration: true,
    clearCacheOnQuit: false,
    developerMode: false,
    nsfwMode: true,
  };
  applyUIPrefs(defaults);
  settings.autoRefresh = true;
  settings.refreshIntervalMinutes = 30;
  settings.launchAtLogin = false;
  settings.syncEnabled = false;
  settings.syncProvider = "none";
  await persistUIPrefs(true);
  await persistLibraryConfig();
}

/** Download a small diagnostics JSON for support. */
async function exportDiagnostics(): Promise<void> {
  const payload = {
    exportedAt: new Date().toISOString(),
    backendReady: backendReady.value,
    bootstrapError: bootstrapError.value,
    feedCount: feeds.value.length,
    folderCount: folders.value.length,
    smartCounts: { ...smartCounts },
    settings: JSON.parse(JSON.stringify(settings)),
  };
  const blob = new Blob([JSON.stringify(payload, null, 2)], {
    type: "application/json;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  try {
    const a = document.createElement("a");
    a.href = url;
    a.download = `lrss-diagnostics-${Date.now()}.json`;
    a.rel = "noopener";
    document.body.appendChild(a);
    a.click();
    a.remove();
  } finally {
    URL.revokeObjectURL(url);
  }
}

// Kick off backend bootstrap once (browser-safe if bindings missing).
void bootstrap();

export function useRssStore() {
  return {
    folders,
    feeds,
    sidebarFolders,
    sidebarFeeds,
    articles,
    collectionId,
    selectedArticleId,
    searchQuery,
    searchBusy,
    searchSource,
    emptyListReason,
    addFeedOpen,
    addFeedTargetFolderId,
    settingsOpen,
    zenMode,
    aiPanel,
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
    fetchFullContent,
    markAllRead,
    openAddFeed,
    openAddFeedInFolder,
    setAddFeedFolderId,
    closeAddFeed,
    setSearchQuery,
    openSettings,
    closeSettings,
    toggleZenMode,
    setZenMode,
    closeAIPanel,
    aiSummarize,
    aiTranslate,
    aiAsk,
    aiDailyDigest,
    aiSuggest,
    aiClassify,
    aiApplyFolder,
    addFeed,
    addFeedFromURL,
    addFeedsFromURLs,
    refreshFeeds,
    refreshOneFeed,
    markFeedRead,
    deleteFeed,
    moveFeedToFolder,
    reloadLibrary,
    persistLibraryConfig,
    persistUIPrefs,
    loadUIPrefs,
    resetUIPrefsToDefaults,
    exportDiagnostics,
    purgeOldArticles,
    importOPMLFile,
    exportOPMLDownload,
    clearAllSubscriptions,
    createFolder,
    renameFolder,
    deleteFolder,
    markFolderRead,
    refreshFolderFeeds,
    renameFeed,
    setFeedRefreshInterval,
    setFeedPaused,
    setFeedNsfw,
    setFolderNsfw,
    setNsfwMode,
  };
}
