<script setup lang="ts">
import {
  BookOpenText,
  CalendarDays,
  ChevronsDownUp,
  ChevronsUpDown,
  ChevronRight,
  Clock,
  Folder,
  FolderPlus,
  Inbox,
  ListFilter,
  LocateFixed,
  Newspaper,
  Settings,
  Sparkles,
  Star,
  X,
} from "@lucide/vue";
import { computed, nextTick, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import {
  loadCollapsedFolders,
  pruneCollapsedFolders,
  saveCollapsedFolders,
} from "@/lib/folderCollapse";
import { folderCollectionId } from "@/lib/folderMenu";
import { compactSidebarClass } from "@/lib/uiGaps";
import { cn } from "@/lib/utils";
import type { CollectionId, Feed, FeedFolder, FolderDisplayMode } from "@/types/rss";
import FeedIcon from "@/components/feed/FeedIcon.vue";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { TriangleAlert } from "@lucide/vue";

const { t } = useI18n();

const {
  folders,
  feeds,
  sidebarFolders,
  sidebarFeeds,
  smartCounts,
  briefingUnreadCount,
  collectionId,
  settings,
  libraryLoading,
  webMode,
  selectCollection,
  selectedArticle,
  selectedFeed,
  openAddFeedInFolder,
  openSettings,
  assistant,
  toggleAssistant,
  openFeedEdit,
  createFolder,
  renameFolder,
  deleteFolder,
  markFolderRead,
  refreshFolderFeeds,
  refreshOneFeed,
  markFeedRead,
  renameFeed,
  setFeedPaused,
  setFeedNsfw,
  setFolderNsfw,
  setFolderDisplayMode,
  moveFeedToFolder,
  deleteFeed,
} = useRssStore();

/** true = folder children hidden; persisted in localStorage (`lrss.folderCollapsed`). */
const collapsedFolders = ref<Record<string, boolean>>(loadCollapsedFolders());
const creatingFolder = ref(false);
const folderBusyId = ref<string | null>(null);
const feedBusyId = ref<string | null>(null);

/** Sidebar feed/folder name filter (client-side only). */
const feedFilterOpen = ref(false);
const feedFilterQuery = ref("");
const feedFilterInputEl = ref<HTMLInputElement | null>(null);

watch(collapsedFolders, (map) => {
  saveCollapsedFolders(map);
});

// Drop deleted folder ids from the map (and storage via the watch above).
watch(
  () => folders.value.map((f) => f.id).join("\0"),
  (ids) => {
    const next = pruneCollapsedFolders(collapsedFolders.value, ids ? ids.split("\0") : []);
    if (next !== collapsedFolders.value) {
      collapsedFolders.value = next;
    }
  },
);

const sidebarDensityClass = computed(() => compactSidebarClass(settings.compactSidebar));

// Rename dialog (folder or feed)
const renameOpen = ref(false);
const renameKind = ref<"folder" | "feed">("folder");
const renameFolderId = ref<string | null>(null);
const renameFeedId = ref<string | null>(null);
const renameDraft = ref("");
const renameSaving = ref(false);

// Delete confirm (folder or feed)
const deleteOpen = ref(false);
const deleteKind = ref<"folder" | "feed">("folder");
const deleteTarget = ref<FeedFolder | null>(null);
const deleteFeedTarget = ref<Feed | null>(null);
const deleteBusy = ref(false);

/** Sidebar-only feeds (office mode hides isNsfw). */
const unfiledFeeds = computed(() => sidebarFeeds.value.filter((f) => !f.folderId));

const feedFilterQueryDebounced = ref("");
let feedFilterTimer: ReturnType<typeof setTimeout> | null = null;
watch(feedFilterQuery, (v) => {
  if (feedFilterTimer) clearTimeout(feedFilterTimer);
  feedFilterTimer = setTimeout(() => {
    feedFilterQueryDebounced.value = v;
  }, 120);
});

const feedFilterNeedle = computed(() => feedFilterQueryDebounced.value.trim().toLowerCase());

function feedMatchesFilter(feed: Feed): boolean {
  const q = feedFilterNeedle.value;
  if (!q) return true;
  return (
    feed.title.toLowerCase().includes(q) ||
    (feed.siteUrl ?? "").toLowerCase().includes(q) ||
    (feed.feedUrl ?? "").toLowerCase().includes(q)
  );
}

const EMPTY_FEEDS: Feed[] = [];

/** Pre-grouped so the template does not re-filter every feed per folder. */
const feedsByFolderId = computed(() => {
  const m = new Map<string, Feed[]>();
  const q = feedFilterNeedle.value;
  for (const f of sidebarFeeds.value) {
    if (!f.folderId) continue;
    if (q && !feedMatchesFilter(f)) continue;
    const arr = m.get(f.folderId);
    if (arr) arr.push(f);
    else m.set(f.folderId, [f]);
  }
  return m;
});

const feedsInFolder = (folderId: string) => feedsByFolderId.value.get(folderId) ?? EMPTY_FEEDS;

const ctxKind = ref<"feed" | "folder" | null>(null);
const ctxFeed = ref<Feed | null>(null);
const ctxFolder = ref<FeedFolder | null>(null);

function onLibraryContextCapture(e: MouseEvent) {
  const el = e.target as HTMLElement | null;
  if (!el || typeof el.closest !== "function") {
    e.preventDefault();
    e.stopPropagation();
    return;
  }
  if (el.closest("input, textarea, select, [contenteditable='true']")) {
    ctxKind.value = null;
    ctxFeed.value = null;
    ctxFolder.value = null;
    e.stopPropagation();
    return;
  }
  const feedEl = el.closest("[data-feed-id]") as HTMLElement | null;
  const feedId = feedEl?.dataset.feedId;
  if (feedId) {
    const feed =
      sidebarFeeds.value.find((f) => f.id === feedId) ??
      feeds.value.find((f) => f.id === feedId) ??
      null;
    if (feed) {
      ctxKind.value = "feed";
      ctxFeed.value = feed;
      ctxFolder.value = null;
      return;
    }
  }
  const folderEl = el.closest("[data-folder-id]") as HTMLElement | null;
  const folderId = folderEl?.dataset.folderId;
  if (folderId) {
    const folder = sidebarFolders.value.find((f) => f.id === folderId) ?? null;
    if (folder) {
      ctxKind.value = "folder";
      ctxFolder.value = folder;
      ctxFeed.value = null;
      return;
    }
  }
  ctxKind.value = null;
  ctxFeed.value = null;
  ctxFolder.value = null;
  e.preventDefault();
  e.stopPropagation();
}

const ctxMoveTargets = computed(() => (ctxFeed.value ? moveFolderTargets(ctxFeed.value) : []));

/** Folders visible under the current name filter (empty query → all). */
const filteredSidebarFolders = computed(() => {
  const q = feedFilterNeedle.value;
  if (!q) return sidebarFolders.value;
  return sidebarFolders.value.filter((folder) => {
    if (folder.name.toLowerCase().includes(q)) return true;
    return sidebarFeeds.value.some(
      (f) => f.folderId === folder.id && feedMatchesFilter(f),
    );
  });
});

const filteredUnfiledFeeds = computed(() => {
  const list = unfiledFeeds.value;
  if (!feedFilterNeedle.value) return list;
  return list.filter(feedMatchesFilter);
});

/** True when every folder is collapsed (or there are no folders). */
const allFoldersCollapsed = computed(() => {
  const list = sidebarFolders.value;
  if (!list.length) return true;
  return list.every((f) => !!collapsedFolders.value[f.id]);
});

function collapseAllFolders() {
  const next: Record<string, boolean> = {};
  for (const f of sidebarFolders.value) next[f.id] = true;
  collapsedFolders.value = next;
}

function expandAllFolders() {
  collapsedFolders.value = {};
}

function toggleExpandAllFolders() {
  if (allFoldersCollapsed.value) expandAllFolders();
  else collapseAllFolders();
}

async function toggleFeedFilter() {
  feedFilterOpen.value = !feedFilterOpen.value;
  if (!feedFilterOpen.value) {
    feedFilterQuery.value = "";
    feedFilterQueryDebounced.value = "";
    return;
  }
  await nextTick();
  feedFilterInputEl.value?.focus();
}

function clearFeedFilter() {
  feedFilterQuery.value = "";
  feedFilterQueryDebounced.value = "";
  feedFilterOpen.value = false;
}

onUnmounted(() => {
  if (feedFilterTimer) clearTimeout(feedFilterTimer);
  if (locateFlashTimer) clearTimeout(locateFlashTimer);
});

// While filtering, auto-expand folders that still have visible feeds.
watch(feedFilterNeedle, (q) => {
  if (!q) return;
  const next = { ...collapsedFolders.value };
  let changed = false;
  for (const folder of filteredSidebarFolders.value) {
    if (next[folder.id]) {
      delete next[folder.id];
      changed = true;
    }
  }
  if (changed) collapsedFolders.value = next;
});

/** Sum of unread for visible feeds inside each folder (respects nsfwMode). */
const folderUnreadMap = computed(() => {
  const map: Record<string, number> = {};
  for (const f of sidebarFeeds.value) {
    if (!f.folderId) continue;
    const n = f.unreadCount || 0;
    if (n <= 0) continue;
    map[f.folderId] = (map[f.folderId] ?? 0) + n;
  }
  return map;
});

function folderUnread(folderId: string): number {
  return folderUnreadMap.value[folderId] ?? 0;
}

function isActive(id: CollectionId) {
  return collectionId.value === id;
}

function goCollection(id: CollectionId) {
  selectCollection(id);
}

const locateFeedId = computed(() => {
  const fromArticle = selectedArticle.value?.feedId;
  if (fromArticle) return fromArticle;
  if (selectedFeed.value?.id) return selectedFeed.value.id;
  const col = collectionId.value;
  if (col.startsWith("feed:")) return col.slice(5);
  return "";
});

const canLocateFeed = computed(() => {
  const id = locateFeedId.value;
  if (!id) return false;
  return sidebarFeeds.value.some((f) => f.id === id) || feeds.value.some((f) => f.id === id);
});

const locateFlashId = ref("");
let locateFlashTimer: ReturnType<typeof setTimeout> | null = null;

async function locateCurrentFeed() {
  const feedId = locateFeedId.value;
  if (!feedId) return;
  const feed =
    sidebarFeeds.value.find((f) => f.id === feedId) ?? feeds.value.find((f) => f.id === feedId);
  if (!feed) {
    toast.error(t("nav.locateFeedMissing"));
    return;
  }
  if (feedFilterNeedle.value && !feedMatchesFilter(feed)) {
    clearFeedFilter();
  }
  // Accordion: only the feed's folder is open; every other folder collapses.
  const next: Record<string, boolean> = {};
  for (const folder of sidebarFolders.value) {
    if (folder.id !== feed.folderId) next[folder.id] = true;
  }
  collapsedFolders.value = next;
  selectCollection(`feed:${feed.id}`, { keepArticle: true });
  await nextTick();
  const row = document.querySelector<HTMLElement>(
    `[data-feed-id="${CSS.escape(feed.id)}"]`,
  );
  if (!row) {
    toast.error(t("nav.locateFeedMissing"));
    return;
  }
  row.scrollIntoView({ block: "center", behavior: "smooth" });
  locateFlashId.value = feed.id;
  if (locateFlashTimer) clearTimeout(locateFlashTimer);
  locateFlashTimer = setTimeout(() => {
    locateFlashId.value = "";
    locateFlashTimer = null;
  }, 1600);
}

function toggleFolder(id: string) {
  const next = { ...collapsedFolders.value };
  if (next[id]) {
    delete next[id];
  } else {
    next[id] = true;
  }
  collapsedFolders.value = next;
}

function isFolderCollapsed(id: string): boolean {
  return !!collapsedFolders.value[id];
}

async function onCreateFolder() {
  if (creatingFolder.value) return;
  const name = window.prompt(t("nav.newFolderPrompt"));
  if (name == null) return;
  const trimmed = name.trim();
  if (!trimmed) return;
  creatingFolder.value = true;
  try {
    await createFolder(trimmed);
  } catch (e) {
    console.warn("[lrss] createFolder failed", e);
  } finally {
    creatingFolder.value = false;
  }
}

// —— Context menu actions ——

function onFolderOpen(folder: FeedFolder) {
  goCollection(folderCollectionId(folder.id));
}

function onFolderToggleExpand(folder: FeedFolder) {
  toggleFolder(folder.id);
}

async function onFolderMarkRead(folder: FeedFolder) {
  if (folderBusyId.value) return;
  folderBusyId.value = folder.id;
  try {
    await markFolderRead(folder.id);
    toast.success(t("folderMenu.markReadDone"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("folderMenu.markReadFailed"), { description: msg });
  } finally {
    folderBusyId.value = null;
  }
}

async function onFolderRefresh(folder: FeedFolder) {
  if (folderBusyId.value) return;
  const hasFeeds = feeds.value.some((f) => f.folderId === folder.id);
  if (!hasFeeds) {
    toast.message(t("folderMenu.refreshEmpty"));
    return;
  }
  folderBusyId.value = folder.id;
  try {
    const res = await refreshFolderFeeds(folder.id);
    toast.success(
      t("folderMenu.refreshDone", { refreshed: res.refreshed, added: res.added }),
    );
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("folderMenu.refresh"), { description: msg });
  } finally {
    folderBusyId.value = null;
  }
}

function onFolderAddFeed(folder: FeedFolder) {
  openAddFeedInFolder(folder.id);
}

function openRename(folder: FeedFolder) {
  renameKind.value = "folder";
  renameFolderId.value = folder.id;
  renameFeedId.value = null;
  renameDraft.value = folder.name;
  renameOpen.value = true;
}

function openRenameFeed(feed: Feed) {
  renameKind.value = "feed";
  renameFeedId.value = feed.id;
  renameFolderId.value = null;
  renameDraft.value = feed.title;
  renameOpen.value = true;
}

async function confirmRename() {
  if (renameSaving.value) return;
  const name = renameDraft.value.trim();
  if (!name) {
    toast.error(
      renameKind.value === "feed"
        ? t("feedMenu.renameEmpty")
        : t("folderMenu.renameEmpty"),
    );
    return;
  }
  renameSaving.value = true;
  try {
    if (renameKind.value === "feed" && renameFeedId.value) {
      await renameFeed(renameFeedId.value, name);
      toast.success(t("feedMenu.renameSaved"));
    } else if (renameFolderId.value) {
      await renameFolder(renameFolderId.value, name);
      toast.success(t("folderMenu.renameSaved"));
    }
    renameOpen.value = false;
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(
      renameKind.value === "feed"
        ? t("feedMenu.renameFailed")
        : t("folderMenu.renameFailed"),
      { description: msg },
    );
  } finally {
    renameSaving.value = false;
  }
}

function openDelete(folder: FeedFolder) {
  deleteKind.value = "folder";
  deleteTarget.value = folder;
  deleteFeedTarget.value = null;
  deleteOpen.value = true;
}

function openDeleteFeed(feed: Feed) {
  deleteKind.value = "feed";
  deleteFeedTarget.value = feed;
  deleteTarget.value = null;
  deleteOpen.value = true;
}

function onDeleteOpenChange(open: boolean) {
  if (deleteBusy.value && !open) return;
  deleteOpen.value = open;
  if (!open) {
    deleteTarget.value = null;
    deleteFeedTarget.value = null;
  }
}

async function confirmDelete(ev: Event) {
  ev.preventDefault();
  if (deleteBusy.value) return;
  deleteBusy.value = true;
  try {
    if (deleteKind.value === "feed" && deleteFeedTarget.value) {
      await deleteFeed(deleteFeedTarget.value.id);
      toast.success(t("feedMenu.deleteDone"));
    } else if (deleteTarget.value) {
      await deleteFolder(deleteTarget.value.id);
      toast.success(t("folderMenu.deleteDone"));
    }
    deleteOpen.value = false;
    deleteTarget.value = null;
    deleteFeedTarget.value = null;
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(
      deleteKind.value === "feed"
        ? t("feedMenu.deleteFailed")
        : t("folderMenu.deleteFailed"),
      { description: msg },
    );
  } finally {
    deleteBusy.value = false;
  }
}

// —— Feed context menu ——

function onFeedOpen(feed: Feed) {
  goCollection(`feed:${feed.id}`);
}

async function onFeedRefresh(feed: Feed) {
  if (feedBusyId.value) return;
  feedBusyId.value = feed.id;
  try {
    const n = await refreshOneFeed(feed.id);
    toast.success(t("feedMenu.refreshDone", { n }));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("feedMenu.refreshFailed"), { description: msg });
  } finally {
    feedBusyId.value = null;
  }
}

