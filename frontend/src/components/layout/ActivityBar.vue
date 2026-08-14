<script setup lang="ts">
import { Clock, LoaderCircle, Newspaper, RefreshCw } from "@lucide/vue";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute, formatAuthor } from "@/lib/format";
import { filterFeedsForSidebar } from "@/lib/nsfw";
import { pickNextRefresh } from "@/lib/nextRefresh";

const { t } = useI18n();
const {
  jobActivity,
  selectedArticle,
  selectedFeed,
  selectedBriefing,
  feeds,
  folders,
  settings,
} = useRssStore();

const nowMs = ref(Date.now());
let nowTimer: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
  nowTimer = setInterval(() => {
    nowMs.value = Date.now();
  }, 15_000);
});
onUnmounted(() => {
  if (nowTimer) clearInterval(nowTimer);
});

const refreshText = computed(() => {
  const a = jobActivity;
  if (!a.refreshing) return "";
  const name = a.feedTitle.trim();
  const queued = (a.queuedTitles ?? []).filter(Boolean);
  const pending = a.pending;
  if (name && pending > 0 && queued.length) {
    return t("activity.refreshCurrentQueued", {
      name,
      queued: queued.join(t("activity.listSep")),
      n: pending,
    });
  }
  if (name && pending > 0) {
    return t("activity.refreshCurrentPending", { name, n: pending });
  }
  if (name) return t("activity.refreshCurrent", { name });
  if (pending > 0) return t("activity.refreshPending", { n: pending });
  return t("activity.refreshing");
});

const briefingText = computed(() => {
  const a = jobActivity;
  if (a.briefingState === "generating") {
    const n = a.briefingArticles || a.briefingPending;
    return n > 0 ? t("activity.briefingGeneratingN", { n }) : t("activity.briefingGenerating");
  }
  if (a.briefingState === "queued") {
    const n = a.briefingPending;
    return n > 0 ? t("activity.briefingQueuedN", { n }) : t("activity.briefingQueued");
  }
  return "";
});

const nextDue = computed(() => {
  if (!settings.autoRefresh) return null;
  const visible = filterFeedsForSidebar(feeds.value, settings.nsfwMode, folders.value);
  return pickNextRefresh(visible, settings.refreshIntervalMinutes, nowMs.value);
});

const nextDueText = computed(() => {
  if (refreshText.value) return "";
  if (!settings.autoRefresh) return t("activity.autoRefreshOff");
  const n = nextDue.value;
  if (!n) return "";
  if (n.minutes <= 0) return t("activity.nextDueSoon", { name: n.title });
  if (n.minutes === 1) return t("activity.nextDueOne", { name: n.title });
  return t("activity.nextDue", { name: n.title, n: n.minutes });
});

const hasJobs = computed(() => !!(refreshText.value || briefingText.value || nextDueText.value));

/** Right-side identity of whatever is open in the reader. */
const currentText = computed(() => {
  const article = selectedArticle.value;
  if (article) {
    const feed = selectedFeed.value?.title?.trim() || t("article.feedFallback");
    const when = article.publishedAt ? formatAbsolute(article.publishedAt) : "";
    const author = formatAuthor(article.author);
    const title = article.title.trim();
    return [feed, when, author, title].filter(Boolean).join(" · ");
  }
  const briefing = selectedBriefing.value;
  if (!briefing) return "";
  const when = briefing.createdAt ? formatAbsolute(briefing.createdAt) : "";
  const count =
    briefing.articleCount > 0
      ? briefing.articleCount === 1
        ? t("briefing.articleCountOne")
        : t("briefing.articleCount", { n: briefing.articleCount })
      : "";
  return [t("nav.briefing"), when, count].filter(Boolean).join(" · ");
});
</script>

<template>
  <div
    class="activity-bar flex h-7 shrink-0 items-center justify-between gap-3 overflow-hidden border-t border-border/70 bg-muted/35 px-3 text-[11.5px] text-muted-foreground"
    role="status"
    aria-live="polite"
  >
    <div class="flex min-w-0 flex-1 items-center gap-2.5 overflow-hidden">
      <span
        v-if="refreshText"
        class="flex min-w-0 items-center gap-1.5 truncate"
      >
        <RefreshCw class="size-3 shrink-0 animate-spin" />
        <span class="truncate">{{ refreshText }}</span>
      </span>
      <span
        v-else-if="nextDueText"
        class="flex min-w-0 items-center gap-1.5 truncate"
        :title="nextDueText"
      >
        <Clock class="size-3 shrink-0" />
        <span class="truncate">{{ nextDueText }}</span>
      </span>
      <span
        v-if="refreshText && briefingText"
        class="h-3 w-px shrink-0 bg-border"
        aria-hidden="true"
      />
      <span
        v-if="briefingText"
        class="flex min-w-0 items-center gap-1.5 truncate"
      >
        <Newspaper class="size-3 shrink-0" />
        <LoaderCircle
          v-if="jobActivity.briefingState === 'generating'"
          class="size-3 shrink-0 animate-spin"
        />
        <span class="truncate">{{ briefingText }}</span>
      </span>
    </div>

    <p
      v-if="currentText"
      class="min-w-0 max-w-[52%] shrink-0 truncate text-right"
      :class="hasJobs ? '' : 'ml-auto'"
      :title="currentText"
    >
      {{ currentText }}
    </p>
  </div>
</template>
