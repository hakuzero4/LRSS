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
import type { Feed } from "@/types/rss";
import { Pencil, TriangleAlert } from "@lucide/vue";

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

const subscriptionCount = computed(() => feeds.value.length);

const sortedFeeds = computed(() =>
  [...feeds.value].sort((a, b) =>
    a.title.localeCompare(b.title, undefined, { sensitivity: "base" }),
  ),
);

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

function intervalValue(feed: Feed): string {
  const n = feed.refreshIntervalMinutes ?? 0;
  // If user somehow has a non-preset value, still show it via select string.
  return String(n);
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

const busy = computed(() => importing.value || exporting.value || clearing.value || purging.value);

async function onIntervalChange(feed: Feed, raw: string) {
  const minutes = Math.max(0, Math.floor(Number(raw) || 0));
  try {
    await setFeedRefreshInterval(feed.id, minutes);
    toast.success(t("settings.feeds.intervalSaved"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.intervalFailed"), { description: msg });
  }
}

async function onPauseChange(feed: Feed, paused: boolean) {
  try {
    await setFeedPaused(feed.id, paused);
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.pauseFailed"), { description: msg });
  }
}

// ── Rename dialog ──────────────────────────────────────────────
const renameOpen = ref(false);
const renameFeedId = ref<string | null>(null);
const renameDraft = ref("");
const renameSaving = ref(false);

function openRename(feed: Feed) {
  renameFeedId.value = feed.id;
  renameDraft.value = feed.title;
  renameOpen.value = true;
}

async function confirmRename() {
  if (!renameFeedId.value || renameSaving.value) return;
  const title = renameDraft.value.trim();
  if (!title) {
    toast.error(t("settings.feeds.renameEmpty"));
    return;
  }
  renameSaving.value = true;
  try {
    await renameFeed(renameFeedId.value, title);
    renameOpen.value = false;
    toast.success(t("settings.feeds.renameSaved"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.renameFailed"), { description: msg });
  } finally {
    renameSaving.value = false;
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
    <!-- All subscriptions -->
    <SettingsGroup
      :title="t('settings.feeds.listGroup')"
      :description="t('settings.feeds.listGroupDesc')"
    >
      <p
        v-if="subscriptionCount === 0"
        class="py-4 text-center text-[12.5px] text-muted-foreground"
      >
        {{ t("settings.feeds.listEmpty") }}
      </p>
      <template v-else>
        <p class="pb-2 pt-1 text-[11.5px] tabular-nums text-muted-foreground">
          {{ t("settings.feeds.listCount", { n: subscriptionCount }) }}
        </p>
        <ul class="divide-y divide-border/70 rounded-lg border border-border/70">
          <li
            v-for="feed in sortedFeeds"
            :key="feed.id"
            class="flex flex-col gap-2.5 px-3 py-3 sm:flex-row sm:items-start sm:justify-between sm:gap-4"
          >
            <div class="min-w-0 flex-1">
              <div class="flex items-start gap-2.5">
                <FeedIcon
                  :src="feed.favicon"
                  :title="feed.title"
                  size="md"
                  class="mt-0.5"
                />
                <div class="min-w-0 flex-1">
                  <div class="flex flex-wrap items-center gap-1.5">
                    <p class="truncate text-[13px] font-medium leading-snug">
                      {{ feed.title }}
                    </p>
                    <span
                      v-if="feed.isPaused"
                      class="rounded bg-muted px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground"
                    >
                      {{ t("settings.feeds.paused") }}
                    </span>
                  </div>
                  <p
                    class="mt-0.5 truncate text-[11.5px] text-muted-foreground"
                    :title="feed.feedUrl"
                  >
                    {{ feed.feedUrl }}
                  </p>
                  <p class="mt-1 text-[11.5px] text-muted-foreground">
                    <span class="text-foreground/70">{{ t("settings.feeds.lastUpdated") }}</span>
                    ·
                    <span class="tabular-nums">{{ lastUpdatedLabel(feed) }}</span>
                    ·
                    <span>{{ folderName(feed) }}</span>
                  </p>
                  <p
                    v-if="feed.lastError"
                    class="mt-1 line-clamp-2 text-[11px] text-destructive/90"
                    :title="feed.lastError"
                  >
                    {{ feed.lastError }}
                  </p>
                </div>
              </div>
            </div>

            <div class="flex shrink-0 flex-col gap-2 sm:items-end">
              <div class="flex flex-wrap items-center gap-2 sm:justify-end">
                <Select
                  :model-value="intervalValue(feed)"
                  :disabled="busy || feed.isPaused"
                  @update:model-value="(v) => onIntervalChange(feed, String(v))"
                >
                  <SelectTrigger class="h-8 w-[168px] text-[12px]">
                    <SelectValue :placeholder="t('settings.feeds.refreshInterval')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem
                      v-for="opt in INTERVAL_OPTIONS"
                      :key="opt"
                      :value="String(opt)"
                      class="text-[12.5px]"
                    >
                      {{ intervalOptionLabel(opt) }}
                    </SelectItem>
                  </SelectContent>
                </Select>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  class="h-8 gap-1 px-2.5 text-[12px]"
                  :disabled="busy"
                  @click="openRename(feed)"
                >
                  <Pencil class="size-3.5 opacity-70" />
                  {{ t("settings.feeds.editName") }}
                </Button>
              </div>
              <label
                class="flex cursor-pointer items-center gap-2 text-[11.5px] text-muted-foreground"
              >
                <Switch
                  :checked="!!feed.isPaused"
                  :disabled="busy"
                  @update:checked="(v) => onPauseChange(feed, v)"
                />
                {{ t("settings.feeds.pause") }}
              </label>
            </div>
          </li>
        </ul>
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

    <!-- Rename dialog -->
    <Dialog :open="renameOpen" @update:open="(v) => (renameOpen = v)">
      <DialogContent class="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{{ t("settings.feeds.renameTitle") }}</DialogTitle>
          <DialogDescription>{{ t("settings.feeds.renameDesc") }}</DialogDescription>
        </DialogHeader>
        <div class="space-y-2 py-1">
          <label class="text-[12.5px] font-medium" for="feed-rename-input">
            {{ t("settings.feeds.renameLabel") }}
          </label>
          <Input
            id="feed-rename-input"
            v-model="renameDraft"
            :placeholder="t('settings.feeds.renamePlaceholder')"
            class="h-9"
            :disabled="renameSaving"
            @keydown.enter.prevent="confirmRename"
          />
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            size="sm"
            :disabled="renameSaving"
            @click="renameOpen = false"
          >
            {{ t("common.cancel") }}
          </Button>
          <Button
            type="button"
            size="sm"
            :disabled="renameSaving || !renameDraft.trim()"
            @click="confirmRename"
          >
            {{ renameSaving ? t("common.saving") : t("settings.feeds.saveName") }}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>

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
