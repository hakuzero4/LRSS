<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, resetUIPrefsToDefaults, exportDiagnostics } = useRssStore();

async function onReset() {
  if (!window.confirm(t("settings.advanced.resetConfirm"))) return;
  try {
    await resetUIPrefsToDefaults();
    toast.success(t("settings.advanced.resetDone"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.advanced.resetFailed"), { description: msg });
  }
}

async function onExportLogs() {
  try {
    await exportDiagnostics();
    toast.success(t("settings.advanced.exportDone"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.advanced.exportFailed"), { description: msg });
  }
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.advanced.group')"
      :description="t('settings.advanced.groupDesc')"
    >
      <p
        class="mb-2 rounded-md border border-border/70 bg-muted/40 px-3 py-2 text-[12px] leading-relaxed text-muted-foreground"
        role="status"
      >
        {{ t("settings.unavailable.advancedBanner") }}
      </p>
      <div class="py-2.5 opacity-60">
        <SettingsRow
          :title="t('settings.advanced.hardwareAccel')"
          :description="t('settings.advanced.hardwareAccelDesc')"
        >
          <Switch
            :checked="settings.hardwareAcceleration"
            disabled
            :aria-disabled="true"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5 opacity-60">
        <SettingsRow
          :title="t('settings.advanced.clearCacheOnQuit')"
          :description="t('settings.advanced.clearCacheOnQuitDesc')"
        >
          <Switch
            :checked="settings.clearCacheOnQuit"
            disabled
            :aria-disabled="true"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5 opacity-60">
        <SettingsRow
          :title="t('settings.advanced.developerMode')"
          :description="t('settings.advanced.developerModeDesc')"
        >
          <Switch
            :checked="settings.developerMode"
            disabled
            :aria-disabled="true"
          />
        </SettingsRow>
      </div>
      <p class="pb-2 text-[11.5px] text-muted-foreground">
        {{ t("settings.unavailable.notWiredNote") }}
      </p>
      <div class="flex flex-wrap gap-2 py-3">
        <Button variant="outline" size="sm" type="button" @click="onExportLogs">
          {{ t("settings.advanced.exportLogs") }}
        </Button>
        <Button variant="outline" size="sm" type="button" @click="onReset">
          {{ t("settings.advanced.resetSettings") }}
        </Button>
      </div>
    </SettingsGroup>
  </div>
</template>
