<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { Brain, Loader2 } from "@lucide/vue";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { toast } from "vue-sonner";

const { t } = useI18n();

/** Bound at runtime when Wails services exist; UI stays usable in pure Vite. */
type EmbeddingConfig = {
  provider: string;
  baseUrl: string;
  apiKey: string;
  model: string;
  dimensions: number;
  batchSize: number;
};

type Caps = {
  fts: boolean;
  vectorExtension: boolean;
  embeddingConfigured: boolean;
  vectorSearch: boolean;
  reason?: string;
};

const form = reactive<EmbeddingConfig>({
  provider: "disabled",
  baseUrl: "",
  apiKey: "",
  model: "",
  dimensions: 1536,
  batchSize: 16,
});

const caps = ref<Caps>({
  fts: true,
  vectorExtension: false,
  embeddingConfigured: false,
  vectorSearch: false,
  reason: undefined,
});
const vectorStatus = ref("");
const saving = ref(false);
const loading = ref(true);
const enabled = ref(false);

async function loadSettingsModule(): Promise<any> {
  try {
    const mod = await import("../../../../bindings/lrss/internal/appsvc/index.js");
    return (mod as { SettingsService?: unknown }).SettingsService ?? null;
  } catch {
    return null;
  }
}

async function load() {
  loading.value = true;
  try {
    const Settings = await loadSettingsModule();
    if (Settings?.GetEmbeddingConfig) {
      const cfg = await Settings.GetEmbeddingConfig();
      Object.assign(form, cfg);
      enabled.value = cfg.provider === "openai_compatible";
    } else {
      caps.value.reason = t("settings.searchAi.previewReason");
    }
    if (Settings?.GetSearchCapabilities) {
      caps.value = await Settings.GetSearchCapabilities();
    }
    if (Settings?.GetVectorStatus) {
      const st = await Settings.GetVectorStatus();
      vectorStatus.value = st.loaded
        ? t("settings.searchAi.extLoaded", {
            version: st.version || "",
            backend: st.backend || "",
          })
        : t("settings.searchAi.extNotLoaded", {
            error: st.error || t("settings.searchAi.extCosineFallback"),
          });
    }
  } catch (e) {
    console.warn(e);
  } finally {
    loading.value = false;
  }
}

async function save() {
  saving.value = true;
  try {
    const Settings = await loadSettingsModule();
    if (!Settings?.SetEmbeddingConfig) {
      toast.message(t("settings.searchAi.previewMode"), {
        description: t("settings.searchAi.backendDisconnected"),
      });
      return;
    }
    const payload = {
      ...form,
      provider: enabled.value ? "openai_compatible" : "disabled",
      dimensions: Number(form.dimensions) || 0,
      batchSize: Number(form.batchSize) || 16,
    };
    await Settings.SetEmbeddingConfig(payload);
    toast.success(t("settings.searchAi.saved"), {
      description: enabled.value
        ? t("settings.searchAi.savedEnabled")
        : t("settings.searchAi.savedDisabled"),
    });
    await load();
  } catch (e: any) {
    toast.error(t("settings.searchAi.saveFailed"), { description: e?.message || String(e) });
  } finally {
    saving.value = false;
  }
}

async function runEmbed() {
  try {
    const Settings = await loadSettingsModule();
    if (!Settings?.RunEmbedOnce) {
      toast.message(t("settings.searchAi.previewMode"));
      return;
    }
    const n = await Settings.RunEmbedOnce(32);
    toast.success(t("settings.searchAi.embedded", { n }));
  } catch (e: any) {
    toast.error(t("settings.searchAi.embedFailed"), { description: e?.message || String(e) });
  }
}

const rebuilding = ref(false);

