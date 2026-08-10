<script setup lang="ts">
import { CheckCheck, RefreshCw, Search, X } from "@lucide/vue";
import { computed } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import ArticleListItem from "@/components/article/ArticleListItem.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

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
</script>

<template>
  <section class="flex h-full w-[340px] shrink-0 flex-col border-r border-border/70 bg-background">
    <header class="pane-chrome flex h-12 items-center gap-2 px-3">
      <div class="min-w-0 flex-1">
        <h2 class="truncate text-[13px] font-semibold tracking-tight">
          {{ collectionTitle }}
        </h2>
        <p class="text-[11px] text-muted-foreground tabular-nums">
          {{ filteredArticles.length }}
          {{ filteredArticles.length === 1 ? "article" : "articles" }}
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
              aria-label="Refresh feeds"
              @click="refreshFeeds"
            >
              <RefreshCw class="size-4" :class="refreshing && 'animate-spin'" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Refresh</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground"
              :disabled="empty"
              aria-label="Mark all as read"
              @click="markAllRead"
            >
              <CheckCheck class="size-4" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Mark all read</TooltipContent>
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
          placeholder="Search articles"
          class="h-8 bg-muted/50 pl-8 pr-8 text-[13px]"
          aria-label="Search articles"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="absolute top-1/2 right-2 -translate-y-1/2 rounded-sm p-0.5 text-muted-foreground hover:text-foreground"
          aria-label="Clear search"
          @click="searchQuery = ''"
        >
          <X class="size-3.5" />
        </button>
      </div>
    </div>

    <ScrollArea class="flex-1">
      <div v-if="empty" class="flex h-48 flex-col items-center justify-center px-6 text-center">
        <p class="text-[13px] font-medium text-foreground/80">No articles here</p>
        <p class="mt-1 text-[12px] leading-relaxed text-muted-foreground">
          Try another list, clear search, or refresh your feeds.
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
    </ScrollArea>
  </section>
</template>
