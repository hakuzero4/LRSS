<script setup lang="ts">
import {
  BookOpenText,
  CalendarDays,
  ChevronRight,
  Folder,
  FolderPlus,
  Inbox,
  Plus,
  Settings,
  Sparkles,
  Star,
} from "@lucide/vue";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { folderCollectionId } from "@/lib/folderMenu";
import { compactSidebarClass } from "@/lib/uiGaps";
import { cn } from "@/lib/utils";
import type { CollectionId, Feed, FeedFolder } from "@/types/rss";
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
  smartCounts,
  collectionId,
  settings,
  selectCollection,
  openAddFeed,
  openAddFeedInFolder,
  openSettings,
  createFolder,
  renameFolder,
  deleteFolder,
  markFolderRead,
  refreshFolderFeeds,
  refreshOneFeed,
  markFeedRead,
  renameFeed,
  setFeedPaused,
  moveFeedToFolder,
  deleteFeed,
} = useRssStore();

const collapsedFolders = ref<Record<string, boolean>>({});
const creatingFolder = ref(false);
const folderBusyId = ref<string | null>(null);
const feedBusyId = ref<string | null>(null);

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

const unfiledFeeds = computed(() => feeds.value.filter((f) => !f.folderId));

/** Sum of unread articles for feeds inside each folder (for red-dot badge). */
const folderUnreadMap = computed(() => {
  const map: Record<string, number> = {};
  for (const f of feeds.value) {
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

function toggleFolder(id: string) {
  collapsedFolders.value[id] = !collapsedFolders.value[id];
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
    <div class="flex h-13 items-center justify-between gap-2 px-3 pt-2 pb-1">
      <div class="flex min-w-0 items-center gap-2.5 pl-0.5">
        <span class="brand-mark" aria-hidden="true">
          <Sparkles />
        </span>
        <div class="min-w-0">
          <p class="truncate text-[13.5px] font-semibold tracking-tight text-foreground">
            LRSS
          </p>
          <p class="truncate text-[11px] text-muted-foreground">{{ t("nav.library") }}</p>
        </div>
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-muted-foreground"
        :aria-label="t('nav.addFeed')"
        @click="openAddFeed"
      >
        <Plus class="size-4" />
      </Button>
    </div>

    <div class="scroll-pane flex-1 px-2.5 pb-3">
      <nav class="space-y-5 pt-2" :aria-label="t('nav.library')">
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
            <Button
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
          <ul v-if="folders.length" class="mt-1.5 space-y-0.5">
            <li v-for="folder in folders" :key="folder.id">
              <ContextMenu>
                <ContextMenuTrigger as-child>
                  <div class="flex items-center gap-0.5">
                    <button
                      type="button"
                      class="nav-row flex-1"
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
                      <span class="min-w-0 flex-1 truncate text-left">{{ folder.name }}</span>
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
                </ContextMenuTrigger>
                <ContextMenuContent class="w-52">
                  <ContextMenuItem @select="onFolderOpen(folder)">
                    {{ t("folderMenu.open") }}
                  </ContextMenuItem>
                  <ContextMenuItem @select="onFolderToggleExpand(folder)">
                    {{
                      isFolderCollapsed(folder.id)
                        ? t("folderMenu.expand")
                        : t("folderMenu.collapse")
                    }}
                  </ContextMenuItem>
                  <ContextMenuSeparator />
                  <ContextMenuItem
                    :disabled="folderBusyId === folder.id || folderUnread(folder.id) === 0"
                    @select="onFolderMarkRead(folder)"
                  >
                    {{ t("folderMenu.markAllRead") }}
                  </ContextMenuItem>
                  <ContextMenuItem
                    :disabled="folderBusyId === folder.id"
                    @select="onFolderRefresh(folder)"
                  >
                    {{
                      folderBusyId === folder.id
                        ? t("folderMenu.refreshing")
                        : t("folderMenu.refresh")
                    }}
                  </ContextMenuItem>
                  <ContextMenuItem @select="onFolderAddFeed(folder)">
                    {{ t("folderMenu.addFeed") }}
                  </ContextMenuItem>
                  <ContextMenuSeparator />
                  <ContextMenuItem @select="openRename(folder)">
                    {{ t("folderMenu.rename") }}
                  </ContextMenuItem>
                  <ContextMenuItem variant="destructive" @select="openDelete(folder)">
                    {{ t("folderMenu.delete") }}
                  </ContextMenuItem>
                </ContextMenuContent>
              </ContextMenu>

              <ul
                v-if="!collapsedFolders[folder.id]"
                class="mt-0.5 ml-3 space-y-0.5 border-l border-border pl-1.5"
              >
                <li
                  v-for="feed in feeds.filter((f) => f.folderId === folder.id)"
                  :key="feed.id"
                >
                  <ContextMenu>
                    <ContextMenuTrigger as-child>
                      <button
                        type="button"
                        :class="cn('nav-row', isActive(`feed:${feed.id}`) && 'nav-row-active')"
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
                    </ContextMenuTrigger>
                    <ContextMenuContent class="w-52">
                      <ContextMenuItem @select="onFeedOpen(feed)">
                        {{ t("feedMenu.open") }}
                      </ContextMenuItem>
                      <ContextMenuItem
                        :disabled="feedBusyId === feed.id"
                        @select="onFeedRefresh(feed)"
                      >
                        {{ t("feedMenu.refresh") }}
                      </ContextMenuItem>
                      <ContextMenuItem
                        :disabled="feedBusyId === feed.id || feed.unreadCount === 0"
                        @select="onFeedMarkRead(feed)"
                      >
                        {{ t("feedMenu.markAllRead") }}
                      </ContextMenuItem>
                      <ContextMenuSeparator />
                      <ContextMenuItem @select="openRenameFeed(feed)">
                        {{ t("feedMenu.rename") }}
                      </ContextMenuItem>
                      <ContextMenuItem @select="onFeedPauseToggle(feed)">
                        {{ feed.isPaused ? t("feedMenu.unpause") : t("feedMenu.pause") }}
                      </ContextMenuItem>
                      <ContextMenuItem @select="onFeedMove(feed, null)">
                        {{ t("feedMenu.moveTo") }} → {{ t("feedMenu.unfiled") }}
                      </ContextMenuItem>
                      <ContextMenuItem
                        v-for="f in folders.filter((x) => x.id !== folder.id)"
                        :key="f.id"
                        @select="onFeedMove(feed, f.id)"
                      >
                        {{ t("feedMenu.moveTo") }} → {{ f.name }}
                      </ContextMenuItem>
                      <ContextMenuSeparator />
                      <ContextMenuItem variant="destructive" @select="openDeleteFeed(feed)">
                        {{ t("feedMenu.delete") }}
                      </ContextMenuItem>
                    </ContextMenuContent>
                  </ContextMenu>
                </li>
              </ul>
            </li>
          </ul>
          <p
            v-else
            class="mt-1.5 px-2 text-[11.5px] text-muted-foreground"
          >
            {{ t("nav.noFolders") }}
          </p>
        </section>

        <section v-if="unfiledFeeds.length">
          <p class="section-label px-2">{{ t("nav.feeds") }}</p>
          <ul class="mt-1.5 space-y-0.5">
            <li v-for="feed in unfiledFeeds" :key="feed.id">
              <ContextMenu>
                <ContextMenuTrigger as-child>
                  <button
                    type="button"
                    :class="cn('nav-row', isActive(`feed:${feed.id}`) && 'nav-row-active')"
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
                </ContextMenuTrigger>
                <ContextMenuContent class="w-52">
                  <ContextMenuItem @select="onFeedOpen(feed)">
                    {{ t("feedMenu.open") }}
                  </ContextMenuItem>
                  <ContextMenuItem
                    :disabled="feedBusyId === feed.id"
                    @select="onFeedRefresh(feed)"
                  >
                    {{ t("feedMenu.refresh") }}
                  </ContextMenuItem>
                  <ContextMenuItem
                    :disabled="feedBusyId === feed.id || feed.unreadCount === 0"
                    @select="onFeedMarkRead(feed)"
                  >
                    {{ t("feedMenu.markAllRead") }}
                  </ContextMenuItem>
                  <ContextMenuSeparator />
                  <ContextMenuItem @select="openRenameFeed(feed)">
                    {{ t("feedMenu.rename") }}
                  </ContextMenuItem>
                  <ContextMenuItem @select="onFeedPauseToggle(feed)">
                    {{ feed.isPaused ? t("feedMenu.unpause") : t("feedMenu.pause") }}
                  </ContextMenuItem>
                  <ContextMenuItem
                    v-for="f in folders"
                    :key="f.id"
                    @select="onFeedMove(feed, f.id)"
                  >
                    {{ t("feedMenu.moveTo") }} → {{ f.name }}
                  </ContextMenuItem>
                  <ContextMenuSeparator />
                  <ContextMenuItem variant="destructive" @select="openDeleteFeed(feed)">
                    {{ t("feedMenu.delete") }}
                  </ContextMenuItem>
                </ContextMenuContent>
              </ContextMenu>
            </li>
          </ul>
        </section>
      </nav>
    </div>

    <Separator class="opacity-70" />
    <div class="p-2.5">
      <button type="button" class="nav-row w-full" @click="openSettings">
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