async function onFeedMarkRead(feed: Feed) {
  if (feedBusyId.value) return;
  feedBusyId.value = feed.id;
  try {
    await markFeedRead(feed.id);
    toast.success(t("feedMenu.markReadDone"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("feedMenu.markReadFailed"), { description: msg });
  } finally {
    feedBusyId.value = null;
  }
}

async function onFeedPauseToggle(feed: Feed) {
  try {
    await setFeedPaused(feed.id, !feed.isPaused);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("feedMenu.pauseFailed"), { description: msg });
  }
}

async function onFeedNsfwToggle(feed: Feed) {
  try {
    await setFeedNsfw(feed.id, !feed.isNsfw);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("feedMenu.nsfwFailed"), { description: msg });
  }
}

async function onFolderNsfwToggle(folder: FeedFolder) {
  try {
    await setFolderNsfw(folder.id, !folder.isNsfw);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("folderMenu.nsfwFailed"), { description: msg });
  }
}

async function onFolderDisplayMode(folder: FeedFolder, mode: FolderDisplayMode) {
  try {
    await setFolderDisplayMode(folder.id, mode);
    toast.success(
      mode === "cards" ? t("folderMenu.displayCardsOn") : t("folderMenu.displayListOn"),
    );
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("folderMenu.displayModeFailed"), { description: msg });
  }
}

