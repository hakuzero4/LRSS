<script setup lang="ts">
import { computed } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";

const { settings, folders } = useRssStore();

const keepDaysModel = computed({
  get: () => [settings.keepArticlesDays],
  set: (v: number[]) => {
    settings.keepArticlesDays = v[0] ?? 90;
  },
});

const defaultFolderModel = computed({
  get: () => settings.defaultFolderId ?? "none",
  set: (v: string) => {
    settings.defaultFolderId = v === "none" ? null : v;
  },
});
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup title="订阅" description="新订阅与文章保留策略。">
      <div class="py-2.5">
        <SettingsRow title="默认文件夹" description="添加订阅时默认归入的文件夹。">
          <Select v-model="defaultFolderModel">
            <SelectTrigger class="h-8 w-[150px] text-[13px]">
              <SelectValue placeholder="未分组" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">未分组</SelectItem>
              <SelectItem v-for="f in folders" :key="f.id" :value="f.id">
                {{ f.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          title="抓取全文"
          description="在支持时尝试提取完整正文（可能更慢）。"
        >
          <Switch
            :checked="settings.fetchFullContent"
            @update:checked="(v: boolean) => (settings.fetchFullContent = v)"
          />
        </SettingsRow>
      </div>
      <div class="space-y-3 py-3">
        <div class="flex items-end justify-between gap-3">
          <div>
            <p class="text-[13px] font-medium">文章保留天数</p>
            <p class="mt-0.5 text-[12px] text-muted-foreground">超过后清理本地已读文章。</p>
          </div>
          <span class="tabular-nums text-[12.5px] font-medium">
            {{ settings.keepArticlesDays }} 天
          </span>
        </div>
        <Slider v-model="keepDaysModel" :min="7" :max="365" :step="1" class="w-full" />
      </div>
    </SettingsGroup>
  </div>
</template>
