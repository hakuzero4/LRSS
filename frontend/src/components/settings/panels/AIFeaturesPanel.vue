<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

function patchBool(
  key: "autoSummarize" | "selectTranslate" | "autoFetchFull",
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
          :title="t('settings.searchAi.selectTranslate')"
          :description="t('settings.searchAi.selectTranslateDesc')"
        >
          <Switch
            :checked="settings.selectTranslate"
            @update:checked="(v: boolean) => patchBool('selectTranslate', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5 text-[12.5px] leading-relaxed text-muted-foreground">
        <p>{{ t("settings.searchAi.translateKeepBoth") }}</p>
        <p class="mt-1.5">{{ t("settings.aiFeatures.needModelHint") }}</p>
      </div>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.aiFeatures.catalogGroup')"
      :description="t('settings.aiFeatures.catalogGroupDesc')"
    >
      <div class="space-y-2.5 py-3 text-[12.5px] leading-relaxed text-muted-foreground">
        <p>{{ t("settings.aiFeatures.llmFeatureAutoFetchFull") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureSummarize") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureSelectTranslate") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureTranslate") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureAsk") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureDigest") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureSuggest") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureClassify") }}</p>
      </div>
    </SettingsGroup>
  </div>
</template>
