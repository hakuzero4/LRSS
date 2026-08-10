<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

function onHideDuplicates(v: boolean) {
  settings.hideDuplicateTitles = v;
  persistUIPrefs();
}

function onBlockKeywords(v: string | number) {
  settings.blockKeywords = String(v ?? "");
  persistUIPrefs();
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.filters.group')"
      :description="t('settings.filters.groupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.filters.hideDuplicates')"
          :description="t('settings.filters.hideDuplicatesDesc')"
        >
          <Switch
            :checked="settings.hideDuplicateTitles"
            @update:checked="onHideDuplicates"
          />
        </SettingsRow>
      </div>
      <div class="space-y-2 py-3">
        <p class="text-[13px] font-medium">{{ t("settings.filters.blockKeywords") }}</p>
        <p class="text-[12px] text-muted-foreground">
          {{ t("settings.filters.blockKeywordsDesc") }}
        </p>
        <Input
          :model-value="settings.blockKeywords"
          :placeholder="t('settings.filters.blockKeywordsPlaceholder')"
          class="h-9 text-[13px]"
          @update:model-value="onBlockKeywords"
        />
      </div>
    </SettingsGroup>
  </div>
</template>
