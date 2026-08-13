<script setup lang="ts">
import { computed } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import { useLocale } from "@/composables/useLocale";
import { useTheme } from "@/composables/useTheme";
import type { AppLocale } from "@/i18n";
import { normalizeAccent } from "@/lib/accent";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { ColorPicker } from "@/components/ui/color-picker";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { AppSettings, ReaderToolbarButtons } from "@/types/rss";
import { READER_TOOLBAR_KEYS } from "@/types/rss";

const { settings, persistUIPrefs, webMode } = useRssStore();
const { locale, setLocale, t } = useLocale();
const { isDark } = useTheme();

const micaSupported = computed(() => {
  if (webMode.value) return false;
  return /Windows/i.test(typeof navigator !== "undefined" ? navigator.userAgent : "");
});

const micaHint = computed(() => {
  if (!micaSupported.value) return t("settings.appearance.micaUnavailable");
  if (!settings.hardwareAcceleration) return t("settings.appearance.micaNeedsGpu");
  return t("settings.appearance.micaDesc");
});

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
    settings.accent = normalizeAccent(v, settings.accent) as AppSettings["accent"];
    persistUIPrefs(true);
  },
});

function onCompactSidebar(v: boolean) {
  settings.compactSidebar = v;
  persistUIPrefs();
}

function onMicaBackdrop(v: boolean) {
  settings.micaBackdrop = v;
  persistUIPrefs(true);
}

const toolbarRows = computed(() =>
  READER_TOOLBAR_KEYS.map((key) => ({
    key,
    title: t(`settings.appearance.toolbar.${key}`),
    description: t(`settings.appearance.toolbar.${key}Desc`),
  })),
);

function onToolbarToggle(key: keyof ReaderToolbarButtons, v: boolean) {
  settings.readerToolbar[key] = v;
  persistUIPrefs();
}

function showAllToolbar() {
  for (const key of READER_TOOLBAR_KEYS) {
    settings.readerToolbar[key] = true;
  }
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
        <div>
          <p class="text-[13px] font-medium">{{ t("settings.appearance.accent") }}</p>
          <p class="mt-0.5 text-[12px] text-muted-foreground">
            {{ t("settings.appearance.accentDesc") }}
          </p>
        </div>
        <ColorPicker v-model="accentModel" :is-dark="isDark" />
      </div>

      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.appearance.mica')"
          :description="micaHint"
        >
          <Switch
            :checked="settings.micaBackdrop && micaSupported && settings.hardwareAcceleration"
            :disabled="!micaSupported || !settings.hardwareAcceleration"
            @update:checked="onMicaBackdrop"
          />
        </SettingsRow>
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

    <SettingsGroup
      :title="t('settings.appearance.toolbarGroup')"
      :description="t('settings.appearance.toolbarGroupDesc')"
    >
      <div class="flex items-center justify-end py-1">
        <button
          type="button"
          class="text-[12px] font-medium text-primary hover:underline"
          @click="showAllToolbar"
        >
          {{ t("settings.appearance.toolbarShowAll") }}
        </button>
      </div>
      <div
        v-for="row in toolbarRows"
        :key="row.key"
        class="py-2.5"
      >
        <SettingsRow :title="row.title" :description="row.description">
          <Switch
            :checked="settings.readerToolbar[row.key]"
            @update:checked="(v: boolean) => onToolbarToggle(row.key, v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
