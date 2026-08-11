<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { loadAppsvc } from "@/lib/backend";
import { readerFontFamilyCSS } from "@/lib/readingSettings";
import type { AppSettings } from "@/types/rss";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

/** "" = system / app default. */
const SYSTEM_VALUE = "";

const fontList = ref<string[]>([]);
const fontsLoading = ref(true);
const fontFilter = ref("");

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

const fontFamilyModel = computed({
  get: () => settings.readerFontFamily || SYSTEM_VALUE,
  set: (v: string) => {
    const next = String(v ?? "").trim();
    settings.readerFontFamily =
      !next || next.toLowerCase() === "system" || next.toLowerCase() === "default"
        ? ""
        : next.slice(0, 80);
    persistUIPrefs();
  },
});

const filteredFonts = computed(() => {
  const q = fontFilter.value.trim().toLowerCase();
  if (!q) return fontList.value;
  return fontList.value.filter((f) => f.toLowerCase().includes(q));
});

const previewStyle = computed(() => {
  const css = readerFontFamilyCSS(settings.readerFontFamily);
  if (!css) return undefined;
  return { fontFamily: css };
});

function patchBool(key: "showUnreadOnly" | "openLinksInBrowser", v: boolean) {
  settings[key] = v;
  persistUIPrefs();
}

function mergeFonts(...lists: (string[] | null | undefined)[]): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const list of lists) {
    if (!list) continue;
    for (const raw of list) {
      const n = String(raw ?? "").trim();
      if (!n || n.length > 80) continue;
      if (/[/\\<>|{};]/.test(n)) continue;
      const key = n.toLowerCase();
      if (seen.has(key)) continue;
      seen.add(key);
      out.push(n);
    }
  }
  out.sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" }));
  return out;
}

/** Optional Chromium Local Font Access API (WebView2 may support). */
async function queryBrowserLocalFonts(): Promise<string[]> {
  try {
    const q = (window as unknown as { queryLocalFonts?: () => Promise<{ family: string }[]> })
      .queryLocalFonts;
    if (typeof q !== "function") return [];
    const data = await q();
    return (data ?? []).map((d) => d.family).filter(Boolean);
  } catch {
    return [];
  }
}

async function loadSystemFonts() {
  fontsLoading.value = true;
  try {
    let backend: string[] = [];
    try {
      const api = await loadAppsvc();
      const fn = api?.SettingsService?.ListSystemFonts;
      if (typeof fn === "function") {
        const res = await fn();
        if (Array.isArray(res)) backend = res.map(String);
      }
    } catch {
      /* browser-only / unbound */
    }
    const local = await queryBrowserLocalFonts();
    fontList.value = mergeFonts(backend, local);
    // Keep current selection visible even if missing from enum (hand-edited prefs).
    const cur = settings.readerFontFamily?.trim();
    if (cur && !fontList.value.some((f) => f.toLowerCase() === cur.toLowerCase())) {
      fontList.value = mergeFonts(fontList.value, [cur]);
    }
  } finally {
    fontsLoading.value = false;
  }
}

onMounted(() => {
  void loadSystemFonts();
});
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
        <div class="flex items-baseline justify-between gap-2">
          <p class="text-[13px] font-medium">{{ t("settings.reading.fontFamily") }}</p>
          <span
            v-if="!fontsLoading && fontList.length"
            class="text-[11px] tabular-nums text-muted-foreground"
          >
            {{ t("settings.reading.fontFamilyCount", { n: fontList.length }) }}
          </span>
        </div>
        <p class="text-[12px] leading-relaxed text-muted-foreground">
          {{ t("settings.reading.fontFamilyDesc") }}
        </p>
        <input
          v-model="fontFilter"
          type="search"
          class="border-input dark:bg-input/30 focus-visible:border-ring focus-visible:ring-ring/50 placeholder:text-muted-foreground h-8 w-full rounded-lg border bg-transparent px-2.5 text-[12.5px] outline-none focus-visible:ring-3"
          :placeholder="
            fontsLoading
              ? t('settings.reading.fontFamilyLoading')
              : t('settings.reading.fontFamilyFilter')
          "
          :disabled="fontsLoading"
          autocomplete="off"
        />
        <select
          class="border-input dark:bg-input/30 focus-visible:border-ring focus-visible:ring-ring/50 h-9 w-full rounded-lg border bg-transparent px-2.5 text-[13px] outline-none focus-visible:ring-3 disabled:opacity-50"
          :value="fontFamilyModel"
          :disabled="fontsLoading"
          :aria-label="t('settings.reading.fontFamily')"
          @change="
            fontFamilyModel = ($event.target as HTMLSelectElement).value
          "
        >
          <option :value="SYSTEM_VALUE">
            {{ t("settings.reading.fontFamilySystem") }}
          </option>
          <option
            v-for="name in filteredFonts"
            :key="name"
            :value="name"
            :style="{ fontFamily: `'${name.replace(/'/g, '')}'` }"
          >
            {{ name }}
          </option>
        </select>
        <p
          class="rounded-lg border border-border/60 bg-muted/30 px-3 py-2.5 text-[13.5px] leading-relaxed text-foreground/90"
          :style="previewStyle"
        >
          {{ t("settings.reading.fontFamilyPreview") }}
        </p>
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
