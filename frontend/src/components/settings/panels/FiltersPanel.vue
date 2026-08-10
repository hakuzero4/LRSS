<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import { parseBlockKeywords } from "@/lib/articleFilters";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

const keywordCount = computed(() => parseBlockKeywords(settings.blockKeywords).length);

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
        <div class="flex items-end justify-between gap-2">
          <div>
            <p class="text-[13px] font-medium">{{ t("settings.filters.blockKeywords") }}</p>
            <p class="mt-0.5 text-[12px] text-muted-foreground">
              {{ t("settings.filters.blockKeywordsDesc") }}
            </p>
          </div>
          <span
            v-if="keywordCount > 0"
            class="shrink-0 tabular-nums text-[11.5px] text-muted-foreground"
          >
            {{ t("settings.filters.keywordCount", { n: keywordCount }) }}
          </span>
        </div>
        <Input
          :model-value="settings.blockKeywords"
          :placeholder="t('settings.filters.blockKeywordsPlaceholder')"
          class="h-9 text-[13px]"
          @update:model-value="onBlockKeywords"
        />
        <p class="text-[11.5px] leading-relaxed text-muted-foreground">
          {{ t("settings.filters.applyHint") }}
        </p>
      </div>
    </SettingsGroup>
  </div>
</template>
