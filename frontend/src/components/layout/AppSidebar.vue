<script setup lang="ts">
import {
  BookOpenText,
  CalendarDays,
  ChevronRight,
  Folder,
  Inbox,
  Plus,
  Rss,
  Settings,
  Sparkles,
  Star,
} from "@lucide/vue";
import { computed, ref } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import { cn } from "@/lib/utils";
import type { CollectionId } from "@/types/rss";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";

const {
  folders,
  feeds,
  smartCounts,
  collectionId,
  selectCollection,
  openAddFeed,
  openSettings,
} = useRssStore();

const collapsedFolders = ref<Record<string, boolean>>({});

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

const smartItems = [
  { id: "unread" as const, label: "未读", icon: Inbox, count: () => smartCounts.value.unread },
  { id: "today" as const, label: "今日", icon: CalendarDays, count: () => smartCounts.value.today },
  { id: "starred" as const, label: "收藏", icon: Star, count: () => smartCounts.value.starred },
  { id: "all" as const, label: "全部文章", icon: BookOpenText, count: () => smartCounts.value.all },
];
</script>

<template>
  <aside class="app-sidebar flex h-full w-[248px] shrink-0 flex-col">
    <div class="flex h-13 items-center justify-between gap-2 px-3 pt-2 pb-1">
      <div class="flex min-w-0 items-center gap-2.5 pl-0.5">
        <span class="brand-mark" aria-hidden="true">
          <Sparkles />
        </span>
        <div class="min-w-0">
          <p class="truncate text-[13.5px] font-semibold tracking-tight text-foreground">
            LRSS
          </p>
          <p class="truncate text-[11px] text-muted-foreground">订阅库</p>
        </div>
      </div>
      <Button
        variant="ghost"
        size="icon-sm"
        class="text-muted-foreground"
        aria-label="添加订阅"
        @click="openAddFeed"
      >
        <Plus class="size-4" />
      </Button>
    </div>

    <ScrollArea class="flex-1 px-2.5 pb-3">
      <nav class="space-y-5 pt-2" aria-label="订阅库">
        <section>
          <p class="section-label px-2">智能列表</p>
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

        <section v-if="folders.length">
          <p class="section-label px-2">文件夹</p>
          <ul class="mt-1.5 space-y-0.5">
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
                    <Rss class="nav-icon" />
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
        </section>

        <section v-if="unfiledFeeds.length">
          <p class="section-label px-2">订阅源</p>
          <ul class="mt-1.5 space-y-0.5">
            <li v-for="feed in unfiledFeeds" :key="feed.id">
              <button
                type="button"
                :class="cn('nav-row', isActive(`feed:${feed.id}`) && 'nav-row-active')"
                @click="goCollection(`feed:${feed.id}`)"
              >
                <Rss class="nav-icon" />
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
    </ScrollArea>

    <Separator class="opacity-70" />
    <div class="p-2.5">
      <button type="button" class="nav-row w-full" @click="openSettings">
        <Settings class="nav-icon" />
        <span class="flex-1 text-left">设置</span>
      </button>
    </div>
  </aside>
</template>
