<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import FeedIcon from "@/components/feed/FeedIcon.vue";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { parseFeedUrlsFromText } from "@/lib/feedUrls";
import { relativeTime } from "@/lib/format";
import type { Feed } from "@/types/rss";
import { Pencil, Plus, Search, TriangleAlert } from "@lucide/vue";

const { t } = useI18n();

const {
  settings,
  folders,
  feeds,
  importOPMLFile,
  exportOPMLDownload,
  clearAllSubscriptions,
  persistUIPrefs,
  purgeOldArticles,
  renameFeed,
  setFeedRefreshInterval,
  setFeedPaused,
  moveFeedToFolder,
  refreshOneFeed,
  deleteFeed,
  setFeedNsfw,
  addFeedsFromURLs,
} = useRssStore();

const fileInputRef = ref<HTMLInputElement | null>(null);
const importing = ref(false);
const exporting = ref(false);
const importStatus = ref("");
const importCurrent = ref(0);
const importTotal = ref(0);

const confirmClearOpen = ref(false);
const clearing = ref(false);
const clearStatus = ref("");
const purging = ref(false);

/** Interval presets (minutes). 0 = follow global default. */
const INTERVAL_OPTIONS = [0, 5, 15, 30, 60, 120, 180] as const;

const feedFilter = ref("");

const subscriptionCount = computed(() => feeds.value.length);

const sortedFeeds = computed(() => {
  const q = feedFilter.value.trim().toLowerCase();
  let list = [...feeds.value];
  if (q) {
    list = list.filter(
      (f) =>
        f.title.toLowerCase().includes(q) ||
        f.feedUrl.toLowerCase().includes(q) ||
        (f.siteUrl?.toLowerCase().includes(q) ?? false),
    );
  }
  return list.sort((a, b) =>
    a.title.localeCompare(b.title, undefined, { sensitivity: "base" }),
  );
});

const globalIntervalLabel = computed(() => formatIntervalMinutes(settings.refreshIntervalMinutes));

function formatIntervalMinutes(m: number): string {
  if (m < 60) return t("common.minutes", { n: m });
  if (m === 60) return t("common.oneHour");
  if (m % 60 === 0) return t("common.hours", { n: m / 60 });
  return t("common.minutes", { n: m });
}

function intervalOptionLabel(minutes: number): string {
  if (minutes === 0) {
    return t("settings.feeds.intervalDefault", { n: globalIntervalLabel.value });
  }
  return t("settings.feeds.intervalCustom", { n: formatIntervalMinutes(minutes) });
}

function lastUpdatedLabel(feed: Feed): string {
  const iso = feed.lastFetchedAt?.trim();
  if (!iso) return t("settings.feeds.neverUpdated");
  const tms = Date.parse(iso);
  if (Number.isNaN(tms)) return t("settings.feeds.neverUpdated");
  return relativeTime(iso);
}

function folderName(feed: Feed): string {
  if (!feed.folderId) return t("settings.feeds.unfiled");
  return folders.value.find((f) => f.id === feed.folderId)?.name ?? t("settings.feeds.unfiled");
}

const busy = computed(
  () => importing.value || exporting.value || clearing.value || purging.value || addingFeeds.value,
);

// ── Add feed(s) dialog (multi-line) ────────────────────────────
const addOpen = ref(false);
const addUrlsText = ref("");
const addFolderId = ref("none");
const addNsfw = ref(false);
const addingFeeds = ref(false);
const addProgress = ref({ current: 0, total: 0 });

const parsedAddUrls = computed(() => parseFeedUrlsFromText(addUrlsText.value));

function openAddFeeds() {
  addUrlsText.value = "";
  addFolderId.value = settings.defaultFolderId?.trim() || "none";
  addNsfw.value = false;
  addingFeeds.value = false;
  addProgress.value = { current: 0, total: 0 };
  addOpen.value = true;
}

