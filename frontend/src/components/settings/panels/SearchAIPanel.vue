<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { Brain, Loader2 } from "@lucide/vue";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { toast } from "vue-sonner";

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
  reason: "后端未连接时显示为设计预览",
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
    }
    if (Settings?.GetSearchCapabilities) {
      caps.value = await Settings.GetSearchCapabilities();
    }
    if (Settings?.GetVectorStatus) {
      const st = await Settings.GetVectorStatus();
      vectorStatus.value = st.loaded
        ? `扩展已加载 · ${st.version || ""} · ${st.backend || ""}`
        : `扩展未加载 · ${st.error || "进程内余弦可用"}`;
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
      toast.message("设计预览", { description: "Go 服务未连接，配置未写入。" });
      return;
    }
    const payload = {
      ...form,
      provider: enabled.value ? "openai_compatible" : "disabled",
      dimensions: Number(form.dimensions) || 0,
      batchSize: Number(form.batchSize) || 16,
    };
    await Settings.SetEmbeddingConfig(payload);
    toast.success("已保存", {
      description: enabled.value
        ? "向量搜索已启用（进程内余弦；扩展见 docs/embedding.md）"
        : "已关闭向量，使用全文检索",
    });
    await load();
  } catch (e: any) {
    toast.error("保存失败", { description: e?.message || String(e) });
  } finally {
    saving.value = false;
  }
}

async function runEmbed() {
  try {
    const Settings = await loadSettingsModule();
    if (!Settings?.RunEmbedOnce) {
      toast.message("设计预览");
      return;
    }
    const n = await Settings.RunEmbedOnce(32);
    toast.success(`已处理 ${n} 篇文章向量`);
  } catch (e: any) {
    toast.error("回填失败", { description: e?.message || String(e) });
  }
}

const rebuilding = ref(false);

async function rebuildAll() {
  if (!enabled.value) {
    toast.message("请先启用并保存向量模型");
    return;
  }
  rebuilding.value = true;
  try {
    const Settings = await loadSettingsModule();
    if (!Settings?.RebuildAllEmbeddings) {
      toast.message("设计预览", { description: "Go 服务未连接" });
      return;
    }
    const res = await Settings.RebuildAllEmbeddings();
    const processed = res?.processed ?? res?.Processed ?? 0;
    toast.success("向量已重新生成", {
      description: `共处理 ${processed} 篇文章`,
    });
    await load();
  } catch (e: any) {
    toast.error("重建失败", { description: e?.message || String(e) });
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
        搜索能力
      </div>
      <p v-if="loading" class="flex items-center gap-2">
        <Loader2 class="size-3.5 animate-spin" /> 加载中…
      </p>
      <template v-else>
        <p>全文检索：{{ caps.fts ? "可用" : "不可用" }}</p>
        <p>向量搜索：{{ caps.vectorSearch ? "可用" : "未启用" }}</p>
        <p v-if="caps.reason">{{ caps.reason }}</p>
        <p class="mt-1 font-mono text-[11px] opacity-80">{{ vectorStatus }}</p>
      </template>
    </div>

    <SettingsGroup
      title="向量模型"
      description="配置 OpenAI 兼容的 Embedding API。未配置时仅使用全文检索。"
    >
      <div class="py-2.5">
        <SettingsRow title="启用语义搜索" description="关闭后始终使用 FTS 全文。">
          <Switch
            :checked="enabled"
            @update:checked="(v: boolean) => (enabled = v)"
          />
        </SettingsRow>
      </div>

      <div class="space-y-3 py-3" :class="!enabled && 'opacity-50 pointer-events-none'">
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">Base URL</Label>
          <Input
            v-model="form.baseUrl"
            placeholder="https://api.openai.com/v1"
            class="h-9 text-[13px]"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">API Key</Label>
          <Input
            v-model="form.apiKey"
            type="password"
            placeholder="sk-…"
            class="h-9 text-[13px]"
            autocomplete="off"
          />
        </div>
        <div class="space-y-1.5">
          <Label class="text-[12px] text-muted-foreground">Model</Label>
          <Input
            v-model="form.model"
            placeholder="text-embedding-3-small"
            class="h-9 text-[13px]"
          />
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div class="space-y-1.5">
            <Label class="text-[12px] text-muted-foreground">Dimensions</Label>
            <Input
              v-model.number="form.dimensions"
              type="number"
              min="32"
              max="4096"
              class="h-9 text-[13px]"
            />
          </div>
          <div class="space-y-1.5">
            <Label class="text-[12px] text-muted-foreground">Batch size</Label>
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
          {{ saving ? "保存中…" : "保存配置" }}
        </Button>
        <Button size="sm" variant="outline" :disabled="!enabled" @click="runEmbed">
          回填一批
        </Button>
        <Button
          size="sm"
          variant="outline"
          :disabled="!enabled || rebuilding"
          @click="rebuildAll"
        >
          {{ rebuilding ? "重建中…" : "重新生成全部向量" }}
        </Button>
      </div>
    </SettingsGroup>

    <SettingsGroup title="说明">
      <div class="space-y-2 py-3 text-[12.5px] leading-relaxed text-muted-foreground">
        <p>· 兼容 OpenAI Embeddings API 与多数本地代理（含 Ollama 的 OpenAI 兼容层）。</p>
        <p>· 扩展未加载时，将使用进程内余弦距离（适合小库）；加载 sqlite-vector 后走加速检索。</p>
        <p>· 修改 model / dimensions 会清空旧向量并重新排队。</p>
      </div>
    </SettingsGroup>
  </div>
</template>
