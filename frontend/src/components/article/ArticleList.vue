<script setup lang="ts">
import { CheckCheck, RefreshCw, Search, X } from "@lucide/vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import ArticleListItem from "@/components/article/ArticleListItem.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  collectionTitle,
  selectedArticleId,
  searchQuery,
  refreshing,
  selectArticle,
  markAllRead,
  refreshFeeds,
} = useRssStore();

const feedById = computed(() => new Map(feeds.value.map((f) => [f.id, f])));
const empty = computed(() => filteredArticles.value.length === 0);

const articleCountLabel = computed(() => {
  const n = filteredArticles.value.length;
  return n === 1 ? t("article.countOne") : t("article.count", { n });
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
        </p>
      </div>

      <TooltipProvider :delay-duration="300">
        <Tooltip>
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground"
              :disabled="refreshing"
              :aria-label="t('article.refreshFeeds')"
              @click="refreshFeeds"
            >
              <RefreshCw class="size-4" :class="refreshing && 'animate-spin'" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ t("common.refresh") }}</TooltipContent>
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

    <div class="px-3 pb-2">
      <div class="relative">
        <Search
          class="pointer-events-none absolute top-1/2 left-2.5 size-3.5 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          v-model="searchQuery"
          type="search"
          :placeholder="t('article.searchPlaceholder')"
          class="h-8 bg-muted/50 pl-8 pr-8 text-[13px]"
          :aria-label="t('article.searchAria')"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="absolute top-1/2 right-2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground hover:text-foreground"
          :aria-label="t('article.clearSearch')"
          @click="searchQuery = ''"
        >
          <X class="size-3.5" />
        </button>
      </div>
    </div>

    <div class="scroll-pane flex-1">
      <div v-if="empty" class="flex h-48 flex-col items-center justify-center px-6 text-center">
        <p class="text-[13px] font-medium text-foreground/80">{{ t("article.emptyTitle") }}</p>
        <p class="mt-1 text-[12px] leading-relaxed text-muted-foreground">
          {{ t("article.emptyHint") }}
        </p>
      </div>

      <ArticleListItem
        v-for="article in filteredArticles"
        :key="article.id"
        :article="article"
        :feed="feedById.get(article.feedId)"
        :active="article.id === selectedArticleId"
        @select="selectArticle(article.id)"
      />
    </div>
  </section>
</template>
