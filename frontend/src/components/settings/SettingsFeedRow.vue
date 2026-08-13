<script setup lang="ts">
import { useI18n } from "vue-i18n";
import FeedIcon from "@/components/feed/FeedIcon.vue";
import { buttonVariants } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { Feed } from "@/types/rss";
import { Pencil, Trash2 } from "@lucide/vue";

defineProps<{
  feed: Feed;
  folderLabel: string;
  updatedLabel: string;
  busy?: boolean;
}>();

const emit = defineEmits<{
  edit: [];
  remove: [];
}>();

const { t } = useI18n();
</script>

<template>
  <div class="flex h-full items-center gap-2.5 px-3">
    <FeedIcon :src="feed.favicon" :title="feed.title" size="md" class="shrink-0" />
    <div class="min-w-0 flex-1">
      <div class="flex min-w-0 items-center gap-1.5">
        <p class="truncate text-[13px] font-medium leading-snug">
          {{ feed.title }}
        </p>
        <span
          v-if="feed.isPaused"
          class="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
        >
          {{ t("settings.feeds.paused") }}
        </span>
        <span
          v-if="feed.isNsfw"
          class="shrink-0 rounded bg-destructive/15 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-destructive"
        >
          {{ t("settings.feeds.nsfwBadge") }}
        </span>
      </div>
      <p class="mt-0.5 truncate text-[11px] text-muted-foreground" :title="feed.feedUrl">
        {{ folderLabel }}
        ·
        <span class="tabular-nums">{{ updatedLabel }}</span>
      </p>
      <p
        v-if="feed.lastError"
        class="mt-0.5 line-clamp-1 text-[11px] text-destructive/90"
        :title="feed.lastError"
      >
        {{ feed.lastError }}
      </p>
    </div>
    <div class="flex shrink-0 items-center gap-1.5">
      <button
        type="button"
        :class="
          cn(
            buttonVariants({ variant: 'outline', size: 'sm' }),
            'h-8 gap-1 px-2.5 text-[12px]',
          )
        "
        :disabled="busy"
        @click="emit('edit')"
      >
        <Pencil class="size-3.5 opacity-70" />
        {{ t("settings.feeds.edit") }}
      </button>
      <button
        type="button"
        :class="
          cn(
            buttonVariants({ variant: 'outline', size: 'sm' }),
            'h-8 gap-1 px-2.5 text-[12px] text-destructive hover:bg-destructive/10 hover:text-destructive',
          )
        "
        :disabled="busy"
        :aria-label="t('settings.feeds.deleteFeed')"
        @click="emit('remove')"
      >
        <Trash2 class="size-3.5 opacity-80" />
        {{ t("settings.feeds.deleteFeed") }}
      </button>
    </div>
  </div>
</template>
