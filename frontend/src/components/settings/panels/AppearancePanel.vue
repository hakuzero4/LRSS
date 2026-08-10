<script setup lang="ts">
import { computed } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import { useLocale } from "@/composables/useLocale";
import type { AppLocale } from "@/i18n";
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { AppSettings } from "@/types/rss";

const { settings, persistUIPrefs } = useRssStore();
const { locale, setLocale, t } = useLocale();

const languageModel = computed({
  get: () => locale.value,
  set: (v: string) => setLocale(v as AppLocale),
});

const themeModel = computed({
  get: () => settings.theme,
  set: (v: string) => {
    settings.theme = v as AppSettings["theme"];
    persistUIPrefs();
  },
});

const accentModel = computed({
  get: () => settings.accent,
  set: (v: string) => {
    settings.accent = v as AppSettings["accent"];
    persistUIPrefs();
  },
});

function onCompactSidebar(v: boolean) {
  settings.compactSidebar = v;
  persistUIPrefs();
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.appearance.themeGroup')"
      :description="t('settings.appearance.themeGroupDesc')"
    >
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">{{ t("settings.appearance.appearance") }}</p>
        <Tabs v-model="themeModel" class="w-full">
          <TabsList class="grid w-full grid-cols-3">
            <TabsTrigger value="system">{{ t("settings.appearance.themeSystem") }}</TabsTrigger>
            <TabsTrigger value="light">{{ t("settings.appearance.themeLight") }}</TabsTrigger>
            <TabsTrigger value="dark">{{ t("settings.appearance.themeDark") }}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">{{ t("settings.appearance.accent") }}</p>
        <Tabs v-model="accentModel" class="w-full">
          <TabsList class="grid w-full grid-cols-4">
            <TabsTrigger value="purple">{{ t("settings.appearance.accentPurple") }}</TabsTrigger>
            <TabsTrigger value="blue">{{ t("settings.appearance.accentBlue") }}</TabsTrigger>
            <TabsTrigger value="teal">{{ t("settings.appearance.accentTeal") }}</TabsTrigger>
            <TabsTrigger value="orange">{{ t("settings.appearance.accentOrange") }}</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.appearance.compactSidebar')"
          :description="t('settings.appearance.compactSidebarDesc')"
        >
          <Switch
            :checked="settings.compactSidebar"
            @update:checked="onCompactSidebar"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.appearance.language')"
          :description="t('settings.appearance.languageDesc')"
        >
          <Select v-model="languageModel">
            <SelectTrigger class="h-8 w-[150px] text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="zh-CN">{{ t("settings.appearance.langZhCN") }}</SelectItem>
              <SelectItem value="en-US">{{ t("settings.appearance.langEnUS") }}</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
