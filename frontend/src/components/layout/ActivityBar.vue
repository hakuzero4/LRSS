<script setup lang="ts">
import { Briefcase, Clock, Eye, LoaderCircle, Newspaper, RefreshCw } from "@lucide/vue";
import { computed, onMounted, onUnmounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute, formatAuthor } from "@/lib/format";
import { filterFeedsForSidebar } from "@/lib/nsfw";
import { pickNextRefresh } from "@/lib/nextRefresh";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const { t } = useI18n();
const {
  jobActivity,
  selectedArticle,
  selectedFeed,
  selectedBriefing,
  feeds,
  folders,
  settings,
  webMode,
  setNsfwMode,
} = useRssStore();

const nsfwToggling = ref(false);
const officeMode = computed(() => !settings.nsfwMode);

async function applyNsfwMode(showNsfw: boolean) {
  if (nsfwToggling.value || settings.nsfwMode === showNsfw) return;
  nsfwToggling.value = true;
  try {
    await setNsfwMode(showNsfw);
    if (showNsfw) {
      toast.success(t("nav.officeModeOffTitle"), {
        description: t("nav.officeModeOffDesc"),
        duration: 2800,
      });
    } else {
      toast.success(t("nav.officeModeOnTitle"), {
        description: t("nav.officeModeOnDesc"),
        duration: 2800,
      });
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("nav.officeModeFailed"), { description: msg });
  } finally {
    nsfwToggling.value = false;
  }
}

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
  >
    <div
      class="flex min-w-0 flex-1 items-center gap-2.5 overflow-hidden"
      role="status"
      aria-live="polite"
    >
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

    <div class="flex min-w-0 shrink-0 items-center justify-end gap-2">
      <p
        v-if="currentText"
        class="min-w-0 max-w-[42vw] truncate text-right"
        :class="hasJobs || !webMode ? '' : 'ml-auto'"
        :title="currentText"
      >
        {{ currentText }}
      </p>
      <template v-if="!webMode">
        <span
          v-if="currentText"
          class="h-3 w-px shrink-0 bg-border"
          aria-hidden="true"
        />
        <TooltipProvider :delay-duration="400">
          <div
            class="flex shrink-0 items-center gap-px rounded-md border border-border/80 bg-background/50 p-px"
            role="group"
            :aria-label="t('nav.officeModeTooltip')"
          >
            <Tooltip>
              <TooltipTrigger as-child>
                <button
                  type="button"
                  class="inline-flex h-5 items-center gap-1 rounded-[5px] px-1.5 text-[11px] font-medium transition-colors active:scale-[0.98] disabled:opacity-50"
                  :class="
                    officeMode
                      ? 'text-muted-foreground hover:text-foreground'
                      : 'bg-background text-foreground shadow-sm'
                  "
                  :aria-pressed="!officeMode"
                  :aria-label="t('nav.officeModeOffAria')"
                  :disabled="nsfwToggling"
                  @click="applyNsfwMode(true)"
                >
                  <Eye class="size-3 shrink-0" />
                  <span>{{ t("nav.nsfwVisible") }}</span>
                </button>
              </TooltipTrigger>
              <TooltipContent side="top" class="max-w-[240px] text-[12px]">
                {{ t("nav.officeModeTooltip") }}
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger as-child>
                <button
                  type="button"
                  class="inline-flex h-5 items-center gap-1 rounded-[5px] px-1.5 text-[11px] font-medium transition-colors active:scale-[0.98] disabled:opacity-50"
                  :class="
                    officeMode
                      ? 'bg-background text-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  "
                  :aria-pressed="officeMode"
                  :aria-label="t('nav.officeModeOnAria')"
                  :disabled="nsfwToggling"
                  @click="applyNsfwMode(false)"
                >
                  <Briefcase class="size-3 shrink-0" />
                  <span>{{ t("nav.officeMode") }}</span>
                </button>
              </TooltipTrigger>
              <TooltipContent side="top" class="max-w-[240px] text-[12px]">
                {{ t("nav.officeModeTooltip") }}
              </TooltipContent>
            </Tooltip>
          </div>
        </TooltipProvider>
      </template>
    </div>
  </div>
</template>
