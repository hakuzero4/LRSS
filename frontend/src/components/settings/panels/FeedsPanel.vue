<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import { TriangleAlert } from "@lucide/vue";

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

const subscriptionCount = computed(() => feeds.value.length);

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
  if (purging.value || importing.value || exporting.value || clearing.value) return;
  purging.value = true;
  try {
    // purgeOldArticles flushes pending SetUIPrefs so keep days is current.
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
  // Allow re-selecting the same file later
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
  // Block dismiss (Esc / overlay) while the wipe is running.
  if (clearing.value && !open) return;
  confirmClearOpen.value = open;
}

async function confirmClearAll(ev: Event) {
  // Keep dialog open until the async wipe finishes (AlertDialogAction closes by default).
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
