<script setup lang="ts">
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";

const { settings } = useRssStore();
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup title="通知" description="有新文章时如何提醒你。">
      <div class="py-2.5">
        <SettingsRow title="新文章通知" description="后台刷新发现新内容时弹出系统通知。">
          <Switch
            :checked="settings.notifyOnNewArticles"
            @update:checked="(v: boolean) => (settings.notifyOnNewArticles = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5" :class="!settings.notifyOnNewArticles && 'opacity-50'">
        <SettingsRow title="通知声音" description="发送通知时播放提示音。">
          <Switch
            :checked="settings.notifySound"
            :disabled="!settings.notifyOnNewArticles"
            @update:checked="(v: boolean) => (settings.notifySound = v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
