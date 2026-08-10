<script setup lang="ts">
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";

const { settings } = useRssStore();
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup title="过滤规则" description="减少噪音，保留你关心的内容。">
      <div class="py-2.5">
        <SettingsRow
          title="隐藏重复标题"
          description="同一订阅或跨订阅的重复标题只显示一条。"
        >
          <Switch
            :checked="settings.hideDuplicateTitles"
            @update:checked="(v: boolean) => (settings.hideDuplicateTitles = v)"
          />
        </SettingsRow>
      </div>
      <div class="space-y-2 py-3">
        <p class="text-[13px] font-medium">屏蔽关键词</p>
        <p class="text-[12px] text-muted-foreground">
          用逗号分隔。标题或摘要命中后隐藏文章。
        </p>
        <Input
          v-model="settings.blockKeywords"
          placeholder="广告, 促销, sponsored"
          class="h-9 text-[13px]"
        />
      </div>
    </SettingsGroup>
  </div>
</template>
