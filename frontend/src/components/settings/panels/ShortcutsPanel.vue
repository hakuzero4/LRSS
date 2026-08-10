<script setup lang="ts">
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";

const { settings } = useRssStore();

const shortcuts = [
  { keys: "j / k", action: "下一条 / 上一条" },
  { keys: "o 或 Enter", action: "打开 / 聚焦阅读区" },
  { keys: "r", action: "刷新订阅" },
  { keys: "s", action: "收藏 / 取消收藏" },
  { keys: "m", action: "切换已读" },
  { keys: "⌘ ,", action: "打开设置" },
];
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup title="快捷键" description="键盘优先的阅读节奏。">
      <div class="py-2.5">
        <SettingsRow title="启用键盘快捷键" description="关闭后仅保留系统级快捷键。">
          <Switch
            :checked="settings.enableKeyboardShortcuts"
            @update:checked="(v: boolean) => (settings.enableKeyboardShortcuts = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2">
        <ul class="divide-y divide-border/60">
          <li
            v-for="item in shortcuts"
            :key="item.keys"
            class="flex items-center justify-between gap-3 py-2.5 text-[13px]"
          >
            <span class="text-muted-foreground">{{ item.action }}</span>
            <kbd
              class="rounded-md border border-border bg-muted/50 px-2 py-0.5 font-mono text-[11.5px] text-foreground/80"
            >
              {{ item.keys }}
            </kbd>
          </li>
        </ul>
      </div>
    </SettingsGroup>
  </div>
</template>
