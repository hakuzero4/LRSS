<script setup lang="ts">
import { LoaderCircle, Newspaper, RefreshCw } from "@lucide/vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";

const { t } = useI18n();
const { jobActivity } = useRssStore();

const visible = computed(() => {
  const a = jobActivity;
  return !!(a.refreshing || a.briefingState);
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
</script>

<template>
  <div
    v-if="visible"
    class="flex h-7 shrink-0 items-center gap-2.5 overflow-hidden border-t border-border/70 bg-muted/35 px-3 text-[11.5px] text-muted-foreground"
    role="status"
    :aria-live="'polite'"
  >
    <span
      v-if="refreshText"
      class="flex min-w-0 items-center gap-1.5 truncate"
    >
      <RefreshCw class="size-3 shrink-0 animate-spin" />
      <span class="truncate">{{ refreshText }}</span>
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
</template>
