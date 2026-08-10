<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
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

const { t } = useI18n();
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
    <SettingsGroup
      :title="t('settings.sync.group')"
      :description="t('settings.sync.groupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.sync.enable')"
          :description="t('settings.sync.enableDesc')"
        >
          <Switch
            :checked="settings.syncEnabled"
            @update:checked="(v: boolean) => (settings.syncEnabled = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5" :class="!settings.syncEnabled && 'opacity-50'">
        <SettingsRow
          :title="t('settings.sync.provider')"
          :description="t('settings.sync.providerDesc')"
        >
          <Select v-model="providerModel" :disabled="!settings.syncEnabled">
            <SelectTrigger class="h-8 w-[150px] text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">{{ t("settings.sync.providerNone") }}</SelectItem>
              <SelectItem value="icloud">{{ t("settings.sync.providerICloud") }}</SelectItem>
              <SelectItem value="webdav">{{ t("settings.sync.providerWebDAV") }}</SelectItem>
              <SelectItem value="custom">{{ t("settings.sync.providerCustom") }}</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
