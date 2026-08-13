<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import SettingsFeedRow from "@/components/settings/SettingsFeedRow.vue";
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
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { relativeTime } from "@/lib/format";
import { virtualWindow } from "@/lib/virtualWindow";
import type { Feed } from "@/types/rss";
import { AlertCircle, Plus, Search, Trash2, TriangleAlert } from "@lucide/vue";

/** Fixed row height for the virtualized subscription list (px). */
const FEED_ROW_H = 68;
const FEED_LIST_MAX_H = 280;

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
  deleteFeed,
  deleteFailedFeeds,
  openAddFeed,
  openFeedEdit,
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
const deleteFailedOpen = ref(false);
const deleteFailedBusy = ref(false);

const feedFilter = ref("");
/** Debounced query so typing in a large library does not re-filter every key. */
const feedFilterQ = ref("");
/** When true, only show feeds with a non-empty lastError. */
const feedErrorsOnly = ref(false);

let filterTimer: ReturnType<typeof setTimeout> | null = null;
watch(feedFilter, (v) => {
  if (filterTimer) clearTimeout(filterTimer);
  filterTimer = setTimeout(() => {
    feedFilterQ.value = v;
  }, 120);
});

const subscriptionCount = computed(() => feeds.value.length);

const errorFeedCount = computed(
  () => feeds.value.filter((f) => !!f.lastError?.trim()).length,
);

const folderNameById = computed(() => {
  const m = new Map<string, string>();
  for (const f of folders.value) m.set(f.id, f.name);
  return m;
});

const unfiledLabel = computed(() => t("settings.feeds.unfiled"));
const neverUpdatedLabel = computed(() => t("settings.feeds.neverUpdated"));

const sortedFeeds = computed(() => {
  const q = feedFilterQ.value.trim().toLowerCase();
  let list = feeds.value;
  if (feedErrorsOnly.value) {
    list = list.filter((f) => !!f.lastError?.trim());
  }
  if (q) {
    list = list.filter(
      (f) =>
        f.title.toLowerCase().includes(q) ||
        f.feedUrl.toLowerCase().includes(q) ||
        (f.siteUrl?.toLowerCase().includes(q) ?? false) ||
        (f.lastError?.toLowerCase().includes(q) ?? false),
    );
  }
  return list.slice().sort((a, b) =>
    a.title.localeCompare(b.title, undefined, { sensitivity: "base" }),
  );
});

const listFilterActive = computed(
  () => feedErrorsOnly.value || !!feedFilter.value.trim(),
);

const listEl = ref<HTMLElement | null>(null);
const listScrollTop = ref(0);
const listViewportH = ref(FEED_LIST_MAX_H);

const listBoxH = computed(() => {
  const n = sortedFeeds.value.length;
  if (n === 0) return 120;
  return Math.min(FEED_LIST_MAX_H, n * FEED_ROW_H);
});

const windowed = computed(() => {
  const list = sortedFeeds.value;
  const win = virtualWindow({
    count: list.length,
    scrollTop: listScrollTop.value,
    viewportH: listViewportH.value,
    itemH: FEED_ROW_H,
    overscan: 6,
  });
  return {
    ...win,
    items: list.slice(win.start, win.end),
  };
});

function onListScroll(e: Event) {
  listScrollTop.value = (e.target as HTMLElement).scrollTop;
}

let listRo: ResizeObserver | null = null;
watch(listEl, (el) => {
  listRo?.disconnect();
  listRo = null;
  if (!el || typeof ResizeObserver === "undefined") return;
  const measure = () => {
    listViewportH.value = el.clientHeight || FEED_LIST_MAX_H;
  };
  measure();
  listRo = new ResizeObserver(measure);
  listRo.observe(el);
});
watch([feedFilterQ, feedErrorsOnly], () => {
  listScrollTop.value = 0;
  if (listEl.value) listEl.value.scrollTop = 0;
});
onUnmounted(() => {
  listRo?.disconnect();
  listRo = null;
  if (filterTimer) clearTimeout(filterTimer);
});

