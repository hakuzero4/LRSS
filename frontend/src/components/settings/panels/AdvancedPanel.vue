<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

function patchBool(
  key: "hardwareAcceleration" | "clearCacheOnQuit" | "developerMode",
  v: boolean,
) {
  settings[key] = v;
  persistUIPrefs();
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.advanced.group')"
      :description="t('settings.advanced.groupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.advanced.hardwareAccel')"
          :description="t('settings.advanced.hardwareAccelDesc')"
        >
          <Switch
            :checked="settings.hardwareAcceleration"
            @update:checked="(v: boolean) => patchBool('hardwareAcceleration', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.advanced.clearCacheOnQuit')"
          :description="t('settings.advanced.clearCacheOnQuitDesc')"
        >
          <Switch
            :checked="settings.clearCacheOnQuit"
            @update:checked="(v: boolean) => patchBool('clearCacheOnQuit', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.advanced.developerMode')"
          :description="t('settings.advanced.developerModeDesc')"
        >
          <Switch
            :checked="settings.developerMode"
            @update:checked="(v: boolean) => patchBool('developerMode', v)"
          />
        </SettingsRow>
      </div>
      <div class="flex flex-wrap gap-2 py-3">
        <Button variant="outline" size="sm" type="button">
          {{ t("settings.advanced.exportLogs") }}
        </Button>
        <Button variant="outline" size="sm" type="button">
          {{ t("settings.advanced.resetSettings") }}
        </Button>
      </div>
    </SettingsGroup>
  </div>
</template>
