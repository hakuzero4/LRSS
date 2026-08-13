<script setup lang="ts">
import { CheckCheck, RefreshCw } from "@lucide/vue";
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
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
  searchQuery,
  searchBusy,
  searchSource,
  emptyListReason,
  articlesLoading,
  refreshing,
  webMode,
  selectArticle,
  markAllRead,
  refreshFeeds,
  openAddFeed,
} = useRssStore();

const feedById = computed(() => new Map(feeds.value.map((f) => [f.id, f])));
const empty = computed(() => filteredArticles.value.length === 0);

const listEl = ref<HTMLElement | null>(null);

watch(selectedArticleId, async (id) => {
  if (!id || !listEl.value) return;
  await nextTick();
  const row = listEl.value.querySelector(`[data-article-id="${CSS.escape(id)}"]`);
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

const articleCountLabel = computed(() => {
  if (articlesLoading.value && empty.value) {
    return t("common.loading");
  }
  const n = filteredArticles.value.length;
  return n === 1 ? t("article.countOne") : t("article.count", { n });
});

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

    <div ref="listEl" class="scroll-pane flex-1">
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

      <ArticleListItem
        v-for="article in filteredArticles"
        :key="article.id"
        :data-article-id="article.id"
        :article="article"
        :feed="feedById.get(article.feedId)"
        :active="article.id === selectedArticleId"
        @select="selectArticle(article.id)"
      />
    </div>
  </section>
</template>
