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
import { Slider } from "@/components/ui/slider";
import { Switch } from "@/components/ui/switch";
import type { SmartCollectionId } from "@/types/rss";

const { t } = useI18n();
const { settings, persistLibraryConfig, persistUIPrefs } = useRssStore();

const intervalLabel = computed(() => {
  const m = settings.refreshIntervalMinutes;
  if (m < 60) return t("common.minutes", { n: m });
  if (m === 60) return t("common.oneHour");
  if (m % 60 === 0) return t("common.hours", { n: m / 60 });
  return t("common.minutes", { n: m });
});

const intervalModel = computed({
  get: () => [settings.refreshIntervalMinutes],
  set: (v: number[]) => {
    settings.refreshIntervalMinutes = v[0] ?? 30;
    void persistLibraryConfig();
  },
});

const openOnStartupModel = computed({
  get: () => settings.openOnStartup,
  set: (v: SmartCollectionId) => {
    settings.openOnStartup = v;
    persistUIPrefs();
  },
});

function onAutoRefreshChange(v: boolean) {
  settings.autoRefresh = v;
  void persistLibraryConfig();
}

function patchBool(
  key:
    | "markAsReadOnOpen"
    | "markAsReadOnScrollEnd"
    | "launchAtLogin"
    | "hideReadOnStartup",
  v: boolean,
) {
  settings[key] = v;
  // launchAtLogin is UI-only (no system API / not in UIPrefs)
  if (key !== "launchAtLogin") persistUIPrefs();
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.general.refreshGroup')"
      :description="t('settings.general.refreshGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.general.autoRefresh')"
          :description="t('settings.general.autoRefreshDesc')"
        >
          <Switch
            :checked="settings.autoRefresh"
            @update:checked="onAutoRefreshChange"
          />
        </SettingsRow>
      </div>

      <div class="space-y-3 py-3" :class="!settings.autoRefresh && 'opacity-50'">
        <div class="flex items-end justify-between gap-3">
          <div>
            <p class="text-[13px] font-medium">{{ t("settings.general.interval") }}</p>
            <p class="mt-0.5 text-[12px] text-muted-foreground">
              {{ t("settings.general.intervalHint") }}
            </p>
          </div>
          <span class="tabular-nums text-[12.5px] font-medium text-foreground/80">
            {{ intervalLabel }}
          </span>
        </div>
        <Slider
          v-model="intervalModel"
          :min="5"
          :max="180"
          :step="5"
          :disabled="!settings.autoRefresh"
          class="w-full"
          :aria-label="t('settings.general.intervalAria', { label: intervalLabel })"
        />
        <div class="flex justify-between text-[11px] text-muted-foreground">
          <span>{{ t("settings.general.intervalMin") }}</span>
          <span>{{ t("settings.general.intervalMax") }}</span>
        </div>
      </div>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.general.readBehavior')"
      :description="t('settings.general.readBehaviorDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.general.markOnOpen')"
          :description="t('settings.general.markOnOpenDesc')"
        >
          <Switch
            :checked="settings.markAsReadOnOpen"
            @update:checked="(v: boolean) => patchBool('markAsReadOnOpen', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.general.markOnScroll')"
          :description="t('settings.general.markOnScrollDesc')"
        >
          <Switch
            :checked="settings.markAsReadOnScrollEnd"
            @update:checked="(v: boolean) => patchBool('markAsReadOnScrollEnd', v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.general.startup')"
      :description="t('settings.general.startupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.general.launchAtLogin')"
          :description="t('settings.general.launchAtLoginDesc')"
        >
          <Switch
            :checked="settings.launchAtLogin"
            @update:checked="(v: boolean) => patchBool('launchAtLogin', v)"
          />
        </SettingsRow>
      </div>

      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.general.openOnStartup')"
          :description="t('settings.general.openOnStartupDesc')"
        >
          <Select v-model="openOnStartupModel">
            <SelectTrigger class="h-8 w-[140px] text-[13px]">
              <SelectValue :placeholder="t('settings.general.openOnStartupPlaceholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="unread">{{ t("nav.unread") }}</SelectItem>
              <SelectItem value="today">{{ t("nav.today") }}</SelectItem>
              <SelectItem value="starred">{{ t("nav.starred") }}</SelectItem>
              <SelectItem value="all">{{ t("nav.all") }}</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>

      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.general.hideReadOnStartup')"
          :description="t('settings.general.hideReadOnStartupDesc')"
        >
          <Switch
            :checked="settings.hideReadOnStartup"
            @update:checked="(v: boolean) => patchBool('hideReadOnStartup', v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
