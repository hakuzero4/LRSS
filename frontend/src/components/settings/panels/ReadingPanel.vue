<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { AppSettings } from "@/types/rss";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

const fontSizeModel = computed({
  get: () => settings.fontSize,
  set: (v: string) => {
    settings.fontSize = v as AppSettings["fontSize"];
    persistUIPrefs();
  },
});

const readerWidthModel = computed({
  get: () => settings.readerWidth,
  set: (v: string) => {
    settings.readerWidth = v as AppSettings["readerWidth"];
    persistUIPrefs();
  },
});

function patchBool(key: "showUnreadOnly" | "openLinksInBrowser", v: boolean) {
  settings[key] = v;
  persistUIPrefs();
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.reading.group')"
      :description="t('settings.reading.groupDesc')"
    >
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">{{ t("settings.reading.fontSize") }}</p>
        <Tabs v-model="fontSizeModel" class="w-full">
          <TabsList class="grid w-full grid-cols-3">
            <TabsTrigger value="sm">{{ t("settings.reading.fontSm") }}</TabsTrigger>
            <TabsTrigger value="md">{{ t("settings.reading.fontMd") }}</TabsTrigger>
            <TabsTrigger value="lg">{{ t("settings.reading.fontLg") }}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">{{ t("settings.reading.width") }}</p>
        <Tabs v-model="readerWidthModel" class="w-full">
          <TabsList class="grid w-full grid-cols-4">
            <TabsTrigger value="narrow">{{ t("settings.reading.widthNarrow") }}</TabsTrigger>
            <TabsTrigger value="medium">{{ t("settings.reading.widthMedium") }}</TabsTrigger>
            <TabsTrigger value="wide">{{ t("settings.reading.widthWide") }}</TabsTrigger>
            <TabsTrigger value="fill">{{ t("settings.reading.widthFill") }}</TabsTrigger>
          </TabsList>
        </Tabs>
        <p class="text-[12px] leading-relaxed text-muted-foreground">
          {{ t("settings.reading.widthFillHint") }}
        </p>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.reading.unreadOnly')"
          :description="t('settings.reading.unreadOnlyDesc')"
        >
          <Switch
            :checked="settings.showUnreadOnly"
            @update:checked="(v: boolean) => patchBool('showUnreadOnly', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.reading.openInBrowser')"
          :description="t('settings.reading.openInBrowserDesc')"
        >
          <Switch
            :checked="settings.openLinksInBrowser"
            @update:checked="(v: boolean) => patchBool('openLinksInBrowser', v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