async function confirmAddFeeds() {
  if (addingFeeds.value) return;
  const urls = parsedAddUrls.value;
  if (urls.length === 0) {
    toast.error(t("settings.feeds.addEmpty"));
    return;
  }
  addingFeeds.value = true;
  addProgress.value = { current: 0, total: urls.length };
  try {
    // Sequential with progress: call one-by-one via batch helper is all-or-reload;
    // use batch API which is sequential and reports result.
    const folder = addFolderId.value === "none" ? null : addFolderId.value;
    // Progress: reimplement loop here for UI, or enhance store — simple toast after batch is OK.
    // For live progress, loop with addFeedsFromURLs is one-shot; show indeterminate then result.
    const result = await addFeedsFromURLs(urls, {
      folderId: folder,
      isNsfw: addNsfw.value,
      selectLast: true,
      onProgress: (current, total) => {
        addProgress.value = { current, total };
      },
    });

    if (result.added > 0 && result.failed.length === 0) {
      toast.success(t("settings.feeds.addDone", { n: result.added }));
      addOpen.value = false;
    } else if (result.added > 0 && result.failed.length > 0) {
      toast.success(t("settings.feeds.addPartial", { ok: result.added, fail: result.failed.length }), {
        description: result.failed
          .slice(0, 3)
          .map((f) => `${f.url}: ${f.message}`)
          .join("\n"),
      });
      addOpen.value = false;
    } else {
      toast.error(t("settings.feeds.addAllFailed"), {
        description: result.failed[0]?.message,
      });
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.addFailed"), { description: msg });
  } finally {
    addingFeeds.value = false;
  }
}

// ── Edit subscription dialog ───────────────────────────────────
const editOpen = ref(false);
const editFeedId = ref<string | null>(null);
const editTitle = ref("");
const editInterval = ref("0");
const editFolderId = ref("none");
const editPaused = ref(false);
const editNsfw = ref(false);
const editSaving = ref(false);
const editRefreshing = ref(false);
const editFeedUrl = ref("");
const deleteFeedOpen = ref(false);
const deleteFeedBusy = ref(false);

const editFeed = computed(() =>
  editFeedId.value ? feeds.value.find((f) => f.id === editFeedId.value) ?? null : null,
);

function openEdit(feed: Feed) {
  editFeedId.value = feed.id;
  editTitle.value = feed.title;
  editInterval.value = String(feed.refreshIntervalMinutes ?? 0);
  editFolderId.value = feed.folderId ?? "none";
  editPaused.value = !!feed.isPaused;
  editNsfw.value = !!feed.isNsfw;
  editFeedUrl.value = feed.feedUrl;
  editOpen.value = true;
}

async function confirmEdit() {
  if (!editFeedId.value || editSaving.value) return;
  const title = editTitle.value.trim();
  if (!title) {
    toast.error(t("settings.feeds.renameEmpty"));
    return;
  }
  editSaving.value = true;
  const id = editFeedId.value;
  try {
    const current = feeds.value.find((f) => f.id === id);
    if (!current) throw new Error("feed missing");

    if (title !== current.title) {
      await renameFeed(id, title);
    }
    const minutes = Math.max(0, Math.floor(Number(editInterval.value) || 0));
    if (minutes !== (current.refreshIntervalMinutes ?? 0)) {
      await setFeedRefreshInterval(id, minutes);
    }
    const folder = editFolderId.value === "none" ? null : editFolderId.value;
    const curFolder = current.folderId ?? null;
    if ((folder ?? null) !== (curFolder ?? null)) {
      await moveFeedToFolder(id, folder);
    }
    if (editPaused.value !== !!current.isPaused) {
      await setFeedPaused(id, editPaused.value);
    }
    if (editNsfw.value !== !!current.isNsfw) {
      await setFeedNsfw(id, editNsfw.value);
    }
    editOpen.value = false;
    toast.success(t("settings.feeds.saved"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.saveFailed"), { description: msg });
  } finally {
    editSaving.value = false;
  }
}

async function onEditRefresh() {
  if (!editFeedId.value || editRefreshing.value) return;
  editRefreshing.value = true;
  try {
    const n = await refreshOneFeed(editFeedId.value);
    toast.success(t("settings.feeds.refreshDone", { n }));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.refreshFailed"), { description: msg });
  } finally {
    editRefreshing.value = false;
  }
}

function openDeleteFeed() {
  if (!editFeedId.value) return;
  deleteFeedOpen.value = true;
}

async function confirmDeleteFeed(ev: Event) {
  ev.preventDefault();
  if (!editFeedId.value || deleteFeedBusy.value) return;
  deleteFeedBusy.value = true;
  try {
    await deleteFeed(editFeedId.value);
    deleteFeedOpen.value = false;
    editOpen.value = false;
    toast.success(t("settings.feeds.deleteFeedDone"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.deleteFeedFailed"), { description: msg });
  } finally {
    deleteFeedBusy.value = false;
  }
}

const importPercent = computed(() => {
  if (!importTotal.value) return 0;
  return Math.min(100, Math.round((importCurrent.value / importTotal.value) * 100));
});

const keepDaysModel = computed({
  get: () => [settings.keepArticlesDays],
  set: (v: number[]) => {
    settings.keepArticlesDays = v[0] ?? 90;
    persistUIPrefs();
  },
});

const defaultFolderModel = computed({
  get: () => settings.defaultFolderId ?? "none",
  set: (v: string) => {
    settings.defaultFolderId = v === "none" ? null : v;
    persistUIPrefs();
  },
});

async function onPurgeNow() {
  if (purging.value || busy.value) return;
  purging.value = true;
  try {
    const deleted = await purgeOldArticles();
    toast.success(t("settings.feeds.purged", { n: deleted }));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.purgeFailed"), { description: msg });
  } finally {
    purging.value = false;
  }
}

const clearDescription = computed(() =>
  subscriptionCount.value > 0
    ? t("danger.clear.descriptionCount", { n: subscriptionCount.value })
    : t("danger.clear.description"),
);

function triggerImport() {
  if (importing.value) return;
  fileInputRef.value?.click();
}

async function onImportFileChange(ev: Event) {
  const input = ev.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;

  importing.value = true;
  importStatus.value = t("opml.preparing");
  importCurrent.value = 0;
  importTotal.value = 0;
  try {
    const result = await importOPMLFile(file, (p) => {
      importStatus.value = p.message;
      if (typeof p.total === "number" && p.total > 0) {
        importTotal.value = p.total;
      }
      if (typeof p.current === "number") {
        importCurrent.value = p.current;
      }
    });
    const parts = [
      t("opml.partFolders", { n: result.foldersCreated }),
      t("opml.partAdded", { n: result.feedsAdded }),
      t("opml.partSkipped", { n: result.feedsSkipped }),
      t("opml.partFailed", { n: result.feedsFailed }),
    ];
    importStatus.value = t("opml.importDone", { parts: parts.join(" · ") });
    if (result.addedFeedIds?.length) {
      importCurrent.value = result.addedFeedIds.length;
      importTotal.value = result.addedFeedIds.length;
    }
    if (result.errors?.length) {
      console.warn("[lrss] OPML import errors", result.errors);
      importStatus.value += t("opml.importErrorsSuffix", { n: result.errors.length });
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    importStatus.value = t("opml.importFailed", { msg });
    console.warn("[lrss] importOPMLFile failed", e);
  } finally {
    importing.value = false;
  }
}

async function onExport() {
  if (exporting.value || importing.value || clearing.value) return;
  exporting.value = true;
  importStatus.value = "";
  importCurrent.value = 0;
  importTotal.value = 0;
  try {
    await exportOPMLDownload();
    importStatus.value = t("opml.exported");
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    importStatus.value = t("opml.exportFailed", { msg });
    console.warn("[lrss] exportOPMLDownload failed", e);
  } finally {
    exporting.value = false;
  }
}

function openClearConfirm() {
  if (clearing.value || importing.value || exporting.value) return;
  if (subscriptionCount.value === 0 && folders.value.length === 0) {
    clearStatus.value = t("danger.clear.empty");
    return;
  }
  clearStatus.value = "";
  confirmClearOpen.value = true;
}

function onConfirmOpenChange(open: boolean) {
  if (clearing.value && !open) return;
  confirmClearOpen.value = open;
}

async function confirmClearAll(ev: Event) {
  ev.preventDefault();
  if (clearing.value) return;
  clearing.value = true;
  clearStatus.value = t("danger.clear.inProgress");
  try {
    const res = await clearAllSubscriptions();
    confirmClearOpen.value = false;
    clearStatus.value = t("danger.clear.done", {
      feeds: res.feedsDeleted,
      folders: res.foldersDeleted,
    });
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    clearStatus.value = t("danger.clear.failed", { msg });
    console.warn("[lrss] clearAllSubscriptions failed", e);
  } finally {
    clearing.value = false;
  }
}
</script>

<template>
  <div class="space-y-7">
    <!-- All subscriptions: fixed max height so settings panel does not balloon -->
    <SettingsGroup
      :title="t('settings.feeds.listGroup')"
      :description="t('settings.feeds.listGroupDesc')"
    >
      <div class="flex flex-wrap items-center justify-between gap-2 pb-2 pt-1">
        <p class="text-[11.5px] tabular-nums text-muted-foreground">
          <template v-if="subscriptionCount === 0">
            {{ t("settings.feeds.listEmpty") }}
          </template>
          <template v-else-if="feedFilter.trim()">
            {{
              t("settings.feeds.listFiltered", {
                shown: sortedFeeds.length,
                n: subscriptionCount,
              })
            }}
          </template>
          <template v-else>
            {{ t("settings.feeds.listCount", { n: subscriptionCount }) }}
          </template>
        </p>
        <div class="flex flex-wrap items-center gap-2">
          <div v-if="subscriptionCount > 0" class="relative w-full max-w-[200px] sm:w-[200px]">
            <Search
              class="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              v-model="feedFilter"
              type="search"
              class="h-8 pl-7 text-[12px]"
              :placeholder="t('settings.feeds.listSearchPlaceholder')"
            />
          </div>
          <Button
            type="button"
            size="sm"
            class="h-8 shrink-0 gap-1 px-2.5 text-[12px]"
            :disabled="busy"
            @click="openAddFeeds"
          >
            <Plus class="size-3.5" />
            {{ t("settings.feeds.addFeeds") }}
          </Button>
        </div>
      </div>

      <template v-if="subscriptionCount === 0">
        <div
          class="flex flex-col items-center gap-3 rounded-lg border border-dashed border-border/80 px-4 py-8 text-center"
        >
          <p class="text-[12.5px] text-muted-foreground">
            {{ t("settings.feeds.listEmptyHint") }}
          </p>
          <Button type="button" size="sm" class="gap-1" :disabled="busy" @click="openAddFeeds">
            <Plus class="size-3.5" />
            {{ t("settings.feeds.addFeeds") }}
          </Button>
        </div>
      </template>
      <template v-else>
        <!-- Cap height (~5–6 rows); scroll inside so OPML / retention stay reachable -->
        <div
          class="max-h-[min(280px,42vh)] overflow-y-auto overscroll-contain rounded-lg border border-border/70"
          role="region"
          :aria-label="t('settings.feeds.listGroup')"
        >
          <ul class="divide-y divide-border/70">
            <li
              v-for="feed in sortedFeeds"
              :key="feed.id"
              class="flex items-center gap-2.5 px-3 py-2.5"
            >
              <FeedIcon
                :src="feed.favicon"
                :title="feed.title"
                size="md"
                class="shrink-0"
              />
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
                <p
                  class="mt-0.5 truncate text-[11px] text-muted-foreground"
                  :title="feed.feedUrl"
                >
                  {{ folderName(feed) }}
                  ·
                  <span class="tabular-nums">{{ lastUpdatedLabel(feed) }}</span>
                </p>
                <p
                  v-if="feed.lastError"
                  class="mt-0.5 line-clamp-1 text-[11px] text-destructive/90"
                  :title="feed.lastError"
                >
                  {{ feed.lastError }}
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="h-8 shrink-0 gap-1 px-2.5 text-[12px]"
                :disabled="busy"
                @click="openEdit(feed)"
              >
                <Pencil class="size-3.5 opacity-70" />
                {{ t("settings.feeds.edit") }}
              </Button>
            </li>
            <li
              v-if="sortedFeeds.length === 0"
              class="px-3 py-6 text-center text-[12px] text-muted-foreground"
            >
              {{ t("settings.feeds.listEmpty") }}
            </li>
          </ul>
        </div>
      </template>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.feeds.opmlGroup')"
      :description="t('settings.feeds.opmlGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.feeds.opmlImportExport')"
          :description="t('settings.feeds.opmlImportExportDesc')"
        >
          <div class="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              :disabled="importing || exporting"
              @click="triggerImport"
            >
              {{ importing ? t("settings.feeds.importing") : t("settings.feeds.importOpml") }}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              :disabled="importing || exporting"
              @click="onExport"
            >
              {{ exporting ? t("settings.feeds.exporting") : t("settings.feeds.exportOpml") }}
            </Button>
          </div>
        </SettingsRow>
      </div>
      <div v-if="importStatus" class="space-y-2 px-0.5 pb-2.5">
        <p class="text-[12px] leading-relaxed text-muted-foreground" role="status">
          {{ importStatus }}
          <span
            v-if="importing && importTotal > 0"
            class="ml-1 tabular-nums text-foreground/70"
          >
            {{
              t("opml.progress", {
                current: importCurrent,
                total: importTotal,
                percent: importPercent,
              })
            }}
          </span>
        </p>
        <div
          v-if="importing && importTotal > 0"
          class="h-1.5 w-full overflow-hidden rounded-full bg-muted"
          role="progressbar"
          :aria-valuenow="importCurrent"
          :aria-valuemin="0"
          :aria-valuemax="importTotal"
        >
          <div
            class="h-full rounded-full bg-primary transition-[width] duration-200 ease-out"
            :style="{ width: `${importPercent}%` }"
          />
        </div>
      </div>
      <input
        ref="fileInputRef"
        type="file"
        class="hidden"
        accept=".opml,.xml,text/xml,application/xml"
        @change="onImportFileChange"
      />
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.feeds.subscriptionGroup')"
      :description="t('settings.feeds.subscriptionGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.feeds.defaultFolder')"
          :description="t('settings.feeds.defaultFolderDesc')"
        >
          <Select v-model="defaultFolderModel">
            <SelectTrigger class="h-8 w-[150px] text-[13px]">
              <SelectValue :placeholder="t('settings.feeds.unfiled')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">{{ t("settings.feeds.unfiled") }}</SelectItem>
              <SelectItem v-for="f in folders" :key="f.id" :value="f.id">
                {{ f.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>
      <div class="py-2.5 opacity-60">
        <SettingsRow
          :title="t('settings.feeds.fetchFullContent')"
          :description="t('settings.unavailable.fetchFullContent')"
        >
          <Switch
            :checked="false"
            disabled
            :aria-disabled="true"
            :aria-label="t('settings.unavailable.comingSoon')"
          />
        </SettingsRow>
      </div>
      <div class="space-y-3 py-3">
        <div class="flex items-end justify-between gap-3">
          <div>
            <p class="text-[13px] font-medium">{{ t("settings.feeds.keepDays") }}</p>
            <p class="mt-0.5 text-[12px] text-muted-foreground">
              {{ t("settings.feeds.keepDaysDesc") }}
            </p>
          </div>
          <span class="tabular-nums text-[12.5px] font-medium">
            {{ t("common.days", { n: settings.keepArticlesDays }) }}
          </span>
        </div>
        <Slider v-model="keepDaysModel" :min="7" :max="365" :step="1" class="w-full" />
        <div class="flex justify-end pt-0.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            :disabled="purging || importing || exporting || clearing"
            @click="onPurgeNow"
          >
            {{ purging ? t("settings.feeds.purging") : t("settings.feeds.purgeNow") }}
          </Button>
        </div>
      </div>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.feeds.dangerGroup')"
      :description="t('settings.feeds.dangerGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('danger.clear.title')"
          :description="clearDescription"
        >
          <Button
            type="button"
            variant="destructive"
            size="sm"
            :disabled="importing || exporting || clearing"
            @click="openClearConfirm"
          >
            {{ clearing ? t("danger.clear.clearing") : t("danger.clear.button") }}
          </Button>
        </SettingsRow>
      </div>
      <p
        v-if="clearStatus"
        class="px-0.5 pb-2.5 text-[12px] leading-relaxed text-muted-foreground"
        role="status"
      >
        {{ clearStatus }}
      </p>
    </SettingsGroup>

    <!-- Add one or many feed URLs -->
    <Dialog :open="addOpen" @update:open="(v) => !addingFeeds && (addOpen = v)">
      <DialogContent class="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{{ t("settings.feeds.addTitle") }}</DialogTitle>
          <DialogDescription>{{ t("settings.feeds.addDesc") }}</DialogDescription>
        </DialogHeader>
        <form class="grid gap-3.5 py-1" @submit.prevent="confirmAddFeeds">
          <div class="grid gap-1.5">
            <Label for="settings-add-urls">{{ t("settings.feeds.addUrlsLabel") }}</Label>
            <textarea
              id="settings-add-urls"
              v-model="addUrlsText"
              rows="7"
              class="border-input dark:bg-input/30 focus-visible:border-ring focus-visible:ring-ring/50 placeholder:text-muted-foreground w-full min-w-0 resize-y rounded-lg border bg-transparent px-2.5 py-2 font-mono text-[12.5px] leading-relaxed outline-none focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50"
              :placeholder="t('settings.feeds.addUrlsPlaceholder')"
              :disabled="addingFeeds"
              spellcheck="false"
              autocomplete="off"
            />
            <p class="text-[11.5px] leading-snug text-muted-foreground">
              {{
                parsedAddUrls.length > 0
                  ? t("settings.feeds.addUrlsCount", { n: parsedAddUrls.length })
                  : t("settings.feeds.addUrlsHint")
              }}
            </p>
          </div>
          <div class="grid gap-1.5">
            <Label>{{ t("settings.feeds.folderLabel") }}</Label>
            <Select v-model="addFolderId" :disabled="addingFeeds">
              <SelectTrigger class="h-9 w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" class="w-[var(--reka-select-trigger-width)]">
                <SelectItem value="none">{{ t("settings.feeds.unfiled") }}</SelectItem>
                <SelectItem v-for="f in folders" :key="f.id" :value="f.id">
                  {{ f.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div
            class="flex items-start justify-between gap-3 rounded-md border border-border/60 px-3 py-2.5"
          >
            <div class="min-w-0">
              <p class="text-[13px] font-medium">{{ t("settings.feeds.nsfw") }}</p>
              <p class="mt-0.5 text-[11.5px] text-muted-foreground">
                {{ t("settings.feeds.addNsfwDesc") }}
              </p>
            </div>
            <Switch v-model:checked="addNsfw" :disabled="addingFeeds" class="mt-0.5" />
          </div>
          <p
            v-if="addingFeeds && addProgress.total > 0"
            class="text-[12px] tabular-nums text-muted-foreground"
            role="status"
          >
            {{
              t("settings.feeds.addProgress", {
                current: addProgress.current,
                total: addProgress.total,
              })
            }}
          </p>
          <DialogFooter class="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="ghost"
              :disabled="addingFeeds"
              @click="addOpen = false"
            >
              {{ t("common.cancel") }}
            </Button>
            <Button type="submit" :disabled="addingFeeds || parsedAddUrls.length === 0">
              {{
                addingFeeds
                  ? t("settings.feeds.adding")
                  : parsedAddUrls.length > 1
                    ? t("settings.feeds.addSubmitMany", { n: parsedAddUrls.length })
                    : t("settings.feeds.addSubmit")
              }}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>

    <!-- Edit subscription (name / interval / folder / pause / refresh / delete) -->
    <Dialog :open="editOpen" @update:open="(v) => (editOpen = v)">
      <DialogContent class="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{{ t("settings.feeds.editTitle") }}</DialogTitle>
          <DialogDescription>{{ t("settings.feeds.editDesc") }}</DialogDescription>
        </DialogHeader>
        <div class="grid gap-3.5 py-1">
          <div class="grid gap-1.5">
            <Label for="feed-edit-title">{{ t("settings.feeds.renameLabel") }}</Label>
            <Input
              id="feed-edit-title"
              v-model="editTitle"
              :placeholder="t('settings.feeds.renamePlaceholder')"
              class="h-9"
              :disabled="editSaving"
            />
          </div>
          <div class="grid gap-1.5">
            <Label>{{ t("settings.feeds.feedUrlLabel") }}</Label>
            <p
              class="truncate rounded-md border border-border/60 bg-muted/40 px-2.5 py-2 text-[12px] text-muted-foreground"
              :title="editFeedUrl"
            >
              {{ editFeedUrl }}
            </p>
          </div>
          <div class="grid gap-1.5">
            <Label>{{ t("settings.feeds.refreshInterval") }}</Label>
            <Select v-model="editInterval" :disabled="editSaving || editPaused">
              <SelectTrigger class="h-9 w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" class="w-[var(--reka-select-trigger-width)]">
                <SelectItem
                  v-for="opt in INTERVAL_OPTIONS"
                  :key="opt"
                  :value="String(opt)"
                >
                  {{ intervalOptionLabel(opt) }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="grid gap-1.5">
            <Label>{{ t("settings.feeds.folderLabel") }}</Label>
            <Select v-model="editFolderId" :disabled="editSaving">
              <SelectTrigger class="h-9 w-full text-[13px]">
                <SelectValue />
              </SelectTrigger>
              <SelectContent position="popper" class="w-[var(--reka-select-trigger-width)]">
                <SelectItem value="none">{{ t("settings.feeds.unfiled") }}</SelectItem>
                <SelectItem v-for="f in folders" :key="f.id" :value="f.id">
                  {{ f.name }}
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div class="flex items-center justify-between gap-3 rounded-md border border-border/60 px-3 py-2.5">
            <div>
              <p class="text-[13px] font-medium">{{ t("settings.feeds.pause") }}</p>
              <p
                v-if="editFeed?.lastError"
                class="mt-0.5 line-clamp-2 text-[11px] text-destructive/90"
              >
                {{ editFeed.lastError }}
              </p>
            </div>
            <Switch v-model:checked="editPaused" :disabled="editSaving" />
          </div>
          <div class="flex items-center justify-between gap-3 rounded-md border border-border/60 px-3 py-2.5">
            <div class="min-w-0">
              <p class="text-[13px] font-medium">{{ t("settings.feeds.nsfw") }}</p>
              <p class="mt-0.5 text-[11.5px] text-muted-foreground">
                {{ t("settings.feeds.nsfwDesc") }}
              </p>
            </div>
            <Switch v-model:checked="editNsfw" :disabled="editSaving" />
          </div>
          <div class="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              :disabled="editSaving || editRefreshing || !editFeedId"
              @click="onEditRefresh"
            >
              {{
                editRefreshing
                  ? t("settings.feeds.refreshing")
                  : t("settings.feeds.refreshNow")
              }}
            </Button>
            <Button
              type="button"
              variant="destructive"
              size="sm"
              :disabled="editSaving || deleteFeedBusy"
              @click="openDeleteFeed"
            >
              {{ t("settings.feeds.deleteFeed") }}
            </Button>
          </div>
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            size="sm"
            :disabled="editSaving"
            @click="editOpen = false"
          >
            {{ t("common.cancel") }}
          </Button>
          <Button
            type="button"
            size="sm"
            :disabled="editSaving || !editTitle.trim()"
            @click="confirmEdit"
          >
            {{ editSaving ? t("common.saving") : t("common.save") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

    <AlertDialog
      :open="deleteFeedOpen"
      @update:open="(v) => !deleteFeedBusy && (deleteFeedOpen = v)"
    >
      <AlertDialogContent class="sm:max-w-sm">
        <AlertDialogHeader>
          <AlertDialogMedia class="bg-destructive/10 text-destructive">
            <TriangleAlert />
          </AlertDialogMedia>
          <AlertDialogTitle>{{ t("settings.feeds.deleteFeedConfirmTitle") }}</AlertDialogTitle>
          <AlertDialogDescription class="text-[13px] leading-relaxed">
            {{
              t("settings.feeds.deleteFeedConfirmBody", {
                name: editTitle || editFeed?.title || "",
              })
            }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel size="sm" :disabled="deleteFeedBusy">
            {{ t("common.cancel") }}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            size="sm"
            :disabled="deleteFeedBusy"
            @click="confirmDeleteFeed"
          >
            {{
              deleteFeedBusy
                ? t("common.loading")
                : t("settings.feeds.deleteFeed")
            }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>

    <AlertDialog :open="confirmClearOpen" @update:open="onConfirmOpenChange">
      <AlertDialogContent class="sm:max-w-sm">
        <AlertDialogHeader>
          <AlertDialogMedia class="bg-destructive/10 text-destructive">
            <TriangleAlert />
          </AlertDialogMedia>
          <AlertDialogTitle>{{ t("danger.clear.confirmTitle") }}</AlertDialogTitle>
          <AlertDialogDescription class="text-[13px] leading-relaxed">
            {{ t("danger.clear.confirmBody", { n: subscriptionCount }) }}
            <span class="mt-1.5 block font-medium text-destructive">
              {{ t("danger.clear.confirmIrreversible") }}
            </span>
            {{ t("danger.clear.confirmBackup") }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel size="sm" :disabled="clearing">
            {{ t("common.cancel") }}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            size="sm"
            :disabled="clearing"
            @click="confirmClearAll"
          >
            {{ clearing ? t("danger.clear.clearing") : t("danger.clear.confirmAction") }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </div>
</template>
