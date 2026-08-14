<script setup lang="ts">
import { CheckCheck, LayoutGrid, LayoutList, Newspaper, RefreshCw, Star } from "@lucide/vue";
import { toast } from "vue-sonner";
import { folderIdForDisplayMode } from "@/lib/folderMenu";
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute, relativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import ArticleListItem from "@/components/article/ArticleListItem.vue";
import ArticleSearch from "@/components/article/ArticleSearch.vue";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const { t } = useI18n();

const {
  feeds,
  filteredArticles,
  collectionId,
  collectionTitle,
  selectedArticleId,
  selectedBriefingId,
  starredBriefings,
  searchQuery,
  searchBusy,
  searchSource,
  emptyListReason,
  articlesLoading,
  articlesLoadingMore,
  articlesHasMore,
  loadMoreArticles,
  smartCounts,
  refreshing,
  webMode,
  collectionDisplayMode,
  selectArticle,
  selectBriefing,
  markAllRead,
  refreshFeeds,
  openAddFeed,
  setFolderDisplayMode,
} = useRssStore();

const feedById = computed(() => new Map(feeds.value.map((f) => [f.id, f])));

const visibleStarredBriefings = computed(() => {
  if (collectionId.value !== "starred") return [];
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return starredBriefings.value;
  return starredBriefings.value.filter((b) => {
    const overview = (b.overview || "").toLowerCase();
    const when = formatAbsolute(b.createdAt).toLowerCase();
    return overview.includes(q) || when.includes(q);
  });
});

const empty = computed(
  () => filteredArticles.value.length === 0 && visibleStarredBriefings.value.length === 0,
);

const displayFolderId = computed(() =>
  folderIdForDisplayMode(collectionId.value, feeds.value),
);
const showDisplayToggle = computed(() => !webMode.value && !!displayFolderId.value);
const cardsOn = computed(() => collectionDisplayMode.value === "cards");
const displayToggleLabel = computed(() =>
  cardsOn.value ? t("article.displayToggleToList") : t("article.displayToggleToCards"),
);

async function toggleDisplayMode() {
  const id = displayFolderId.value;
  if (!id) return;
  const next = cardsOn.value ? "list" : "cards";
  try {
    await setFolderDisplayMode(id, next);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("folderMenu.displayModeFailed"), { description: msg });
  }
}

const listEl = ref<HTMLElement | null>(null);

watch(selectedArticleId, async (id) => {
  if (!id || !listEl.value) return;
  await nextTick();
  const row = listEl.value.querySelector(`[data-article-id="${CSS.escape(id)}"]`);
  row?.scrollIntoView({ block: "nearest" });
});

watch(selectedBriefingId, async (id) => {
  if (!id || collectionId.value !== "starred" || !listEl.value) return;
  await nextTick();
  const row = listEl.value.querySelector(`[data-briefing-id="${CSS.escape(id)}"]`);
  row?.scrollIntoView({ block: "nearest" });
});

watch(collectionId, () => {
  if (listEl.value) listEl.value.scrollTop = 0;
});

/** Tooltip / aria for the list header refresh button (scoped by current collection). */
const refreshLabel = computed(() => {
  const col = collectionId.value;
  if (col.startsWith("feed:")) return t("article.refreshThisFeed");
  if (col.startsWith("folder:")) return t("article.refreshThisFolder");
  return t("article.refreshFeeds");
});

const collectionTotal = computed(() => {
  const id = collectionId.value;
  if (id === "unread") return smartCounts.unread;
  if (id === "today") return smartCounts.today;
  if (id === "starred") return smartCounts.starred;
  if (id === "recent") return smartCounts.recent;
  if (id === "all") return smartCounts.all;
  return filteredArticles.value.length;
});

const articleCountLabel = computed(() => {
  if (articlesLoading.value && empty.value) {
    return t("common.loading");
  }
  const n = Math.max(collectionTotal.value, filteredArticles.value.length);
  const b = visibleStarredBriefings.value.length;
  const articlePart = n === 1 ? t("article.countOne") : t("article.count", { n });
  if (collectionId.value !== "starred" || b === 0) return articlePart;
  const briefingPart = b === 1 ? t("briefing.countOne") : t("briefing.count", { n: b });
  if (filteredArticles.value.length === 0) return briefingPart;
  return `${articlePart} · ${briefingPart}`;
});

function onListScroll(e: Event) {
  const el = e.target as HTMLElement | null;
  if (!el || !articlesHasMore.value || articlesLoadingMore.value || articlesLoading.value) {
    return;
  }
  if (el.scrollHeight - el.scrollTop - el.clientHeight < 240) {
    void loadMoreArticles();
  }
}

const emptyTitle = computed(() => {
  const r = emptyListReason.value;
  if (r === "loading") return t("empty.loadingTitle");
  if (r === "no-feeds") return t("empty.noFeedsTitle");
  if (r === "no-matches") return t("empty.noMatchesTitle");
  return t("empty.emptyCollectionTitle");
});

const emptyHint = computed(() => {
  const r = emptyListReason.value;
  if (r === "loading") return t("empty.loadingHint");
  if (r === "no-feeds") return t("empty.noFeedsHint");
  if (r === "no-matches") return t("empty.noMatchesHint");
  return t("empty.emptyCollectionHint");
});
</script>