function lastUpdatedLabel(feed: Feed): string {
  const iso = feed.lastFetchedAt?.trim();
  if (!iso) return neverUpdatedLabel.value;
  const tms = Date.parse(iso);
  if (Number.isNaN(tms)) return neverUpdatedLabel.value;
  return relativeTime(iso);
}

function folderName(feed: Feed): string {
  if (!feed.folderId) return unfiledLabel.value;
  return folderNameById.value.get(feed.folderId) ?? unfiledLabel.value;
}

const busy = computed(
  () =>
    importing.value ||
    exporting.value ||
    clearing.value ||
    purging.value ||
    deleteFailedBusy.value,
);

function openDeleteFailed() {
  if (errorFeedCount.value === 0 || deleteFailedBusy.value) return;
  deleteFailedOpen.value = true;
}

async function confirmDeleteFailed(ev: Event) {
  ev.preventDefault();
  if (deleteFailedBusy.value || errorFeedCount.value === 0) return;
  deleteFailedBusy.value = true;
  try {
    const { deleted, failed } = await deleteFailedFeeds();
    deleteFailedOpen.value = false;
    feedErrorsOnly.value = false;
    if (failed > 0 && deleted > 0) {
      toast.success(t("settings.feeds.deleteFailedPartial", { ok: deleted, fail: failed }));
    } else if (deleted > 0) {
      toast.success(t("settings.feeds.deleteFailedDone", { n: deleted }));
    } else if (failed > 0) {
      toast.error(t("settings.feeds.deleteFailedAllFailed", { n: failed }));
    } else {
      toast.message(t("settings.feeds.deleteFailedNone"));
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.deleteFailedError"), { description: msg });
  } finally {
    deleteFailedBusy.value = false;
  }
}

function openAddFeeds() {
  openAddFeed();
}

// ── Row unsubscribe (full edit dialog is shared FeedEditDialog) ─
const deleteFeedOpen = ref(false);
const deleteFeedBusy = ref(false);
const deleteFeedId = ref<string | null>(null);
const deleteFeedTitle = ref("");

function openEdit(feed: Feed) {
  openFeedEdit(feed.id);
}

function openDeleteFeed(feed: Feed) {
  deleteFeedId.value = feed.id;
  deleteFeedTitle.value = feed.title;
  deleteFeedOpen.value = true;
}

async function confirmDeleteFeed(ev: Event) {
  ev.preventDefault();
  if (!deleteFeedId.value || deleteFeedBusy.value) return;
  deleteFeedBusy.value = true;
  try {
    await deleteFeed(deleteFeedId.value);
    deleteFeedOpen.value = false;
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

function onFetchFullContent(v: boolean) {
  settings.fetchFullContent = v;
  persistUIPrefs();
}

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
      t("opml.partUpdated", { n: result.feedsUpdated }),
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
          <template v-else-if="listFilterActive">
            {{
              t("settings.feeds.listFiltered", {
                shown: sortedFeeds.length,
                n: subscriptionCount,
              })
            }}
            <span v-if="feedErrorsOnly" class="text-destructive/80">
              · {{ t("settings.feeds.listErrorsOnlyHint") }}
            </span>
          </template>
          <template v-else>
            {{ t("settings.feeds.listCount", { n: subscriptionCount }) }}
            <span v-if="errorFeedCount > 0" class="text-destructive/80">
              · {{ t("settings.feeds.listErrorCount", { n: errorFeedCount }) }}
            </span>
          </template>
        </p>
        <div class="flex flex-wrap items-center gap-2">
          <Button
            v-if="subscriptionCount > 0 && errorFeedCount > 0"
            type="button"
            size="sm"
            variant="outline"
            class="h-8 shrink-0 gap-1 px-2.5 text-[12px]"
            :class="
              feedErrorsOnly
                ? 'border-destructive/40 bg-destructive/10 text-destructive hover:bg-destructive/15 hover:text-destructive'
                : 'text-muted-foreground'
            "
            :aria-pressed="feedErrorsOnly"
            :aria-label="t('settings.feeds.filterErrors')"
            :disabled="busy"
            @click="feedErrorsOnly = !feedErrorsOnly"
          >
            <AlertCircle class="size-3.5" />
            {{
              feedErrorsOnly
                ? t("settings.feeds.filterErrorsOn", { n: errorFeedCount })
                : t("settings.feeds.filterErrors", { n: errorFeedCount })
            }}
          </Button>
          <Button
            v-if="subscriptionCount > 0 && errorFeedCount > 0"
            type="button"
            size="sm"
            variant="outline"
            class="h-8 shrink-0 gap-1 px-2.5 text-[12px] text-destructive hover:bg-destructive/10 hover:text-destructive"
            :disabled="busy"
            :aria-label="t('settings.feeds.deleteFailedFeeds')"
            @click="openDeleteFailed"
          >
            <Trash2 class="size-3.5 opacity-80" />
            {{
              deleteFailedBusy
                ? t("settings.feeds.deleteFailedBusy")
                : t("settings.feeds.deleteFailedFeeds", { n: errorFeedCount })
            }}
          </Button>
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
        <!-- Windowed list: only mount visible rows (large OPML libraries). -->
        <div
          ref="listEl"
          class="overflow-y-auto overscroll-contain rounded-lg border border-border/70"
          role="region"
          :aria-label="t('settings.feeds.listGroup')"
          :style="{ height: `${listBoxH}px` }"
          @scroll.passive="onListScroll"
        >
          <div
            v-if="sortedFeeds.length === 0"
            class="px-3 py-6 text-center text-[12px] text-muted-foreground"
          >
            {{
              feedErrorsOnly
                ? t("settings.feeds.listNoErrors")
                : feedFilter.trim()
                  ? t("settings.feeds.listNoMatch")
                  : t("settings.feeds.listEmpty")
            }}
          </div>
          <div
            v-else
            :style="{ height: `${windowed.totalH}px`, position: 'relative' }"
          >
            <div
              :style="{
                transform: `translateY(${windowed.padTop}px)`,
                willChange: 'transform',
              }"
            >
              <div
                v-for="feed in windowed.items"
                :key="feed.id"
                class="border-b border-border/70 last:border-b-0"
                :style="{ height: `${FEED_ROW_H}px` }"
              >
                <SettingsFeedRow
                  :feed="feed"
                  :folder-label="folderName(feed)"
                  :updated-label="lastUpdatedLabel(feed)"
                  :busy="busy || deleteFeedBusy"
                  @edit="openEdit(feed)"
                  @remove="openDeleteFeed(feed)"
                />
              </div>
            </div>
          </div>
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
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.feeds.fetchFullContent')"
          :description="t('settings.feeds.fetchFullContentDesc')"
        >
          <Switch
            :checked="settings.fetchFullContent"
            :aria-label="t('settings.feeds.fetchFullContent')"
            @update:checked="onFetchFullContent"
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
                name: deleteFeedTitle,
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

    <AlertDialog
      :open="deleteFailedOpen"
      @update:open="(v) => !deleteFailedBusy && (deleteFailedOpen = v)"
    >
      <AlertDialogContent class="sm:max-w-sm">
        <AlertDialogHeader>
          <AlertDialogMedia class="bg-destructive/10 text-destructive">
            <TriangleAlert />
          </AlertDialogMedia>
          <AlertDialogTitle>
            {{ t("settings.feeds.deleteFailedConfirmTitle") }}
          </AlertDialogTitle>
          <AlertDialogDescription class="text-[13px] leading-relaxed">
            {{ t("settings.feeds.deleteFailedConfirmBody", { n: errorFeedCount }) }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel size="sm" :disabled="deleteFailedBusy">
            {{ t("common.cancel") }}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            size="sm"
            :disabled="deleteFailedBusy || errorFeedCount === 0"
            @click="confirmDeleteFailed"
          >
            {{
              deleteFailedBusy
                ? t("settings.feeds.deleteFailedBusy")
                : t("settings.feeds.deleteFailedConfirmAction", { n: errorFeedCount })
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
