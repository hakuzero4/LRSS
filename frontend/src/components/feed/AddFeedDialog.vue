<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { ChevronDown, FolderPlus } from "@lucide/vue";
import { useRssStore } from "@/composables/useRssStore";
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
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const {
  addFeedOpen,
  addFeedTargetFolderId,
  folders,
  settings,
  closeAddFeed,
  setAddFeedFolderId,
  addFeedFromURL,
  createFolder,
} = useRssStore();

const feedUrl = ref("");
const title = ref("");
const submitting = ref(false);
const error = ref("");
const advancedOpen = ref(false);
const isNsfw = ref(false);
/** "0" = follow global; otherwise minutes as string. */
const refreshInterval = ref("0");

const creatingFolder = ref(false);
const newFolderName = ref("");
const folderBusy = ref(false);
const newFolderInputRef = ref<HTMLInputElement | null>(null);

const INTERVAL_OPTIONS = [0, 5, 15, 30, 60, 120, 180] as const;

const canSubmit = computed(
  () => feedUrl.value.trim().length > 8 && !submitting.value && !folderBusy.value,
);

const folderModel = computed({
  get: () => addFeedTargetFolderId.value ?? "none",
  set: (v: string) => setAddFeedFolderId(v === "none" ? null : v),
});

const targetFolderName = computed(() => {
  const id = addFeedTargetFolderId.value;
  if (!id) return "";
  return folders.value.find((f) => f.id === id)?.name ?? "";
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

/** Soft-suggest NSFW for known adult aggregator hosts (user can still uncheck). */
function suggestNsfwFromUrl(raw: string): boolean {
  try {
    const host = new URL(raw).hostname.toLowerCase();
    return (
      host.includes("hentai") ||
      host.includes("bgzo") ||
      host.includes("nhentai") ||
      host.includes("e-hentai") ||
      host.includes("exhentai")
    );
  } catch {
    return false;
  }
}

function resetForm() {
  feedUrl.value = "";
  title.value = "";
  error.value = "";
  submitting.value = false;
  advancedOpen.value = false;
  isNsfw.value = false;
  refreshInterval.value = "0";
  creatingFolder.value = false;
  newFolderName.value = "";
  folderBusy.value = false;
}

watch(addFeedOpen, (open) => {
  if (!open) {
    resetForm();
  } else if (!addFeedTargetFolderId.value && settings.defaultFolderId) {
    setAddFeedFolderId(settings.defaultFolderId);
  }
});

watch(feedUrl, (url) => {
  if (!advancedOpen.value) {
    isNsfw.value = suggestNsfwFromUrl(url.trim());
  }
});

function toggleAdvanced() {
  advancedOpen.value = !advancedOpen.value;
  if (advancedOpen.value && suggestNsfwFromUrl(feedUrl.value.trim())) {
    isNsfw.value = true;
  }
}

async function openCreateFolder() {
  creatingFolder.value = true;
  newFolderName.value = "";
  await nextTick();
  newFolderInputRef.value?.focus();
}

function cancelCreateFolder() {
  creatingFolder.value = false;
  newFolderName.value = "";
}

async function confirmCreateFolder() {
  const name = newFolderName.value.trim();
  if (!name || folderBusy.value) return;
  folderBusy.value = true;
  error.value = "";
  try {
    const id = await createFolder(name);
    if (id) {
      setAddFeedFolderId(id);
      creatingFolder.value = false;
      newFolderName.value = "";
    } else {
      error.value = t("feed.add.newFolderFailed");
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : t("feed.add.newFolderFailed");
  } finally {
    folderBusy.value = false;
  }
}

async function onSubmit() {
  error.value = "";
  const url = feedUrl.value.trim();
  if (!url) {
    error.value = t("feed.add.errorEmpty");
    return;
  }
  try {
    // eslint-disable-next-line no-new
    new URL(url);
  } catch {
    error.value = t("feed.add.errorInvalid");
    return;
  }

  submitting.value = true;
  try {
    await addFeedFromURL(url, {
      title: title.value.trim() || undefined,
      isNsfw: isNsfw.value,
      refreshIntervalMinutes: Number(refreshInterval.value) || 0,
    });
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e);
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Dialog :open="addFeedOpen" @update:open="(v) => !v && closeAddFeed()">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ t("feed.add.title") }}</DialogTitle>
        <DialogDescription>
          <template v-if="targetFolderName">
            {{ t("feed.add.descriptionInFolder", { name: targetFolderName }) }}
          </template>
          <template v-else>
            {{ t("feed.add.description") }}
          </template>
        </DialogDescription>
      </DialogHeader>

      <form class="grid gap-4 py-1" @submit.prevent="onSubmit">
        <div class="grid gap-2">
          <Label for="feed-url">{{ t("feed.add.urlLabel") }}</Label>
          <Input
            id="feed-url"
            v-model="feedUrl"
            type="url"
            :placeholder="t('feed.add.urlPlaceholder')"
            autocomplete="off"
            autofocus
          />
        </div>
        <div class="grid gap-2">
          <Label for="feed-title">
            {{ t("feed.add.titleLabel") }}
            <span class="font-normal text-muted-foreground">{{ t("common.optional") }}</span>
          </Label>
          <Input
            id="feed-title"
            v-model="title"
            type="text"
            :placeholder="t('feed.add.titlePlaceholder')"
            autocomplete="off"
          />
        </div>

        <div class="grid gap-2">
          <div class="flex items-center justify-between gap-2">
            <Label>{{ t("feed.add.folderLabel") }}</Label>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              class="h-7 gap-1 px-2 text-[12px] text-muted-foreground hover:text-foreground"
              :disabled="folderBusy || submitting"
              @click="openCreateFolder"
            >
              <FolderPlus class="size-3.5" />
              {{ t("feed.add.newFolder") }}
            </Button>
          </div>
          <Select v-model="folderModel" :disabled="folderBusy || submitting">
            <SelectTrigger class="h-9 w-full text-[13px]">
              <SelectValue :placeholder="t('settings.feeds.unfiled')" />
            </SelectTrigger>
            <SelectContent position="popper" class="w-[var(--reka-select-trigger-width)]">
              <SelectItem value="none">{{ t("settings.feeds.unfiled") }}</SelectItem>
              <SelectItem v-for="f in folders" :key="f.id" :value="f.id">
                {{ f.name }}
              </SelectItem>
            </SelectContent>
          </Select>

          <div
            v-if="creatingFolder"
            class="flex items-center gap-2 rounded-lg border border-border/80 bg-muted/15 p-2"
          >
            <Input
              ref="newFolderInputRef"
              v-model="newFolderName"
              class="h-8 flex-1 text-[13px]"
              :placeholder="t('feed.add.newFolderPlaceholder')"
              :disabled="folderBusy"
              autocomplete="off"
              @keydown.enter.prevent="confirmCreateFolder"
              @keydown.escape.prevent="cancelCreateFolder"
            />
            <Button
              type="button"
              size="sm"
              class="h-8 shrink-0"
              :disabled="folderBusy || !newFolderName.trim()"
              @click="confirmCreateFolder"
            >
              {{ folderBusy ? t("feed.add.newFolderCreating") : t("feed.add.newFolderConfirm") }}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              class="h-8 shrink-0 px-2"
              :disabled="folderBusy"
              @click="cancelCreateFolder"
            >
              {{ t("common.cancel") }}
            </Button>
          </div>
        </div>

        <!-- Advanced options (NSFW / interval) — collapsible, shadcn-style -->
        <div class="rounded-lg border border-border/80 bg-muted/20">
          <button
            type="button"
            class="flex w-full items-center justify-between gap-2 rounded-lg px-3 py-2.5 text-left text-[13px] font-medium text-foreground transition-colors hover:bg-muted/40"
            :aria-expanded="advancedOpen"
            @click="toggleAdvanced"
          >
            <span class="flex items-center gap-2">
              {{ t("feed.add.advanced") }}
              <span
                v-if="isNsfw || Number(refreshInterval) > 0"
                class="rounded-md bg-primary/10 px-1.5 py-0.5 text-[11px] font-normal text-primary"
              >
                {{ t("feed.add.advancedActive") }}
              </span>
            </span>
            <ChevronDown
              class="size-4 shrink-0 text-muted-foreground transition-transform duration-200"
              :class="advancedOpen ? 'rotate-180' : ''"
            />
          </button>

          <div v-if="advancedOpen" class="grid gap-3 px-3 pb-3 pt-0">
            <Separator class="opacity-60" />

            <div class="flex items-start justify-between gap-3">
              <div class="grid gap-0.5 pr-2">
                <Label for="feed-nsfw" class="text-[13px] font-medium leading-none">
                  {{ t("feed.add.nsfw") }}
                </Label>
                <p class="text-[12px] leading-snug text-muted-foreground">
                  {{ t("feed.add.nsfwDesc") }}
                </p>
              </div>
              <Switch id="feed-nsfw" v-model:checked="isNsfw" class="mt-0.5" />
            </div>

            <div class="grid gap-2">
              <Label class="text-[13px]">{{ t("feed.add.refreshInterval") }}</Label>
              <Select v-model="refreshInterval">
                <SelectTrigger class="h-9 w-full text-[13px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper" class="w-[var(--reka-select-trigger-width)]">
                  <SelectItem
                    v-for="m in INTERVAL_OPTIONS"
                    :key="m"
                    :value="String(m)"
                  >
                    {{ intervalOptionLabel(m) }}
                  </SelectItem>
                </SelectContent>
              </Select>
              <p class="text-[11.5px] leading-snug text-muted-foreground">
                {{ t("feed.add.refreshIntervalHint") }}
              </p>
            </div>
          </div>
        </div>

        <p v-if="error" class="text-[12.5px] text-destructive">{{ error }}</p>

        <DialogFooter class="gap-2 sm:gap-0">
          <Button type="button" variant="ghost" @click="closeAddFeed">
            {{ t("common.cancel") }}
          </Button>
          <Button type="submit" :disabled="!canSubmit">
            {{ submitting ? t("feed.add.submitting") : t("feed.add.submit") }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
