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
import { useRssStore } from "@/composables/useRssStore";
import { cn } from "@/lib/utils";
import type { CollectionId } from "@/types/rss";
import FeedIcon from "@/components/feed/FeedIcon.vue";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";

const { t } = useI18n();

const {
  folders,
  feeds,
  smartCounts,
  collectionId,
  selectCollection,
  openAddFeed,
  openSettings,
  createFolder,
} = useRssStore();

const collapsedFolders = ref<Record<string, boolean>>({});
const creatingFolder = ref(false);

const unfiledFeeds = computed(() => feeds.value.filter((f) => !f.folderId));

function isActive(id: CollectionId) {
  return collectionId.value === id;
}

function goCollection(id: CollectionId) {
  selectCollection(id);
}

function toggleFolder(id: string) {
  collapsedFolders.value[id] = !collapsedFolders.value[id];
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

const smartItems = computed(() => [
  { id: "unread" as const, label: t("nav.unread"), icon: Inbox, count: () => smartCounts.value.unread },
  { id: "today" as const, label: t("nav.today"), icon: CalendarDays, count: () => smartCounts.value.today },
  { id: "starred" as const, label: t("nav.starred"), icon: Star, count: () => smartCounts.value.starred },
  { id: "all" as const, label: t("nav.all"), icon: BookOpenText, count: () => smartCounts.value.all },
]);
</script>

<template>
  <aside class="app-sidebar flex h-full min-h-0 w-[248px] shrink-0 flex-col overflow-hidden">
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
                  v-if="item.count() > 0"
                  class="tabular-nums text-[11px] text-muted-foreground"
                >
                  {{ item.count() }}
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
              <div class="flex items-center gap-0.5">
                <button
                  type="button"
                  class="nav-row flex-1"
                  :class="isActive(`folder:${folder.id}`) && 'nav-row-active'"
                  @click="goCollection(`folder:${folder.id}`)"
                >
                  <Folder class="nav-icon" />
                  <span class="min-w-0 flex-1 truncate text-left">{{ folder.name }}</span>
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
                <li
                  v-for="feed in feeds.filter((f) => f.folderId === folder.id)"
                  :key="feed.id"
                >
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
  </aside>
</template>
