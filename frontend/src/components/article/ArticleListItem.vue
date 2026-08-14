<script setup lang="ts">
import { Star } from "@lucide/vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import type { Article, Feed } from "@/types/rss";
import FeedIcon from "@/components/feed/FeedIcon.vue";
import { articleCardImage } from "@/lib/folderMenu";
import { plainText, relativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";

const props = defineProps<{
  article: Article;
  feed?: Feed | null;
  active?: boolean;
  layout?: "list" | "card";
}>();

const emit = defineEmits<{
  select: [];
}>();

const { t } = useI18n();

const meta = computed(() => {
  const source = props.feed?.title ?? t("article.feedFallback");
  return `${source} · ${relativeTime(props.article.publishedAt)}`;
});

const teaser = computed(() => plainText(props.article.summary, 160));
const cardImage = computed(() => articleCardImage(props.article));
const isCard = computed(() => props.layout === "card");
</script>

<template>
  <button
    v-if="isCard"
    type="button"
    :class="
      cn(
        'article-card group text-left',
        active && 'article-card-active',
        !article.read && 'is-unread',
      )
    "
    @click="emit('select')"
  >
    <div class="article-card-thumb">
      <img
        v-if="cardImage"
        :src="cardImage"
        alt=""
        loading="lazy"
        decoding="async"
        class="article-card-img"
      />
      <div v-else class="article-card-fallback">
        <span class="line-clamp-4 px-2 text-center text-[12px] font-medium leading-snug text-muted-foreground">
          {{ article.title }}
        </span>
      </div>
      <span
        v-if="!article.read"
        class="article-card-unread"
        aria-hidden="true"
      />
      <Star
        v-if="article.starred"
        class="article-card-star size-3.5 fill-amber-500 text-amber-500"
        :aria-label="t('article.starred')"
      />
    </div>
    <div class="article-card-meta">
      <h3
        class="line-clamp-2 text-[12.5px] leading-snug tracking-[-0.01em]"
        :class="article.read ? 'font-medium text-foreground/75' : 'font-semibold text-foreground'"
      >
        {{ article.title }}
      </h3>
      <p class="mt-0.5 truncate text-[10.5px] text-muted-foreground">
        {{ meta }}
      </p>
    </div>
  </button>
  <button
    v-else
    type="button"
    :class="
      cn(
        'article-row group w-full border-b border-border/50 px-3 py-3 text-left transition-colors duration-150',
        'hover:bg-black/[0.03] dark:hover:bg-white/[0.04]',
        'active:scale-[0.995] active:bg-black/[0.05] dark:active:bg-white/[0.06]',
        active && 'article-row-active bg-primary/10 dark:bg-primary/15',
        !article.read && 'is-unread',
      )
    "
    @click="emit('select')"
  >
    <div class="flex items-start gap-2.5">
      <span
        class="mt-1.5 size-1.5 shrink-0 rounded-full"
        :class="article.read ? 'bg-transparent' : 'unread-dot'"
        aria-hidden="true"
      />
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2">
          <FeedIcon
            :src="feed?.favicon"
            :title="feed?.title"
            size="md"
            class="mt-0"
          />
          <p class="min-w-0 flex-1 truncate text-[11px] text-muted-foreground">
            {{ meta }}
          </p>
          <Star
            v-if="article.starred"
            class="size-3 shrink-0 fill-amber-500 text-amber-500"
            :aria-label="t('article.starred')"
          />
        </div>
        <h3
          class="mt-1 line-clamp-2 text-[13.5px] leading-snug tracking-[-0.01em]"
          :class="article.read ? 'font-medium text-foreground/80' : 'font-semibold text-foreground'"
        >
          {{ article.title }}
        </h3>
        <p v-if="teaser" class="article-teaser">
          {{ teaser }}
        </p>
      </div>
    </div>
  </button>
</template>
