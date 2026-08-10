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
import type { SmartCollectionId } from "@/types/rss";

const { settings } = useRssStore();

const intervalLabel = computed(() => {
  const m = settings.refreshIntervalMinutes;
  if (m < 60) return `${m} 分钟`;
  if (m === 60) return "1 小时";
  if (m % 60 === 0) return `${m / 60} 小时`;
  return `${m} 分钟`;
});

const intervalModel = computed({
  get: () => [settings.refreshIntervalMinutes],
  set: (v: number[]) => {
    settings.refreshIntervalMinutes = v[0] ?? 30;
  },
});

const openOnStartupModel = computed({
  get: () => settings.openOnStartup,
  set: (v: SmartCollectionId) => {
    settings.openOnStartup = v;
  },
});
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      title="刷新"
      description="后台检查订阅源更新的方式。"
    >
      <div class="py-2.5">
        <SettingsRow
          title="自动刷新"
          description="开启后定期在后台检查订阅源更新。"
        >
          <Switch
            :checked="settings.autoRefresh"
            @update:checked="(v: boolean) => (settings.autoRefresh = v)"
          />
        </SettingsRow>
      </div>

      <div class="space-y-3 py-3" :class="!settings.autoRefresh && 'opacity-50'">
        <div class="flex items-end justify-between gap-3">
          <div>
            <p class="text-[13px] font-medium">刷新间隔</p>
            <p class="mt-0.5 text-[12px] text-muted-foreground">
              拖动滑块调整检查频率。
            </p>
          </div>
          <span class="tabular-nums text-[12.5px] font-medium text-foreground/80">
            {{ intervalLabel }}
          </span>
        </div>
        <Slider
          v-model="intervalModel"
          :min="5"
          :max="180"
          :step="5"
          :disabled="!settings.autoRefresh"
          class="w-full"
          :aria-label="`刷新间隔，当前 ${intervalLabel}`"
        />
        <div class="flex justify-between text-[11px] text-muted-foreground">
          <span>5 分钟</span>
          <span>3 小时</span>
        </div>
      </div>
    </SettingsGroup>

    <SettingsGroup title="已读行为" description="控制文章何时离开未读列表。">
      <div class="py-2.5">
        <SettingsRow
          title="打开文章时标为已读"
          description="在阅读区打开文章后，自动清除未读状态。"
        >
          <Switch
            :checked="settings.markAsReadOnOpen"
            @update:checked="(v: boolean) => (settings.markAsReadOnOpen = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          title="滚动到底部时标为已读"
          description="阅读到文章末尾时再标记为已读。"
        >
          <Switch
            :checked="settings.markAsReadOnScrollEnd"
            @update:checked="(v: boolean) => (settings.markAsReadOnScrollEnd = v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>

    <SettingsGroup title="启动" description="应用启动时的默认行为。">
      <div class="py-2.5">
        <SettingsRow
          title="开机自启动"
          description="登录系统后自动启动 LRSS。"
        >
          <Switch
            :checked="settings.launchAtLogin"
            @update:checked="(v: boolean) => (settings.launchAtLogin = v)"
          />
        </SettingsRow>
      </div>

      <div class="py-2.5">
        <SettingsRow title="启动时打开" description="打开应用后默认进入的列表。">
          <Select v-model="openOnStartupModel">
            <SelectTrigger class="h-8 w-[140px] text-[13px]">
              <SelectValue placeholder="选择列表" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="unread">未读</SelectItem>
              <SelectItem value="today">今日</SelectItem>
              <SelectItem value="starred">收藏</SelectItem>
              <SelectItem value="all">全部文章</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>

      <div class="py-2.5">
        <SettingsRow
          title="启动时隐藏已读"
          description="启动后默认只显示未读文章。"
        >
          <Switch
            :checked="settings.hideReadOnStartup"
            @update:checked="(v: boolean) => (settings.hideReadOnStartup = v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
