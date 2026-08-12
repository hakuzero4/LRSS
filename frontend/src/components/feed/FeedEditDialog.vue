<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
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
import { Switch } from "@/components/ui/switch";
import { TriangleAlert } from "@lucide/vue";

const { t } = useI18n();

const {
  feeds,
  folders,
  settings,
  feedEditOpen,
  feedEditId,
  closeFeedEdit,
  renameFeed,
  setFeedUrl,
  setFeedRefreshInterval,
  setFeedKeepArticlesDays,
  setFeedPaused,
  moveFeedToFolder,
  refreshOneFeed,
  deleteFeed,
  setFeedNsfw,
} = useRssStore();

/** Interval presets (minutes). 0 = follow global default. */
const INTERVAL_OPTIONS = [0, 5, 15, 30, 60, 120, 180] as const;
/** Keep-days presets. 0 = follow global keepArticlesDays. */
const KEEP_DAYS_OPTIONS = [0, 7, 14, 30, 60, 90, 180, 365] as const;

const editTitle = ref("");
const editInterval = ref("0");
const editKeepDays = ref("0");
const editFolderId = ref("none");
const editPaused = ref(false);
const editNsfw = ref(false);
const editFeedUrl = ref("");
const editSaving = ref(false);
const editRefreshing = ref(false);
const deleteOpen = ref(false);
const deleteBusy = ref(false);

const editFeed = computed(() =>
  feedEditId.value
    ? (feeds.value.find((f) => f.id === feedEditId.value) ?? null)
    : null,
);

function formatIntervalMinutes(m: number): string {
  if (m < 60) return t("common.minutes", { n: m });
  if (m === 60) return t("common.oneHour");
  if (m % 60 === 0) return t("common.hours", { n: m / 60 });
  return t("common.minutes", { n: m });
}

const globalIntervalLabel = computed(() =>
  formatIntervalMinutes(settings.refreshIntervalMinutes),
);

function intervalOptionLabel(minutes: number): string {
  if (minutes === 0) {
    return t("settings.feeds.intervalDefault", { n: globalIntervalLabel.value });
  }
  return t("settings.feeds.intervalCustom", {
    n: formatIntervalMinutes(minutes),
  });
}

const globalKeepDaysLabel = computed(() =>
  t("common.days", { n: settings.keepArticlesDays }),
);

function keepDaysOptionLabel(days: number): string {
  if (days === 0) {
    return t("settings.feeds.keepDaysDefault", { n: globalKeepDaysLabel.value });
  }
  return t("settings.feeds.keepDaysCustom", {
    n: t("common.days", { n: days }),
  });
}

function loadFromFeed() {
  const feed = editFeed.value;
  if (!feed) return;
  editTitle.value = feed.title;
  editInterval.value = String(feed.refreshIntervalMinutes ?? 0);
  editKeepDays.value = String(feed.keepArticlesDays ?? 0);
  editFolderId.value = feed.folderId ?? "none";
  editPaused.value = !!feed.isPaused;
  editNsfw.value = !!feed.isNsfw;
  editFeedUrl.value = feed.feedUrl;
  deleteOpen.value = false;
}

watch(
  () => [feedEditOpen.value, feedEditId.value] as const,
  ([open]) => {
    if (open) loadFromFeed();
  },
);

function onOpenChange(open: boolean) {
  if (!open) {
    if (editSaving.value || deleteBusy.value) return;
    closeFeedEdit();
  }
}

