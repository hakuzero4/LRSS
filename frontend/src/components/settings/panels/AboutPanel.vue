<script setup lang="ts">
import { Sparkles } from "@lucide/vue";
import { computed } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import { Button } from "@/components/ui/button";

const { feeds, articles, smartCounts } = useRssStore();

const summary = computed(
  () =>
    `${feeds.value.length} 个订阅 · ${articles.value.length} 篇文章 · ${smartCounts.value.unread} 未读`,
);
</script>

<template>
  <div class="space-y-7">
    <div class="flex items-start gap-4">
      <div class="brand-mark size-14 shrink-0 rounded-2xl !w-14 !h-14">
        <Sparkles class="!size-7" />
      </div>
      <div class="min-w-0 pt-0.5">
        <h3 class="text-[17px] font-semibold tracking-tight">LRSS</h3>
        <p class="mt-0.5 text-[12.5px] text-muted-foreground">版本 0.1.0 · 设计预览</p>
        <p class="mt-2 text-[13px] leading-relaxed text-muted-foreground">
          本地优先的桌面 RSS 阅读器。Wails v3 · Vue 3 · shadcn-vue。
        </p>
        <p class="mt-1 text-[12px] text-muted-foreground">{{ summary }}</p>
      </div>
    </div>

    <SettingsGroup title="资源">
      <div class="flex flex-wrap gap-2 py-3">
        <Button variant="outline" size="sm" type="button">使用说明</Button>
        <Button variant="outline" size="sm" type="button">检查更新</Button>
        <Button variant="outline" size="sm" type="button">开源许可</Button>
      </div>
    </SettingsGroup>
  </div>
</template>
