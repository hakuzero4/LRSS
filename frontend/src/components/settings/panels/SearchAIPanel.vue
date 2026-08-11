<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { Brain, Loader2, Sparkles } from "@lucide/vue";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { toast } from "vue-sonner";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

/** Bound at runtime when Wails services exist; UI stays usable in pure Vite. */
type EmbeddingConfig = {
  provider: string;
  baseUrl: string;
  apiKey: string;
  model: string;
  dimensions: number;
  batchSize: number;
};

type LLMConfig = {
  provider: string;
  baseUrl: string;
  apiKey: string;
  model: string;
  temperature: number;
  maxTokens: number;
  systemPrompt: string;
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

const llmForm = reactive<LLMConfig>({
  provider: "disabled",
  baseUrl: "",
  apiKey: "",
  model: "",
  temperature: 0.3,
  maxTokens: 2048,
  systemPrompt: "",
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
const llmSaving = ref(false);
const llmTesting = ref(false);
const loading = ref(true);
const enabled = ref(false);
const llmEnabled = ref(false);
const llmConfigured = ref(false);

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
    if (Settings?.GetLLMConfig) {
      const cfg = await Settings.GetLLMConfig();
      Object.assign(llmForm, {
        provider: cfg.provider ?? "disabled",
        baseUrl: cfg.baseUrl ?? cfg.BaseURL ?? "",
        apiKey: cfg.apiKey ?? cfg.APIKey ?? "",
        model: cfg.model ?? "",
        temperature: Number(cfg.temperature ?? cfg.Temperature ?? 0.3),
        maxTokens: Number(cfg.maxTokens ?? cfg.MaxTokens ?? 2048),
        systemPrompt: cfg.systemPrompt ?? cfg.SystemPrompt ?? "",
      });
      llmEnabled.value = (cfg.provider ?? cfg.Provider) === "openai_compatible";
      llmConfigured.value = llmEnabled.value && !!(llmForm.model && llmForm.baseUrl);
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

async function saveLLM() {
  llmSaving.value = true;
  try {
    const Settings = await loadSettingsModule();
    if (!Settings?.SetLLMConfig) {
      toast.message(t("settings.searchAi.previewMode"), {
        description: t("settings.searchAi.backendDisconnected"),
      });
      return;
    }
    const payload = {
      provider: llmEnabled.value ? "openai_compatible" : "disabled",
      baseUrl: llmForm.baseUrl,
      apiKey: llmForm.apiKey,
      model: llmForm.model,
      temperature: Number(llmForm.temperature) || 0,
      maxTokens: Number(llmForm.maxTokens) || 0,
      systemPrompt: llmForm.systemPrompt,
    };
    await Settings.SetLLMConfig(payload);
    toast.success(t("settings.searchAi.llmSaved"), {
      description: llmEnabled.value
        ? t("settings.searchAi.llmSavedEnabled")
        : t("settings.searchAi.llmSavedDisabled"),
    });
    await load();
  } catch (e: any) {
    toast.error(t("settings.searchAi.saveFailed"), { description: e?.message || String(e) });
  } finally {
    llmSaving.value = false;
  }
}

async function testLLM() {
  llmTesting.value = true;
  try {
    const Settings = await loadSettingsModule();
    if (!Settings?.TestLLMConfig) {
      toast.message(t("settings.searchAi.previewMode"));
      return;
    }
    const payload = {
      provider: "openai_compatible",
      baseUrl: llmForm.baseUrl,
      apiKey: llmForm.apiKey,
      model: llmForm.model,
      temperature: Number(llmForm.temperature) || 0,
      maxTokens: Number(llmForm.maxTokens) || 16,
      systemPrompt: llmForm.systemPrompt,
    };
    const reply = await Settings.TestLLMConfig(payload);
    toast.success(t("settings.searchAi.testOk"), {
      description: typeof reply === "string" ? reply : String(reply ?? ""),
    });
  } catch (e: any) {
    toast.error(t("settings.searchAi.testFailed"), { description: e?.message || String(e) });
  } finally {
    llmTesting.value = false;
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
        <p>
          {{
            t("settings.searchAi.llmStatus", {
              status: llmConfigured ? t("common.enabled") : t("common.notEnabled"),
            })
          }}
        </p>
        <p v-if="caps.reason">{{ caps.reason }}</p>
        <p class="mt-1 font-mono text-[11px] opacity-80">{{ vectorStatus }}</p>
      </template>
    </div>

    <!-- —— LLM (chat) —— -->
    <SettingsGroup
      :title="t('settings.searchAi.llmGroup')"
      :description="t('settings.searchAi.llmGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.searchAi.enableLlm')"
          :description="t('settings.searchAi.enableLlmDesc')"
        >
          <Switch
            :checked="llmEnabled"
            @update:checked="(v: boolean) => (llmEnabled = v)"
          />
        </SettingsRow>
      </div>

      <div class="space-y-3 py-3" :class="!llmEnabled && 'opacity-50 pointer-events-none'">
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.baseUrl") }}</Label>
          <Input
            v-model="llmForm.baseUrl"
            placeholder="https://api.openai.com/v1"
            class="h-9 text-[13px]"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.apiKey") }}</Label>
          <Input
            v-model="llmForm.apiKey"
            type="password"
            placeholder="sk-…（本地模型可留空）"
            class="h-9 text-[13px]"
            autocomplete="off"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.llmModel") }}</Label>
          <Input
            v-model="llmForm.model"
            :placeholder="t('settings.searchAi.llmModelPlaceholder')"
            class="h-9 text-[13px]"
          />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1.5">
            <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.temperature") }}</Label>
            <Input
              v-model.number="llmForm.temperature"
              type="number"
              min="0"
              max="2"
              step="0.1"
              class="h-9 text-[13px]"
            />
          </div>
          <div class="space-y-1.5">
            <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.maxTokens") }}</Label>
            <Input
              v-model.number="llmForm.maxTokens"
              type="number"
              min="0"
              max="128000"
              class="h-9 text-[13px]"
            />
          </div>
        </div>
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">{{ t("settings.searchAi.systemPrompt") }}</Label>
          <textarea
            v-model="llmForm.systemPrompt"
            rows="3"
            :placeholder="t('settings.searchAi.systemPromptPlaceholder')"
            class="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex w-full rounded-md border px-3 py-2 text-[13px] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
          />
        </div>
      </div>

      <div class="flex flex-wrap gap-2 py-3">
        <Button size="sm" :disabled="llmSaving" @click="saveLLM">
          <Sparkles class="mr-1.5 size-3.5" />
          {{ llmSaving ? t("settings.searchAi.saving") : t("settings.searchAi.saveConfig") }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="!llmEnabled || llmTesting"
          @click="testLLM"
        >
          <Loader2 v-if="llmTesting" class="mr-1.5 size-3.5 animate-spin" />
          {{ llmTesting ? t("settings.searchAi.testing") : t("settings.searchAi.testConnection") }}
        </Button>
      </div>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.searchAi.autoGroup')"
      :description="t('settings.searchAi.autoGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.searchAi.autoSummarize')"
          :description="t('settings.searchAi.autoSummarizeDesc')"
        >
          <Switch
            :checked="settings.autoSummarize"
            @update:checked="
              (v: boolean) => {
                settings.autoSummarize = v;
                persistUIPrefs();
              }
            "
          />
        </SettingsRow>
      </div>
    </SettingsGroup>

    <SettingsGroup :title="t('settings.searchAi.llmNotesTitle')">
      <div class="space-y-2.5 py-3 text-[12.5px] leading-relaxed text-muted-foreground">
        <p class="text-[13px] font-medium text-foreground">
          {{ t("settings.searchAi.llmFeaturesTitle") }}
        </p>
        <p>{{ t("settings.searchAi.llmFeatureSummarize") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureTranslate") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureAsk") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureDigest") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureSuggest") }}</p>
        <p>{{ t("settings.searchAi.llmFeatureClassify") }}</p>
        <div class="my-2 border-t border-border/60" />
        <p>{{ t("settings.searchAi.llmNoteCompat") }}</p>
        <p>{{ t("settings.searchAi.llmNoteSeparate") }}</p>
        <p>{{ t("settings.searchAi.llmNoteCache") }}</p>
        <p>{{ t("settings.searchAi.llmNoteLocale") }}</p>
        <p>{{ t("settings.searchAi.llmNotePrivacy") }}</p>
      </div>
    </SettingsGroup>

    <!-- —— Embedding (vector) —— -->
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
