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

const { t } = useI18n();
const { addFeedOpen, closeAddFeed, addFeedFromURL } = useRssStore();

const feedUrl = ref("");
const title = ref("");
const submitting = ref(false);
const error = ref("");

const canSubmit = computed(() => feedUrl.value.trim().length > 8 && !submitting.value);

watch(addFeedOpen, (open) => {
  if (!open) {
    feedUrl.value = "";
    title.value = "";
    error.value = "";
    submitting.value = false;
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
          {{ t("feed.add.description") }}
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
