<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { loadAppsvc } from "@/lib/backend";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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

// —— Web access ——
type WebCfg = {
  enabled: boolean;
  bind: string;
  port: number;
  token: string;
};
type WebStatus = {
  running?: boolean;
  url?: string;
  lanUrl?: string;
  bind?: string;
  port?: number;
  error?: string;
  hasToken?: boolean;
};

const webCfg = ref<WebCfg>({
  enabled: false,
  bind: "localhost",
  port: 18765,
  token: "",
});
const webStatus = ref<WebStatus>({});
const webBusy = ref(false);
const webLoaded = ref(false);

const webAccessUrl = computed(() => {
  const s = webStatus.value;
  if (s.running && s.url) return s.url;
  return "";
});
const webLanUrl = computed(() => {
  const s = webStatus.value;
  if (s.running && s.lanUrl) return s.lanUrl;
  return "";
});

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

async function loadWebAccess() {
  if (!backendReady.value) return;
  try {
    const api = await loadAppsvc();
    const getCfg = api?.SettingsService?.GetWebAccessConfig;
    const getSt = api?.SettingsService?.GetWebAccessStatus;
    if (typeof getCfg === "function") {
      const raw = await getCfg();
      if (raw && typeof raw === "object") {
        webCfg.value = {
          enabled: !!(raw.enabled ?? raw.Enabled),
          bind: String(raw.bind ?? raw.Bind ?? "localhost"),
          port: Number(raw.port ?? raw.Port ?? 18765) || 18765,
          token: String(raw.token ?? raw.Token ?? ""),
        };
      }
    }
    if (typeof getSt === "function") {
      const st = await getSt();
      if (st && typeof st === "object") {
        webStatus.value = {
          running: !!(st.running ?? st.Running),
          url: String(st.url ?? st.URL ?? ""),
          lanUrl: String(st.lanUrl ?? st.LanURL ?? st.LanUrl ?? ""),
          bind: String(st.bind ?? st.Bind ?? ""),
          port: Number(st.port ?? st.Port ?? 0) || 0,
          error: String(st.error ?? st.Error ?? ""),
          hasToken: !!(st.hasToken ?? st.HasToken),
        };
      }
    }
    webLoaded.value = true;
  } catch (e) {
    console.warn("[lrss] loadWebAccess", e);
  }
}

