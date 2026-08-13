/**
 * Real unit + structural tests for UI gaps plan.
 * Run: npx tsx src/lib/uiGaps.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  FEED_MENU_ACTIONS,
  compactSidebarClass,
  mergeArticleIntoPools,
  resolveAddFeedFolderId,
  resolveEmptyListReason,
  resolveSelectedArticle,
  shouldUseBackendSearch,
} from "./uiGaps";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// pure helpers
assert(resolveAddFeedFolderId("f1", "def") === "f1", "explicit wins");
assert(resolveAddFeedFolderId("", "def") === "def", "default used");
assert(resolveAddFeedFolderId(null, "  ") === "", "empty default");
assert(resolveAddFeedFolderId(undefined, undefined) === "", "both empty");

assert(shouldUseBackendSearch(true, "vue") === true, "backend search on");
assert(shouldUseBackendSearch(true, "  ") === false, "empty query");
assert(shouldUseBackendSearch(false, "vue") === false, "offline local");

assert(compactSidebarClass(true) === "sidebar-compact", "compact class");
assert(compactSidebarClass(false) === "", "normal class");

assert(resolveEmptyListReason({ feedCount: 0, articleCountInView: 0, hasSearchQuery: false, hasActiveFilters: false }) === "no-feeds", "no feeds");
assert(resolveEmptyListReason({ feedCount: 2, articleCountInView: 0, hasSearchQuery: true, hasActiveFilters: false }) === "no-matches", "search empty");
assert(resolveEmptyListReason({ feedCount: 2, articleCountInView: 0, hasSearchQuery: false, hasActiveFilters: true }) === "no-matches", "filter empty");
assert(resolveEmptyListReason({ feedCount: 2, articleCountInView: 0, hasSearchQuery: false, hasActiveFilters: false }) === "empty-collection", "empty coll");
// Default hideDuplicateTitles must NOT be treated as hasActiveFilters by callers —
// with only that default and empty collection, reason is empty-collection not no-matches.
assert(
  resolveEmptyListReason({
    feedCount: 5,
    articleCountInView: 0,
    hasSearchQuery: false,
    hasActiveFilters: false, // hideDuplicateTitles alone is not active filter
  }) === "empty-collection",
  "default filters do not mislabel empty collection",
);
// articleCountInView > 0 safety
assert(
  resolveEmptyListReason({
    feedCount: 0,
    articleCountInView: 3,
    hasSearchQuery: false,
    hasActiveFilters: false,
  }) === "empty-collection",
  "non-empty view never no-feeds",
);

// —— Backend search hit openable: not in collection page, only in search pool ——
const pageArticles = [
  { id: "page-1", title: "On page", contentHtml: "a" },
];
const searchHits = [
  { id: "search-only", title: "From FTS", contentHtml: "<p>body</p>", feedId: "f1" },
];
const selected = resolveSelectedArticle("search-only", pageArticles, searchHits);
assert(selected !== null && selected.id === "search-only", "search hit resolves when not in page");
assert(resolveSelectedArticle("missing", pageArticles, searchHits) === null, "missing id");
assert(
  resolveSelectedArticle("page-1", pageArticles, searchHits)?.title === "On page",
  "page hit preferred when both",
);

// merge Get() full body into search pool only
const merged = mergeArticleIntoPools(
  { id: "search-only", title: "From FTS", contentHtml: "<p>full</p>", read: true },
  pageArticles,
  searchHits,
);
assert(merged === true, "merge updates search pool");
assert(searchHits[0]!.contentHtml === "<p>full</p>", "search hit body updated");
assert(searchHits[0]!.read === true, "search hit read flag merged");
assert(
  resolveSelectedArticle("search-only", pageArticles, searchHits)?.contentHtml ===
    "<p>full</p>",
  "selected after merge has full body",
);
// page pool must not gain a ghost row for search-only id
assert(pageArticles.every((a) => a.id !== "search-only") || pageArticles.some((a) => a.id === "search-only" && a.contentHtml === "<p>full</p>"), "page unchanged or updated");
assert(pageArticles.length === 1 && pageArticles[0]!.id === "page-1", "page pool size stable");

for (const a of ["open", "refresh", "markAllRead", "rename", "pause", "delete"] as const) {
  assert(FEED_MENU_ACTIONS.includes(a), `feed menu has ${a}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const list = readFileSync(join(root, "components/article/ArticleList.vue"), "utf8");
const layout = readFileSync(join(root, "layouts/AppLayout.vue"), "utf8");
const sync = readFileSync(join(root, "components/settings/panels/SyncPanel.vue"), "utf8");
const advanced = readFileSync(join(root, "components/settings/panels/AdvancedPanel.vue"), "utf8");
const about = readFileSync(join(root, "components/settings/panels/AboutPanel.vue"), "utf8");
const general = readFileSync(join(root, "components/settings/panels/GeneralPanel.vue"), "utf8");
const feedsPanel = readFileSync(join(root, "components/settings/panels/FeedsPanel.vue"), "utf8");
const addFeed = readFileSync(join(root, "components/feed/AddFeedDialog.vue"), "utf8");
const css = readFileSync(join(root, "style.css"), "utf8");

// store wiring
assert(store.includes("resolveAddFeedFolderId"), "add feed folder resolve");
assert(store.includes("shouldUseBackendSearch"), "search decision");
assert(store.includes("SearchService"), "SearchService call");
assert(store.includes("runBackendSearch") || store.includes("Search("), "search path");
assert(store.includes("resolveSelectedArticle"), "store uses resolveSelectedArticle");
assert(store.includes("mergeArticleIntoPools"), "store merges Get into search pool");
assert(store.includes("searchArticles"), "store keeps searchArticles pool");
assert(store.includes("refreshOneFeed"), "refreshOneFeed");
assert(store.includes("markFeedRead"), "markFeedRead");
assert(store.includes("deleteFeed"), "deleteFeed");
assert(store.includes("moveFeedToFolder"), "moveFeedToFolder");
assert(store.includes("resetUIPrefsToDefaults"), "reset prefs");
assert(store.includes("exportDiagnostics"), "export diagnostics");
// emptyListReason must not treat hideDuplicateTitles alone as active filter
assert(
  store.includes("blockKeywords") && store.includes("showUnreadOnly"),
  "active filters from keywords / unread-only",
);
assert(
  !/hasActiveFilters:\s*[\s\S]*?hideDuplicateTitles/.test(
    store.slice(store.indexOf("emptyListReason"), store.indexOf("emptyListReason") + 400),
  ),
  "hideDuplicateTitles not in hasActiveFilters",
);

// sidebar
assert(sidebar.includes("compactSidebarClass") || sidebar.includes("sidebar-compact") || sidebar.includes("sidebarDensityClass"), "compact sidebar");
assert(sidebar.includes("feedMenu"), "feed menu i18n");
assert(sidebar.includes("openFeedEdit"), "feed edit from context menu");
assert(layout.includes("FeedEditDialog"), "shared feed edit dialog");
assert(sidebar.includes("onFeedRefresh") || sidebar.includes("refreshOneFeed"), "feed refresh");
assert(sidebar.includes("openDeleteFeed") || sidebar.includes("deleteFeed"), "feed delete");
assert(sidebar.includes("onFeedMove") || sidebar.includes("moveFeedToFolder"), "feed move");
assert(
  sidebar.includes("ContextMenuSub") && sidebar.includes("moveFolderTargets"),
  "feed move uses submenu",
);
assert(css.includes("sidebar-compact"), "css compact");

// search UI
assert(list.includes("setSearchQuery") || list.includes("searchModel"), "list search wired");
assert(list.includes("empty.noFeeds") || list.includes("emptyListReason"), "empty states");

// honesty
assert(sync.includes("disabled") && sync.includes("unavailable"), "sync marked unavailable");
assert(advanced.includes("resetUIPrefsToDefaults") || advanced.includes("exportDiagnostics"), "advanced real actions");
assert(!general.includes("launchAtLogin"), "general has no launch-at-login stub");
assert(
  advanced.includes("GetLaunchAtLogin") && advanced.includes("SetLaunchAtLogin"),
  "advanced wires launch-at-login",
);
assert(
  feedsPanel.includes("fetchFullContent") && feedsPanel.includes("onFetchFullContent"),
  "full content toggle wired",
);
assert(
  feedsPanel.includes("virtualWindow") && feedsPanel.includes("SettingsFeedRow"),
  "feeds list is windowed",
);
assert(about.includes("smartCounts"), "about uses smartCounts");

// add feed folder + advanced options
assert(addFeed.includes("setAddFeedFolderId") || addFeed.includes("folderModel"), "folder picker");
assert(addFeed.includes("w-full"), "folder select full width");
assert(addFeed.includes("createFolder") || addFeed.includes("newFolder"), "add feed new folder");
assert(addFeed.includes("feed.add.advanced") || addFeed.includes("advancedOpen"), "add feed advanced");
assert(addFeed.includes("isNsfw") && addFeed.includes("refreshInterval"), "add feed nsfw/interval");
assert(store.includes("AddFeedOptions") || store.includes("isNsfw"), "addFeedFromURL options");
assert(layout.includes("bootstrapError"), "bootstrap error UI");

console.log("uiGaps.selftest: OK");