/** Folders a feed can be moved into (exclude current; sorted by name). */
function moveFolderTargets(feed: Feed): FeedFolder[] {
  const cur = feed.folderId ?? null;
  return folders.value
    .filter((f) => f.id !== cur)
    .slice()
    .sort((a, b) =>
      a.name.localeCompare(b.name, undefined, { sensitivity: "base" }),
    );
}

async function onFeedMove(feed: Feed, folderId: string | null) {
  try {
    await moveFeedToFolder(feed.id, folderId);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("feedMenu.moveFailed"), { description: msg });
  }
}

const smartItems = computed(() => [
  // Read smartCounts.* inside the computed so badge numbers stay reactive.
  {
    id: "unread" as const,
    label: t("nav.unread"),
    icon: Inbox,
    count: smartCounts.unread,
  },
  {
    id: "today" as const,
    label: t("nav.today"),
    icon: CalendarDays,
    count: smartCounts.today,
  },
  {
    id: "starred" as const,
    label: t("nav.starred"),
    icon: Star,
    count: smartCounts.starred,
  },
  {
    id: "all" as const,
    label: t("nav.all"),
    icon: BookOpenText,
    count: smartCounts.all,
  },
  {
    id: "recent" as const,
    label: t("nav.recent"),
    icon: Clock,
    count: smartCounts.recent,
  },
  ...(settings.smartBriefing
    ? [
        {
          id: "briefing" as const,
          label: t("nav.briefing"),
          icon: Newspaper,
          count: briefingUnreadCount.value,
        },
      ]
    : []),
]);
</script>