<template>
  <section class="flex h-full min-h-0 w-full min-w-0 flex-col overflow-hidden bg-background">
    <header class="pane-chrome flex h-12 items-center gap-2 px-3">
      <div class="min-w-0 flex-1">
        <h2 class="truncate text-[13px] font-semibold tracking-tight">
          {{ collectionTitle }}
        </h2>
        <p class="text-[11px] text-muted-foreground tabular-nums">
          {{ articleCountLabel }}
          <span v-if="searchBusy" class="ml-1">· {{ t("shell.searching") }}</span>
          <span
            v-else-if="searchQuery.trim() && searchSource === 'backend'"
            class="ml-1"
          >
            · {{ t("shell.searchBackend") }}
          </span>
          <span
            v-else-if="searchQuery.trim() && searchSource === 'local'"
            class="ml-1"
          >
            · {{ t("shell.searchLocal") }}
          </span>
        </p>
      </div>

      <TooltipProvider :delay-duration="300">
        <Tooltip v-if="showDisplayToggle">
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon-sm"
              :class="cardsOn ? 'text-primary' : 'text-muted-foreground'"
              :aria-label="displayToggleLabel"
              :aria-pressed="cardsOn"
              @click="toggleDisplayMode"
            >
              <LayoutGrid v-if="!cardsOn" class="size-4" />
              <LayoutList v-else class="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ displayToggleLabel }}</TooltipContent>
        </Tooltip>

        <Tooltip v-if="!webMode">
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground"
              :disabled="refreshing"
              :aria-label="refreshLabel"
              @click="refreshFeeds"
            >
              <RefreshCw class="size-4" :class="refreshing && 'animate-spin'" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ refreshLabel }}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground"
              :disabled="empty"
              :aria-label="t('article.markAllRead')"
              @click="markAllRead"
            >
              <CheckCheck class="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ t("article.markAllRead") }}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </header>

    <!-- Independent search module (not part of title chrome) -->
    <ArticleSearch />

    <div
      ref="listEl"
      class="scroll-pane flex-1"
      :class="collectionDisplayMode === 'cards' && !empty ? 'article-card-scroll' : ''"
      @scroll.passive="onListScroll"
    >
      <div v-if="empty" class="flex h-48 flex-col items-center justify-center px-6 text-center">
        <p class="text-[13px] font-medium text-foreground/80">{{ emptyTitle }}</p>
        <p class="mt-1 text-[12px] leading-relaxed text-muted-foreground">
          {{ emptyHint }}
        </p>
        <Button
          v-if="emptyListReason === 'no-feeds' && !webMode"
          type="button"
          size="sm"
          class="mt-3"
          @click="openAddFeed"
        >
          {{ t("empty.noFeedsAction") }}
        </Button>
      </div>

      <template v-else-if="visibleStarredBriefings.length">
        <p class="px-3 pb-1 pt-2 text-[11px] font-medium text-muted-foreground">
          {{ t("briefing.inStarred") }}
        </p>
        <button
          v-for="item in visibleStarredBriefings"
          :key="'briefing-' + item.id"
          type="button"
          :data-briefing-id="item.id"
          :class="
            cn(
              'article-row group w-full border-b border-border/50 px-3 py-3 text-left transition-colors duration-150',
              'hover:bg-black/[0.03] dark:hover:bg-white/[0.04]',
              item.id === selectedBriefingId && 'bg-primary/10 dark:bg-primary/15',
              !item.isRead && 'is-unread',
            )
          "
          @click="selectBriefing(item.id)"
        >
          <div class="flex items-start gap-2.5">
            <Newspaper class="mt-0.5 size-3.5 shrink-0 text-muted-foreground" />
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <p class="min-w-0 flex-1 truncate text-[11px] text-muted-foreground">
                  {{ t("nav.briefing") }}
                  <span class="mx-1">·</span>
                  {{ relativeTime(item.createdAt) }}
                </p>
                <Star class="size-3.5 shrink-0 fill-amber-500 text-amber-500" />
              </div>
              <h3
                class="mt-1 line-clamp-2 text-[13.5px] leading-snug tracking-[-0.01em]"
                :class="
                  item.isRead ? 'font-medium text-foreground/80' : 'font-semibold text-foreground'
                "
              >
                {{ formatAbsolute(item.createdAt) }}
              </h3>
              <p v-if="item.overview" class="article-teaser">
                {{ item.overview }}
              </p>
            </div>
          </div>
        </button>
      </template>

      <div
        v-if="!empty && collectionDisplayMode === 'cards' && filteredArticles.length"
        class="article-card-grid"
      >
        <ArticleListItem
          v-for="article in filteredArticles"
          :key="article.id"
          :data-article-id="article.id"
          layout="card"
          :article="article"
          :feed="feedById.get(article.feedId)"
          :active="article.id === selectedArticleId"
          @select="selectArticle(article.id)"
        />
      </div>
      <template v-else-if="filteredArticles.length">
        <ArticleListItem
          v-for="article in filteredArticles"
          :key="article.id"
          :data-article-id="article.id"
          :article="article"
          :feed="feedById.get(article.feedId)"
          :active="article.id === selectedArticleId"
          @select="selectArticle(article.id)"
        />
      </template>
      <p
        v-if="!empty && articlesLoadingMore"
        class="px-3 py-2 text-center text-[11px] text-muted-foreground"
      >
        {{ t("common.loading") }}
      </p>
    </div>
  </section>
</template>