async function confirmEdit() {
  if (!feedEditId.value || editSaving.value) return;
  const title = editTitle.value.trim();
  if (!title) {
    toast.error(t("settings.feeds.renameEmpty"));
    return;
  }
  const url = editFeedUrl.value.trim();
  if (!url) {
    toast.error(t("settings.feeds.feedUrlEmpty"));
    return;
  }
  editSaving.value = true;
  const id = feedEditId.value;
  try {
    const current = feeds.value.find((f) => f.id === id);
    if (!current) throw new Error("feed missing");

    if (title !== current.title) {
      await renameFeed(id, title);
    }
    if (url !== current.feedUrl) {
      await setFeedUrl(id, url);
    }
    const minutes = Math.max(0, Math.floor(Number(editInterval.value) || 0));
    if (minutes !== (current.refreshIntervalMinutes ?? 0)) {
      await setFeedRefreshInterval(id, minutes);
    }
    const keepDays = Math.max(0, Math.floor(Number(editKeepDays.value) || 0));
    if (keepDays !== (current.keepArticlesDays ?? 0)) {
      await setFeedKeepArticlesDays(id, keepDays);
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
    closeFeedEdit();
    toast.success(t("settings.feeds.saved"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.saveFailed"), { description: msg });
  } finally {
    editSaving.value = false;
  }
}

async function onEditRefresh() {
  if (!feedEditId.value || editRefreshing.value) return;
  editRefreshing.value = true;
  try {
    const n = await refreshOneFeed(feedEditId.value);
    toast.success(t("settings.feeds.refreshDone", { n }));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.refreshFailed"), { description: msg });
  } finally {
    editRefreshing.value = false;
  }
}

async function confirmDelete(ev: Event) {
  ev.preventDefault();
  if (!feedEditId.value || deleteBusy.value) return;
  deleteBusy.value = true;
  try {
    await deleteFeed(feedEditId.value);
    deleteOpen.value = false;
    closeFeedEdit();
    toast.success(t("settings.feeds.deleteFeedDone"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.deleteFeedFailed"), { description: msg });
  } finally {
    deleteBusy.value = false;
  }
}
</script>

<template>
  <Dialog :open="feedEditOpen" @update:open="onOpenChange">
    <!-- Above Settings (z-50): nested edit must stack higher, including overlay. -->
    <DialogContent
      class="z-[70] sm:max-w-md"
      overlay-class="z-[70]"
    >
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
          <Label for="feed-edit-url">{{ t("settings.feeds.feedUrlLabel") }}</Label>
          <Input
            id="feed-edit-url"
            v-model="editFeedUrl"
            type="url"
            spellcheck="false"
            autocomplete="off"
            class="h-9 font-mono text-[12.5px]"
            :placeholder="t('settings.feeds.feedUrlPlaceholder')"
            :disabled="editSaving"
          />
          <p class="text-[11.5px] leading-relaxed text-muted-foreground">
            {{ t("settings.feeds.feedUrlHint") }}
          </p>
        </div>
        <div class="grid gap-1.5">
          <Label>{{ t("settings.feeds.refreshInterval") }}</Label>
          <Select v-model="editInterval" :disabled="editSaving || editPaused">
            <SelectTrigger class="h-9 w-full text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent
              position="popper"
              class="z-[80] w-[var(--reka-select-trigger-width)]"
            >
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
          <Label>{{ t("settings.feeds.keepDaysPerFeed") }}</Label>
          <Select v-model="editKeepDays" :disabled="editSaving">
            <SelectTrigger class="h-9 w-full text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent
              position="popper"
              class="z-[80] w-[var(--reka-select-trigger-width)]"
            >
              <SelectItem
                v-for="opt in KEEP_DAYS_OPTIONS"
                :key="opt"
                :value="String(opt)"
              >
                {{ keepDaysOptionLabel(opt) }}
              </SelectItem>
            </SelectContent>
          </Select>
          <p class="text-[11.5px] leading-relaxed text-muted-foreground">
            {{ t("settings.feeds.keepDaysPerFeedDesc") }}
          </p>
        </div>
        <div class="grid gap-1.5">
          <Label>{{ t("settings.feeds.folderLabel") }}</Label>
          <Select v-model="editFolderId" :disabled="editSaving">
            <SelectTrigger class="h-9 w-full text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent
              position="popper"
              class="z-[80] w-[var(--reka-select-trigger-width)]"
            >
              <SelectItem value="none">{{ t("settings.feeds.unfiled") }}</SelectItem>
              <SelectItem v-for="f in folders" :key="f.id" :value="f.id">
                {{ f.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div
          class="flex items-center justify-between gap-3 rounded-md border border-border/60 px-3 py-2.5"
        >
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
        <div
          class="flex items-center justify-between gap-3 rounded-md border border-border/60 px-3 py-2.5"
        >
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
            :disabled="editSaving || editRefreshing || !feedEditId"
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
            :disabled="editSaving || deleteBusy"
            @click="deleteOpen = true"
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
          @click="closeFeedEdit()"
        >
          {{ t("common.cancel") }}
        </Button>
        <Button
          type="button"
          size="sm"
          :disabled="editSaving || !editTitle.trim() || !editFeedUrl.trim()"
          @click="confirmEdit"
        >
          {{ editSaving ? t("common.saving") : t("common.save") }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>

  <AlertDialog
    :open="deleteOpen"
    @update:open="(v) => !deleteBusy && (deleteOpen = v)"
  >
    <AlertDialogContent class="sm:max-w-sm">
      <AlertDialogHeader>
        <AlertDialogMedia class="bg-destructive/10 text-destructive">
          <TriangleAlert />
        </AlertDialogMedia>
        <AlertDialogTitle>
          {{ t("settings.feeds.deleteFeedConfirmTitle") }}
        </AlertDialogTitle>
        <AlertDialogDescription class="text-[13px] leading-relaxed">
          {{
            t("settings.feeds.deleteFeedConfirmBody", {
              name: editTitle || editFeed?.title || "",
            })
          }}
        </AlertDialogDescription>
      </AlertDialogHeader>
      <AlertDialogFooter>
        <AlertDialogCancel size="sm" :disabled="deleteBusy">
          {{ t("common.cancel") }}
        </AlertDialogCancel>
        <AlertDialogAction
          variant="destructive"
          size="sm"
          :disabled="deleteBusy"
          @click="confirmDelete"
        >
          {{
            deleteBusy ? t("common.loading") : t("settings.feeds.deleteFeed")
          }}
        </AlertDialogAction>
      </AlertDialogFooter>
    </AlertDialogContent>
  </AlertDialog>
</template>