<template>
  <aside
    :class="
      cn(
        'app-sidebar flex h-full min-h-0 w-full min-w-0 flex-col overflow-hidden',
        sidebarDensityClass,
      )
    "
  >
    <div class="scroll-pane flex-1 px-2.5 pb-3">
      <ContextMenu>
      <ContextMenuTrigger as-child>
      <nav
        class="space-y-5 pt-2"
        :aria-label="t('nav.library')"
        @contextmenu.capture="onLibraryContextCapture"
      >
        <section>
          <p class="section-label px-2">{{ t("nav.smartLists") }}</p>
          <ul class="mt-1.5 space-y-0.5">
            <li v-for="item in smartItems" :key="item.id">
              <button
                type="button"
                :class="cn('nav-row', isActive(item.id) && 'nav-row-active')"
                @click="goCollection(item.id)"
              >
                <component :is="item.icon" class="nav-icon" />
                <span class="min-w-0 flex-1 truncate text-left">{{ item.label }}</span>
                <span
                  v-if="item.count > 0"
                  class="tabular-nums text-[11px] text-muted-foreground"
                >
                  {{ item.count }}
                </span>
              </button>
            </li>
          </ul>
        </section>

        <section>
          <div class="flex items-center justify-between gap-1 px-2">
            <p class="section-label">{{ t("nav.folders") }}</p>
            <div class="flex shrink-0 items-center gap-0.5">
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                class="text-muted-foreground"
                :disabled="!canLocateFeed"
                :aria-label="t('nav.locateFeed')"
                :title="t('nav.locateFeed')"
                @click="locateCurrentFeed"
              >
                <LocateFixed class="size-3.5" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                class="text-muted-foreground"
                :disabled="!sidebarFolders.length"
                :aria-label="
                  allFoldersCollapsed ? t('nav.expandAllFolders') : t('nav.collapseAllFolders')
                "
                :title="
                  allFoldersCollapsed ? t('nav.expandAllFolders') : t('nav.collapseAllFolders')
                "
                @click="toggleExpandAllFolders"
              >
                <ChevronsUpDown v-if="allFoldersCollapsed" class="size-3.5" />
                <ChevronsDownUp v-else class="size-3.5" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                class="text-muted-foreground"
                :class="feedFilterOpen && 'text-primary'"
                :aria-label="t('nav.filterFeeds')"
                :title="t('nav.filterFeeds')"
                :aria-pressed="feedFilterOpen"
                @click="toggleFeedFilter"
              >
                <ListFilter class="size-3.5" />
              </Button>
              <Button
                v-if="!webMode"
                type="button"
                variant="ghost"
                size="icon-xs"
                class="text-muted-foreground"
                :disabled="creatingFolder"
                :aria-label="t('nav.newFolder')"
                :title="t('nav.newFolder')"
                @click="onCreateFolder"
              >
                <FolderPlus class="size-3.5" />
              </Button>
            </div>
          </div>
          <div v-if="feedFilterOpen" class="mt-1.5 px-2">
            <div class="relative">
              <input
                ref="feedFilterInputEl"
                v-model="feedFilterQuery"
                type="text"
                class="border-input bg-transparent dark:bg-input/30 focus-visible:border-ring focus-visible:ring-ring/50 h-7 w-full min-w-0 rounded-lg border px-2.5 py-1 pr-7 text-[12px] outline-none focus-visible:ring-3 placeholder:text-muted-foreground"
                :placeholder="t('nav.filterFeedsPlaceholder')"
                :aria-label="t('nav.filterFeeds')"
                autocomplete="off"
                @keydown.escape.prevent="clearFeedFilter"
              />
              <button
                v-if="feedFilterQuery"
                type="button"
                class="absolute top-1/2 right-1.5 -translate-y-1/2 rounded p-0.5 text-muted-foreground hover:text-foreground"
                :aria-label="t('common.close')"
                @click="feedFilterQuery = ''"
              >
                <X class="size-3" />
              </button>
            </div>
          </div>
          <ul v-if="filteredSidebarFolders.length" class="mt-1.5 space-y-0.5">
            <li v-for="folder in filteredSidebarFolders" :key="folder.id">
              <div class="flex items-center gap-0.5">
                <button
                  type="button"
                  class="nav-row flex-1"
                  :data-folder-id="folder.id"
                  :class="isActive(`folder:${folder.id}`) && 'nav-row-active'"
                  :aria-label="
                    folderUnread(folder.id) > 0
                      ? `${folder.name} (${folderUnread(folder.id)})`
                      : folder.name
                  "
                  @click="goCollection(`folder:${folder.id}`)"
                >
                  <span class="relative shrink-0">
                    <Folder class="nav-icon" />
                    <span
                      v-if="folderUnread(folder.id) > 0"
                      class="folder-unread-dot"
                      aria-hidden="true"
                    />
                  </span>
                  <span class="min-w-0 flex-1 truncate text-left">
                    {{ folder.name }}
                    <span
                      v-if="folder.isNsfw"
                      class="ml-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
                    >NSFW</span>
                  </span>
                  <span
                    v-if="folderUnread(folder.id) > 0"
                    class="tabular-nums text-[11px] font-medium text-foreground/70"
                  >
                    {{ folderUnread(folder.id) }}
                  </span>
                  <ChevronRight
                    class="nav-icon !opacity-50 transition-transform duration-200"
                    :class="!collapsedFolders[folder.id] && 'rotate-90'"
                    @click.stop="toggleFolder(folder.id)"
                  />
                </button>
              </div>

              <ul
                v-if="!collapsedFolders[folder.id]"
                class="mt-0.5 ml-3 space-y-0.5 border-l border-border pl-1.5"
              >
                <li v-for="feed in feedsInFolder(folder.id)" :key="feed.id">
                  <button
                    type="button"
                    :data-feed-id="feed.id"
                    :class="cn('nav-row', isActive(`feed:${feed.id}`) && 'nav-row-active', locateFlashId === feed.id && 'nav-row-locate')"
                    @click="goCollection(`feed:${feed.id}`)"
                  >
                    <FeedIcon :src="feed.favicon" :title="feed.title" size="sm" />
                    <span class="min-w-0 flex-1 truncate text-left">
                      {{ feed.title }}
                      <span
                        v-if="feed.isPaused"
                        class="ml-1 text-[10px] text-muted-foreground"
                      >·</span>
                    </span>
                    <span
                      v-if="feed.unreadCount > 0"
                      class="tabular-nums text-[11px] font-medium text-foreground/70"
                    >
                      {{ feed.unreadCount }}
                    </span>
                  </button>
                </li>
              </ul>
            </li>
          </ul>
          <p
            v-else
            class="mt-1.5 px-2 text-[11.5px] text-muted-foreground"
          >
            {{
              libraryLoading
                ? t("nav.loadingLibrary")
                : feedFilterNeedle
                  ? t("nav.filterFeedsEmpty")
                  : t("nav.noFolders")
            }}
          </p>
        </section>

        <section v-if="filteredUnfiledFeeds.length || (feedFilterNeedle && unfiledFeeds.length)">
          <p class="section-label px-2">{{ t("nav.feeds") }}</p>
          <ul v-if="filteredUnfiledFeeds.length" class="mt-1.5 space-y-0.5">
            <li v-for="feed in filteredUnfiledFeeds" :key="feed.id">
              <button
                type="button"
                :data-feed-id="feed.id"
                :class="cn('nav-row', isActive(`feed:${feed.id}`) && 'nav-row-active', locateFlashId === feed.id && 'nav-row-locate')"
                @click="goCollection(`feed:${feed.id}`)"
              >
                <FeedIcon :src="feed.favicon" :title="feed.title" size="sm" />
                <span class="min-w-0 flex-1 truncate text-left">{{ feed.title }}</span>
                <span
                  v-if="feed.unreadCount > 0"
                  class="tabular-nums text-[11px] font-medium text-foreground/70"
                >
                  {{ feed.unreadCount }}
                </span>
              </button>
            </li>
          </ul>
          <p
            v-else
            class="mt-1.5 px-2 text-[11.5px] text-muted-foreground"
          >
            {{ t("nav.filterFeedsEmpty") }}
          </p>
        </section>
      </nav>
      </ContextMenuTrigger>
      <ContextMenuContent class="w-52">
        <template v-if="ctxKind === 'folder' && ctxFolder">
        <ContextMenuItem @select="onFolderOpen(ctxFolder)">
          {{ t("folderMenu.open") }}
        </ContextMenuItem>
        <ContextMenuItem @select="onFolderToggleExpand(ctxFolder)">
          {{
            isFolderCollapsed(ctxFolder.id)
              ? t("folderMenu.expand")
              : t("folderMenu.collapse")
          }}
        </ContextMenuItem>
        <ContextMenuSeparator />
        <ContextMenuItem
          :disabled="folderBusyId === ctxFolder.id || folderUnread(ctxFolder.id) === 0"
          @select="onFolderMarkRead(ctxFolder)"
        >
          {{ t("folderMenu.markAllRead") }}
        </ContextMenuItem>
        <template v-if="!webMode">
          <ContextMenuItem
            :disabled="folderBusyId === ctxFolder.id"
            @select="onFolderRefresh(ctxFolder)"
          >
            {{
              folderBusyId === ctxFolder.id
                ? t("folderMenu.refreshing")
                : t("folderMenu.refresh")
            }}
          </ContextMenuItem>
          <ContextMenuItem @select="onFolderAddFeed(ctxFolder)">
            {{ t("folderMenu.addFeed") }}
          </ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuItem @select="onFolderNsfwToggle(ctxFolder)">
            {{
              ctxFolder.isNsfw ? t("folderMenu.unmarkNsfw") : t("folderMenu.markNsfw")
            }}
          </ContextMenuItem>
          <ContextMenuSub>
            <ContextMenuSubTrigger>
              {{ t("folderMenu.displayMode") }}
            </ContextMenuSubTrigger>
            <ContextMenuSubContent class="w-44">
              <ContextMenuItem @select="onFolderDisplayMode(ctxFolder, 'list')">
                {{ t("folderMenu.displayList") }}
                <span
                  v-if="(ctxFolder.displayMode || 'list') === 'list'"
                  class="ml-auto text-[11px] text-muted-foreground"
                >✓</span>
              </ContextMenuItem>
              <ContextMenuItem @select="onFolderDisplayMode(ctxFolder, 'cards')">
                {{ t("folderMenu.displayCards") }}
                <span
                  v-if="ctxFolder.displayMode === 'cards'"
                  class="ml-auto text-[11px] text-muted-foreground"
                >✓</span>
              </ContextMenuItem>
            </ContextMenuSubContent>
          </ContextMenuSub>
          <ContextMenuItem @select="openRename(ctxFolder)">
            {{ t("folderMenu.rename") }}
          </ContextMenuItem>
          <ContextMenuItem variant="destructive" @select="openDelete(ctxFolder)">
            {{ t("folderMenu.delete") }}
          </ContextMenuItem>
        </template>
        </template>
        <template v-else-if="ctxKind === 'feed' && ctxFeed">
        <ContextMenuItem @select="onFeedOpen(ctxFeed)">
          {{ t("feedMenu.open") }}
        </ContextMenuItem>
        <ContextMenuItem
          v-if="!webMode"
          :disabled="feedBusyId === ctxFeed.id"
          @select="onFeedRefresh(ctxFeed)"
        >
          {{ t("feedMenu.refresh") }}
        </ContextMenuItem>
        <ContextMenuItem
          :disabled="feedBusyId === ctxFeed.id || ctxFeed.unreadCount === 0"
          @select="onFeedMarkRead(ctxFeed)"
        >
          {{ t("feedMenu.markAllRead") }}
        </ContextMenuItem>
        <template v-if="!webMode">
          <ContextMenuSeparator />
          <ContextMenuItem @select="openFeedEdit(ctxFeed.id)">
            {{ t("feedMenu.edit") }}
          </ContextMenuItem>
          <ContextMenuItem @select="openRenameFeed(ctxFeed)">
            {{ t("feedMenu.rename") }}
          </ContextMenuItem>
          <ContextMenuItem @select="onFeedPauseToggle(ctxFeed)">
            {{ ctxFeed.isPaused ? t("feedMenu.unpause") : t("feedMenu.pause") }}
          </ContextMenuItem>
          <ContextMenuItem @select="onFeedNsfwToggle(ctxFeed)">
            {{ ctxFeed.isNsfw ? t("feedMenu.unmarkNsfw") : t("feedMenu.markNsfw") }}
          </ContextMenuItem>
          <ContextMenuSub v-if="folders.length > 0 || ctxFeed.folderId">
            <ContextMenuSubTrigger>
              {{ t("feedMenu.moveTo") }}
            </ContextMenuSubTrigger>
            <ContextMenuSubContent>
              <ContextMenuItem
                v-if="ctxFeed.folderId"
                @select="onFeedMove(ctxFeed, null)"
              >
                {{ t("feedMenu.unfiled") }}
              </ContextMenuItem>
              <ContextMenuItem
                v-for="f in ctxMoveTargets"
                :key="f.id"
                @select="onFeedMove(ctxFeed!, f.id)"
              >
                <span class="min-w-0 flex-1 truncate">{{ f.name }}</span>
              </ContextMenuItem>
            </ContextMenuSubContent>
          </ContextMenuSub>
          <ContextMenuSeparator />
          <ContextMenuItem variant="destructive" @select="openDeleteFeed(ctxFeed)">
            {{ t("feedMenu.delete") }}
          </ContextMenuItem>
        </template>
        </template>
      </ContextMenuContent>
      </ContextMenu>
    </div>

    <Separator class="opacity-70" />
    <div class="space-y-0.5 p-2.5">
      <button
        type="button"
        class="nav-row w-full"
        :class="assistant.open && 'nav-row-active'"
        :aria-pressed="assistant.open"
        @click="toggleAssistant"
      >
        <Sparkles class="nav-icon" />
        <span class="flex-1 text-left">{{ t("ai.assistantTitle") }}</span>
      </button>
      <button v-if="!webMode" type="button" class="nav-row w-full" @click="openSettings">
        <Settings class="nav-icon" />
        <span class="flex-1 text-left">{{ t("nav.settings") }}</span>
      </button>
    </div>

    <!-- Rename folder / feed -->
    <Dialog :open="renameOpen" @update:open="(v) => (renameOpen = v)">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>
            {{
              renameKind === "feed"
                ? t("feedMenu.renameTitle")
                : t("folderMenu.renameTitle")
            }}
          </DialogTitle>
          <DialogDescription>
            {{
              renameKind === "feed"
                ? t("feedMenu.renameDesc")
                : t("folderMenu.renameDesc")
            }}
          </DialogDescription>
        </DialogHeader>
        <div class="space-y-2 py-1">
          <label class="text-[12.5px] font-medium" for="folder-rename-input">
            {{
              renameKind === "feed"
                ? t("feedMenu.renameLabel")
                : t("folderMenu.renameLabel")
            }}
          </label>
          <Input
            id="folder-rename-input"
            v-model="renameDraft"
            :placeholder="
              renameKind === 'feed'
                ? t('feedMenu.renamePlaceholder')
                : t('folderMenu.renamePlaceholder')
            "
            class="h-9"
            :disabled="renameSaving"
            @keydown.enter.prevent="confirmRename"
          />
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            size="sm"
            :disabled="renameSaving"
            @click="renameOpen = false"
          >
            {{ t("common.cancel") }}
          </Button>
          <Button
            type="button"
            size="sm"
            :disabled="renameSaving || !renameDraft.trim()"
            @click="confirmRename"
          >
            {{ renameSaving ? t("common.saving") : t("common.save") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <!-- Delete folder / feed -->
    <AlertDialog :open="deleteOpen" @update:open="onDeleteOpenChange">
      <AlertDialogContent class="sm:max-w-sm">
        <AlertDialogHeader>
          <AlertDialogMedia class="bg-destructive/10 text-destructive">
            <TriangleAlert />
          </AlertDialogMedia>
          <AlertDialogTitle>
            {{
              deleteKind === "feed"
                ? t("feedMenu.deleteConfirmTitle")
                : t("folderMenu.deleteConfirmTitle")
            }}
          </AlertDialogTitle>
          <AlertDialogDescription class="text-[13px] leading-relaxed">
            {{
              deleteKind === "feed"
                ? t("feedMenu.deleteConfirmBody", {
                    name: deleteFeedTarget?.title ?? "",
                  })
                : t("folderMenu.deleteConfirmBody", {
                    name: deleteTarget?.name ?? "",
                  })
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel size="sm" :disabled="deleteBusy">
            {{ t("common.cancel") }}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            size="sm"
            :disabled="deleteBusy"
            @click="confirmDelete"
          >
            {{
              deleteBusy
                ? t("common.loading")
                : deleteKind === "feed"
                  ? t("feedMenu.deleteConfirmAction")
                  : t("folderMenu.deleteConfirmAction")
            }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </aside>
</template>
