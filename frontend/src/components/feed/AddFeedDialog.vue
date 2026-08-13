<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { ChevronDown, FolderPlus } from "@lucide/vue";
import { useRssStore } from "@/composables/useRssStore";
import { parseFeedUrlsFromText } from "@/lib/feedUrls";
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
  addFeedsFromURLs,
  createFolder,
} = useRssStore();

const urlsText = ref("");
const title = ref("");
const submitting = ref(false);
const error = ref("");
const advancedOpen = ref(false);
const isNsfw = ref(false);
/** "0" = follow global; otherwise minutes as string. */
const refreshInterval = ref("0");
const addProgress = ref({ current: 0, total: 0 });

const creatingFolder = ref(false);
const newFolderName = ref("");
const folderBusy = ref(false);
const newFolderInputRef = ref<HTMLInputElement | null>(null);

const INTERVAL_OPTIONS = [0, 5, 15, 30, 60, 120, 180] as const;

const parsedUrls = computed(() => parseFeedUrlsFromText(urlsText.value));

const canSubmit = computed(
  () => parsedUrls.value.length > 0 && !submitting.value && !folderBusy.value,
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
  urlsText.value = "";
  title.value = "";
  error.value = "";
  submitting.value = false;
  advancedOpen.value = false;
  isNsfw.value = false;
  refreshInterval.value = "0";
  addProgress.value = { current: 0, total: 0 };
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

watch(parsedUrls, (urls) => {
  if (!advancedOpen.value) {
    isNsfw.value = urls.some((u) => suggestNsfwFromUrl(u));
  }
});

function toggleAdvanced() {
  advancedOpen.value = !advancedOpen.value;
  if (advancedOpen.value && parsedUrls.value.some((u) => suggestNsfwFromUrl(u))) {
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
  const urls = parsedUrls.value;
  if (urls.length === 0) {
    error.value = t("settings.feeds.addEmpty");
    return;
  }

  submitting.value = true;
  addProgress.value = { current: 0, total: urls.length };
  const folderId = addFeedTargetFolderId.value;
  const interval = Number(refreshInterval.value) || 0;
  try {
    if (urls.length === 1) {
      await addFeedFromURL(urls[0]!, {
        title: title.value.trim() || undefined,
        isNsfw: isNsfw.value,
        refreshIntervalMinutes: interval,
        folderId: folderId ?? "",
      });
      toast.success(t("settings.feeds.addDone", { n: 1 }));
      return;
    }

    const result = await addFeedsFromURLs(urls, {
      folderId: folderId ?? "",
      isNsfw: isNsfw.value,
      refreshIntervalMinutes: interval,
      selectLast: true,
      onProgress: (current, total) => {
        addProgress.value = { current, total };
      },
    });
    closeAddFeed();
    if (result.added > 0 && result.failed.length === 0) {
      toast.success(t("settings.feeds.addDone", { n: result.added }));
    } else if (result.added > 0 && result.failed.length > 0) {
      toast.success(t("settings.feeds.addPartial", { ok: result.added, fail: result.failed.length }), {
        description: result.failed
          .slice(0, 3)
          .map((f) => `${f.url}: ${f.message}`)
          .join("\n"),
      });
    } else {
      toast.error(t("settings.feeds.addAllFailed"), {
        description: result.failed[0]?.message,
      });
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.feeds.addFailed"), { description: error.value });
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <Dialog :open="addFeedOpen" @update:open="(v) => !submitting && !v && closeAddFeed()">
    <!-- Above Settings (z-50) when opened from Settings → Feeds. -->
    <DialogContent class="z-[70] sm:max-w-lg" overlay-class="z-[70]">
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
        <div class="grid gap-1.5">
          <Label for="feed-urls">{{ t("settings.feeds.addUrlsLabel") }}</Label>
          <textarea
            id="feed-urls"
            v-model="urlsText"
            rows="6"
            class="border-input dark:bg-input/30 focus-visible:border-ring focus-visible:ring-ring/50 placeholder:text-muted-foreground w-full min-w-0 resize-y rounded-lg border bg-transparent px-2.5 py-2 font-mono text-[12.5px] leading-relaxed outline-none focus-visible:ring-3 disabled:cursor-not-allowed disabled:opacity-50"
            :placeholder="t('settings.feeds.addUrlsPlaceholder')"
            :disabled="submitting"
            spellcheck="false"
            autocomplete="off"
          />
          <p class="text-[11.5px] leading-snug text-muted-foreground">
            {{
              parsedUrls.length > 0
                ? t("settings.feeds.addUrlsCount", { n: parsedUrls.length })
                : t("settings.feeds.addUrlsHint")
            }}
          </p>
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
                v-if="isNsfw || Number(refreshInterval) > 0 || title.trim()"
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

            <div v-if="parsedUrls.length <= 1" class="grid gap-1.5">
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
                :disabled="submitting"
              />
            </div>

            <div class="flex items-start justify-between gap-3">
              <div class="grid gap-0.5 pr-2">
                <Label for="feed-nsfw" class="text-[13px] font-medium leading-none">
                  {{ t("feed.add.nsfw") }}
                </Label>
                <p class="text-[12px] leading-snug text-muted-foreground">
                  {{ t("feed.add.nsfwDesc") }}
                </p>
              </div>
              <Switch id="feed-nsfw" v-model:checked="isNsfw" class="mt-0.5" :disabled="submitting" />
            </div>

            <div class="grid gap-2">
              <Label class="text-[13px]">{{ t("feed.add.refreshInterval") }}</Label>
              <Select v-model="refreshInterval" :disabled="submitting">
                <SelectTrigger class="h-9 w-full text-[13px]">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent position="popper" class="z-[80] w-[var(--reka-select-trigger-width)]">
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

        <p
          v-if="submitting && addProgress.total > 1"
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
        <p v-if="error" class="text-[12.5px] text-destructive">{{ error }}</p>

        <DialogFooter class="gap-2 sm:gap-0">
          <Button type="button" variant="ghost" :disabled="submitting" @click="closeAddFeed">
            {{ t("common.cancel") }}
          </Button>
          <Button type="submit" :disabled="!canSubmit">
            {{
              submitting
                ? parsedUrls.length > 1
                  ? t("settings.feeds.adding")
                  : t("feed.add.submitting")
                : parsedUrls.length > 1
                  ? t("settings.feeds.addSubmitMany", { n: parsedUrls.length })
                  : t("feed.add.submit")
            }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
