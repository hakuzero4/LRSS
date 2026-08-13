<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

function patchBool(
  key: "autoSummarize" | "selectTranslate" | "autoFetchFull" | "smartBriefing",
  v: boolean,
) {
  settings[key] = v;
  persistUIPrefs();
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.aiFeatures.togglesGroup')"
      :description="t('settings.aiFeatures.togglesGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.aiFeatures.autoFetchFull')"
          :description="t('settings.aiFeatures.autoFetchFullDesc')"
        >
          <Switch
            :checked="settings.autoFetchFull"
            @update:checked="(v: boolean) => patchBool('autoFetchFull', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.searchAi.autoSummarize')"
          :description="t('settings.searchAi.autoSummarizeDesc')"
        >
          <Switch
            :checked="settings.autoSummarize"
            @update:checked="(v: boolean) => patchBool('autoSummarize', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.aiFeatures.smartBriefing')"
          :description="t('settings.aiFeatures.smartBriefingDesc')"
        >
          <Switch
            :checked="settings.smartBriefing"
            @update:checked="(v: boolean) => patchBool('smartBriefing', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.searchAi.selectTranslate')"
          :description="t('settings.searchAi.selectTranslateDesc')"
        >
          <Switch
            :checked="settings.selectTranslate"
            @update:checked="(v: boolean) => patchBool('selectTranslate', v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
