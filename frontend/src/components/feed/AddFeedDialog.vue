<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
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

const { t } = useI18n();
const {
  addFeedOpen,
  addFeedTargetFolderId,
  folders,
  settings,
  closeAddFeed,
  setAddFeedFolderId,
  addFeedFromURL,
} = useRssStore();

const feedUrl = ref("");
const title = ref("");
const submitting = ref(false);
const error = ref("");

const canSubmit = computed(() => feedUrl.value.trim().length > 8 && !submitting.value);

const folderModel = computed({
  get: () => addFeedTargetFolderId.value ?? "none",
  set: (v: string) => setAddFeedFolderId(v === "none" ? null : v),
});

const targetFolderName = computed(() => {
  const id = addFeedTargetFolderId.value;
  if (!id) return "";
  return folders.value.find((f) => f.id === id)?.name ?? "";
});

watch(addFeedOpen, (open) => {
  if (!open) {
    feedUrl.value = "";
    title.value = "";
    error.value = "";
    submitting.value = false;
  } else if (!addFeedTargetFolderId.value && settings.defaultFolderId) {
    setAddFeedFolderId(settings.defaultFolderId);
  }
});

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
    await addFeedFromURL(url, title.value.trim() || undefined);
  } catch (e: any) {
    error.value = e?.message || String(e);
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
          <Label>{{ t("settings.feeds.defaultFolder") }}</Label>
          <Select v-model="folderModel">
            <SelectTrigger class="h-9 text-[13px]">
              <SelectValue :placeholder="t('settings.feeds.unfiled')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">{{ t("settings.feeds.unfiled") }}</SelectItem>
              <SelectItem v-for="f in folders" :key="f.id" :value="f.id">
                {{ f.name }}
              </SelectItem>
            </SelectContent>
          </Select>
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