async function rebuildAll() {
  if (!enabled.value) {
    toast.message(t("settings.searchAi.enableFirst"));
    return;
  }
  rebuilding.value = true;
  try {
    const Settings = await loadSettingsModule();
    if (!Settings?.RebuildAllEmbeddings) {
      toast.message(t("settings.searchAi.previewMode"), {
        description: t("settings.searchAi.backendDisconnectedShort"),
      });
      return;
    }
    const res = await Settings.RebuildAllEmbeddings();
    const processed = res?.processed ?? res?.Processed ?? 0;
    toast.success(t("settings.searchAi.rebuilt"), {
      description: t("settings.searchAi.rebuiltDesc", { n: processed }),
    });
    await load();
  } catch (e: any) {
    toast.error(t("settings.searchAi.rebuildFailed"), { description: e?.message || String(e) });
  } finally {
    rebuilding.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div class="space-y-7">
    <div
      class="rounded-xl border border-border bg-muted/40 px-3.5 py-3 text-[12.5px] leading-relaxed text-muted-foreground"
    >
      <div class="mb-1 flex items-center gap-2 text-[13px] font-medium text-foreground">
        <Brain class="size-4 text-primary" />
        {{ t("settings.searchAi.capabilities") }}
      </div>
      <p v-if="loading" class="flex items-center gap-2">
        <Loader2 class="size-3.5 animate-spin" /> {{ t("settings.searchAi.loading") }}
      </p>
      <template v-else>
        <p>
          {{
            t("settings.searchAi.fts", {
              status: caps.fts ? t("common.enabled") : t("common.disabled"),
            })
          }}
        </p>
        <p>
          {{
            t("settings.searchAi.vector", {
              status: caps.vectorSearch ? t("common.enabled") : t("common.notEnabled"),
            })
          }}
        </p>
        <p v-if="caps.reason">{{ caps.reason }}</p>
        <p class="mt-1 font-mono text-[11px] opacity-80">{{ vectorStatus }}</p>
      </template>
    </div>

    <SettingsGroup
      :title="t('settings.searchAi.vectorGroup')"
      :description="t('settings.searchAi.vectorGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.searchAi.enableSemantic')"
          :description="t('settings.searchAi.enableSemanticDesc')"
        >
          <Switch
            :checked="enabled"
            @update:checked="(v: boolean) => (enabled = v)"
          />
        </SettingsRow>
      </div>

      <div class="space-y-3 py-3" :class="!enabled && 'opacity-50 pointer-events-none'">
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.baseUrl") }}</Label>
          <Input
            v-model="form.baseUrl"
            placeholder="https://api.openai.com/v1"
            class="h-9 text-[13px]"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.apiKey") }}</Label>
          <Input
            v-model="form.apiKey"
            type="password"
            placeholder="sk-…"
            class="h-9 text-[13px]"
            autocomplete="off"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.model") }}</Label>
          <Input
            v-model="form.model"
            placeholder="text-embedding-3-small"
            class="h-9 text-[13px]"
          />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1.5">
            <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.dimensions") }}</Label>
            <Input
              v-model.number="form.dimensions"
              type="number"
              min="32"
              max="4096"
              class="h-9 text-[13px]"
            />
          </div>
          <div class="space-y-1.5">
            <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.batchSize") }}</Label>
            <Input
              v-model.number="form.batchSize"
              type="number"
              min="1"
              max="128"
              class="h-9 text-[13px]"
            />
          </div>
        </div>
      </div>

      <div class="flex flex-wrap gap-2 py-3">
        <Button size="sm" :disabled="saving" @click="save">
          {{ saving ? t("settings.searchAi.saving") : t("settings.searchAi.saveConfig") }}
        </Button>
        <Button size="sm" variant="outline" :disabled="!enabled" @click="runEmbed">
          {{ t("settings.searchAi.embedBatch") }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="!enabled || rebuilding"
          @click="rebuildAll"
        >
          {{ rebuilding ? t("settings.searchAi.rebuilding") : t("settings.searchAi.rebuildAll") }}
        </Button>
      </div>
    </SettingsGroup>

    <SettingsGroup :title="t('settings.searchAi.notesTitle')">
      <div class="space-y-2 py-3 text-[12.5px] leading-relaxed text-muted-foreground">
        <p>{{ t("settings.searchAi.noteCompat") }}</p>
        <p>{{ t("settings.searchAi.noteExtension") }}</p>
        <p>{{ t("settings.searchAi.noteModelChange") }}</p>
      </div>
    </SettingsGroup>
  </div>
</template>
