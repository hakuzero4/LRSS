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
import { Switch } from "@/components/ui/switch";

const { settings } = useRssStore();

const providerModel = computed({
  get: () => settings.syncProvider,
  set: (v: typeof settings.syncProvider) => {
    settings.syncProvider = v;
  },
});
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup title="同步" description="跨设备同步订阅与阅读状态（设计稿）。">
      <div class="py-2.5">
        <SettingsRow title="启用同步" description="在设备之间同步订阅列表与已读状态。">
          <Switch
            :checked="settings.syncEnabled"
            @update:checked="(v: boolean) => (settings.syncEnabled = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5" :class="!settings.syncEnabled && 'opacity-50'">
        <SettingsRow title="同步提供方" description="选择同步后端。">
          <Select v-model="providerModel" :disabled="!settings.syncEnabled">
            <SelectTrigger class="h-8 w-[150px] text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">未配置</SelectItem>
              <SelectItem value="icloud">iCloud</SelectItem>
              <SelectItem value="webdav">WebDAV</SelectItem>
              <SelectItem value="custom">自定义</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
