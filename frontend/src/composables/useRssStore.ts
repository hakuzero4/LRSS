import { computed, reactive, ref } from "vue";
import { articles as seedArticles, feeds as seedFeeds, folders as seedFolders } from "@/data/mock";
import { loadAppsvc, mapArticle, mapFeed, mapFolder } from "@/lib/backend";
import type {
  AppSettings,
  Article,
  CollectionId,
  Feed,
  FeedFolder,
  SmartCollectionId,
} from "@/types/rss";

const folders = ref<FeedFolder[]>(structuredClone(seedFolders));
const feeds = ref<Feed[]>(structuredClone(seedFeeds));
const articles = ref<Article[]>(structuredClone(seedArticles));

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
  if (id === "unread") return "未读";
  if (id === "today") return "今日";
  if (id === "starred") return "收藏";
  if (id === "all") return "全部文章";
  if (id.startsWith("feed:")) {
    return feeds.value.find((f) => f.id === id.slice(5))?.title ?? "订阅源";
  }
  if (id.startsWith("folder:")) {
    return folders.value.find((f) => f.id === id.slice(7))?.name ?? "文件夹";
  }
  return "订阅库";
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

async function bootstrap() {
  await reloadLibrary();
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
  };
}
