<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { loadAppsvc } from "@/lib/backend";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const {
  settings,
  persistUIPrefs,
  resetUIPrefsToDefaults,
  exportDiagnostics,
  backendReady,
} = useRssStore();

const cacheCount = ref<number | null>(null);
const clearing = ref(false);

async function refreshCacheCount() {
  if (!backendReady.value) {
    cacheCount.value = null;
    return;
  }
  try {
    const api = await loadAppsvc();
    const fn = api?.SettingsService?.LLMCacheCount;
    if (typeof fn !== "function") {
      cacheCount.value = null;
      return;
    }
    const n = await fn();
    cacheCount.value = typeof n === "number" ? n : Number(n ?? 0);
  } catch {
    cacheCount.value = null;
  }
}

function patchBool(
  key: "hardwareAcceleration" | "clearCacheOnQuit" | "developerMode",
  v: boolean,
) {
  settings[key] = v;
  persistUIPrefs();
  if (key === "hardwareAcceleration") {
    toast.message(t("settings.advanced.restartHint"));
  }
  if (key === "developerMode" && v) {
    toast.message(t("settings.advanced.developerOnHint"));
  }
}

async function onReset() {
  if (!window.confirm(t("settings.advanced.resetConfirm"))) return;
  try {
    await resetUIPrefsToDefaults();
    toast.success(t("settings.advanced.resetDone"));
    await refreshCacheCount();
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

async function onClearCache() {
  if (!backendReady.value) {
    toast.error(t("settings.advanced.clearCacheFailed"), {
      description: t("ai.backendUnavailable"),
    });
    return;
  }
  clearing.value = true;
  try {
    const api = await loadAppsvc();
    const fn = api?.SettingsService?.ClearLLMCache;
    if (typeof fn !== "function") {
      throw new Error(t("ai.unavailable"));
    }
    const raw = await fn();
    const deleted = Number(
      (raw as { deleted?: number; Deleted?: number })?.deleted ??
        (raw as { Deleted?: number })?.Deleted ??
        0,
    );
    toast.success(t("settings.advanced.clearCacheDone", { n: deleted }));
    await refreshCacheCount();
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.advanced.clearCacheFailed"), { description: msg });
  } finally {
    clearing.value = false;
  }
}

onMounted(() => {
  void refreshCacheCount();
});
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.advanced.group')"
      :description="t('settings.advanced.groupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.advanced.hardwareAccel')"
          :description="t('settings.advanced.hardwareAccelDesc')"
        >
          <Switch
            :checked="settings.hardwareAcceleration"
            @update:checked="(v: boolean) => patchBool('hardwareAcceleration', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.advanced.clearCacheOnQuit')"
          :description="t('settings.advanced.clearCacheOnQuitDesc')"
        >
          <Switch
            :checked="settings.clearCacheOnQuit"
            @update:checked="(v: boolean) => patchBool('clearCacheOnQuit', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.advanced.developerMode')"
          :description="t('settings.advanced.developerModeDesc')"
        >
          <Switch
            :checked="settings.developerMode"
            @update:checked="(v: boolean) => patchBool('developerMode', v)"
          />
        </SettingsRow>
      </div>

      <div
        v-if="cacheCount !== null"
        class="py-1 text-[12px] text-muted-foreground"
      >
        {{ t("settings.advanced.cacheCount", { n: cacheCount }) }}
      </div>

      <div class="flex flex-wrap gap-2 py-3">
        <Button
          variant="outline"
          size="sm"
          type="button"
          :disabled="clearing || !backendReady"
          @click="onClearCache"
        >
          {{
            clearing
              ? t("settings.advanced.clearingCache")
              : t("settings.advanced.clearCacheNow")
          }}
        </Button>
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
