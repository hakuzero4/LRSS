<script setup lang="ts">
import { computed, ref, watch } from "vue";
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
    error.value = "请粘贴订阅源 URL。";
    return;
  }
  try {
    // eslint-disable-next-line no-new
    new URL(url);
  } catch {
    error.value = "URL 格式不正确。";
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
        <DialogTitle>添加订阅</DialogTitle>
        <DialogDescription>
          粘贴 RSS / Atom 地址。可选标题；留空则使用拉取到的源标题。
        </DialogDescription>
      </DialogHeader>

      <form class="grid gap-4 py-1" @submit.prevent="onSubmit">
        <div class="grid gap-2">
          <Label for="feed-url">Feed URL</Label>
          <Input
            id="feed-url"
            v-model="feedUrl"
            type="url"
            placeholder="https://example.com/feed.xml"
            autocomplete="off"
            autofocus
          />
        </div>
        <div class="grid gap-2">
          <Label for="feed-title">
            标题
            <span class="font-normal text-muted-foreground">（可选）</span>
          </Label>
          <Input
            id="feed-title"
            v-model="title"
            type="text"
            placeholder="我的博客"
            autocomplete="off"
          />
        </div>
        <p v-if="error" class="text-[12.5px] text-destructive">{{ error }}</p>

        <DialogFooter class="gap-2 sm:gap-0">
          <Button type="button" variant="ghost" @click="closeAddFeed">取消</Button>
          <Button type="submit" :disabled="!canSubmit">
            {{ submitting ? "拉取中…" : "添加并刷新" }}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  </Dialog>
</template>