async function applyWebAccess(partial?: Partial<WebCfg>) {
  if (!backendReady.value) {
    toast.error(t("settings.advanced.webFailed"), {
      description: t("ai.backendUnavailable"),
    });
    return;
  }
  webBusy.value = true;
  try {
    const next: WebCfg = { ...webCfg.value, ...partial };
    next.port = Math.max(1024, Math.min(65535, Math.floor(Number(next.port) || 18765)));
    if (next.bind !== "lan") next.bind = "localhost";
    webCfg.value = next;
    const api = await loadAppsvc();
    const fn = api?.SettingsService?.SetWebAccessConfig;
    if (typeof fn !== "function") {
      throw new Error(t("ai.unavailable"));
    }
    const st = await fn({
      enabled: next.enabled,
      bind: next.bind,
      port: next.port,
      token: next.token,
    });
    if (st && typeof st === "object") {
      webStatus.value = {
        running: !!(st.running ?? st.Running),
        url: String(st.url ?? st.URL ?? ""),
        lanUrl: String(st.lanUrl ?? st.LanURL ?? st.LanUrl ?? ""),
        bind: String(st.bind ?? st.Bind ?? next.bind),
        port: Number(st.port ?? st.Port ?? next.port) || next.port,
        error: String(st.error ?? st.Error ?? ""),
        hasToken: !!(st.hasToken ?? st.HasToken),
      };
      // Refresh token if server generated one
      const cfg2 = await api?.SettingsService?.GetWebAccessConfig?.();
      if (cfg2) {
        webCfg.value.token = String(cfg2.token ?? cfg2.Token ?? webCfg.value.token);
      }
    }
    if (webStatus.value.error) {
      toast.error(t("settings.advanced.webFailed"), {
        description: webStatus.value.error,
      });
    } else if (next.enabled && webStatus.value.running) {
      toast.success(t("settings.advanced.webOn"));
    } else if (!next.enabled) {
      toast.success(t("settings.advanced.webOff"));
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.advanced.webFailed"), { description: msg });
  } finally {
    webBusy.value = false;
  }
}

async function onRegenToken() {
  if (!backendReady.value) return;
  webBusy.value = true;
  try {
    const api = await loadAppsvc();
    const fn = api?.SettingsService?.RegenerateWebAccessToken;
    if (typeof fn !== "function") throw new Error(t("ai.unavailable"));
    const cfg = await fn();
    if (cfg) {
      webCfg.value.token = String(cfg.token ?? cfg.Token ?? "");
    }
    await loadWebAccess();
    toast.success(t("settings.advanced.webTokenRegen"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.advanced.webFailed"), { description: msg });
  } finally {
    webBusy.value = false;
  }
}

async function copyText(text: string) {
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    toast.success(t("settings.advanced.webCopied"));
  } catch {
    toast.error(t("settings.advanced.webCopyFailed"));
  }
}

onMounted(() => {
  void refreshCacheCount();
  void loadWebAccess();
});
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.advanced.webGroup')"
      :description="t('settings.advanced.webGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.advanced.webEnable')"
          :description="t('settings.advanced.webEnableDesc')"
        >
          <Switch
            :checked="webCfg.enabled"
            :disabled="webBusy || !backendReady"
            @update:checked="(v: boolean) => applyWebAccess({ enabled: v })"
          />
        </SettingsRow>
      </div>

      <div class="space-y-3 border-t border-border/60 py-3">
        <p class="text-[12px] font-medium text-foreground">
          {{ t("settings.advanced.webBind") }}
        </p>
        <div class="flex flex-wrap gap-2">
          <Button
            type="button"
            size="sm"
            :variant="webCfg.bind === 'localhost' ? 'default' : 'outline'"
            :disabled="webBusy || !backendReady"
            @click="applyWebAccess({ bind: 'localhost' })"
          >
            {{ t("settings.advanced.webBindLocal") }}
          </Button>
          <Button
            type="button"
            size="sm"
            :variant="webCfg.bind === 'lan' ? 'default' : 'outline'"
            :disabled="webBusy || !backendReady"
            @click="applyWebAccess({ bind: 'lan' })"
          >
            {{ t("settings.advanced.webBindLan") }}
          </Button>
        </div>
        <p class="text-[11.5px] leading-relaxed text-muted-foreground">
          {{ t("settings.advanced.webBindHint") }}
        </p>
      </div>

      <div class="space-y-2 border-t border-border/60 py-3">
        <SettingsRow
          :title="t('settings.advanced.webPort')"
          :description="t('settings.advanced.webPortDesc')"
        >
          <Input
            type="number"
            class="w-28"
            min="1024"
            max="65535"
            :model-value="webCfg.port"
            :disabled="webBusy || !backendReady"
            @update:model-value="
              (v: string | number) => {
                const n = Number(v);
                if (Number.isFinite(n) && n !== webCfg.port) {
                  void applyWebAccess({ port: n });
                }
              }
            "
          />
        </SettingsRow>
      </div>

      <div class="space-y-2 border-t border-border/60 py-3">
        <p class="text-[12px] font-medium text-foreground">
          {{ t("settings.advanced.webToken") }}
        </p>
        <p class="text-[11.5px] leading-relaxed text-muted-foreground">
          {{ t("settings.advanced.webTokenDesc") }}
        </p>
        <div class="flex min-w-0 items-center gap-2">
          <code
            class="min-w-0 flex-1 truncate rounded-md bg-muted px-2 py-1 text-[11px] text-foreground/90"
            :title="webCfg.token || undefined"
          >
            {{ webCfg.token || t("settings.advanced.webTokenEmpty") }}
          </code>
          <Button
            type="button"
            size="sm"
            variant="outline"
            class="shrink-0"
            :disabled="webBusy || !webCfg.token"
            @click="copyText(webCfg.token)"
          >
            {{ t("settings.advanced.webCopyToken") }}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="outline"
            class="shrink-0"
            :disabled="webBusy || !backendReady"
            @click="onRegenToken"
          >
            {{ t("settings.advanced.webRegenToken") }}
          </Button>
        </div>
      </div>

      <div class="space-y-2 border-t border-border/60 py-3">
        <p class="text-[12px] font-medium text-foreground">
          {{ t("settings.advanced.webStatus") }}
        </p>
        <p class="text-[12px] text-muted-foreground">
          <template v-if="webStatus.running">
            {{ t("settings.advanced.webRunning") }}
          </template>
          <template v-else-if="webLoaded">
            {{ t("settings.advanced.webStopped") }}
          </template>
          <template v-else>
            {{ t("common.loading") }}
          </template>
        </p>
        <div v-if="webAccessUrl" class="min-w-0 space-y-1.5">
          <p class="text-[11px] text-muted-foreground">
            {{ t("settings.advanced.webLocalUrl") }}
          </p>
          <div class="flex min-w-0 items-center gap-2">
            <code
              class="min-w-0 flex-1 truncate rounded-md bg-muted px-2 py-1 text-[11px]"
              :title="webAccessUrl"
            >
              {{ webAccessUrl }}
            </code>
            <Button
              type="button"
              size="sm"
              variant="outline"
              class="shrink-0"
              @click="copyText(webAccessUrl)"
            >
              {{ t("settings.advanced.webCopyUrl") }}
            </Button>
          </div>
        </div>
        <div v-if="webLanUrl" class="min-w-0 space-y-1.5">
          <p class="text-[11px] text-muted-foreground">
            {{ t("settings.advanced.webLanUrl") }}
          </p>
          <div class="flex min-w-0 items-center gap-2">
            <code
              class="min-w-0 flex-1 truncate rounded-md bg-muted px-2 py-1 text-[11px]"
              :title="webLanUrl"
            >
              {{ webLanUrl }}
            </code>
            <Button
              type="button"
              size="sm"
              variant="outline"
              class="shrink-0"
              @click="copyText(webLanUrl)"
            >
              {{ t("settings.advanced.webCopyUrl") }}
            </Button>
          </div>
        </div>
        <p
          v-if="webStatus.error"
          class="text-[11.5px] text-destructive"
        >
          {{ webStatus.error }}
        </p>
      </div>
    </SettingsGroup>

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
