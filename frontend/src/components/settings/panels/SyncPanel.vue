<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import { CloudUpload, CloudDownload, Loader2, PlugZap } from "@lucide/vue";
import { toast } from "vue-sonner";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useRssStore } from "@/composables/useRssStore";
import { loadAppsvc } from "@/lib/backend";

const { t } = useI18n();
const { reloadLibrary } = useRssStore();

type SyncConfig = {
  enabled: boolean;
  provider: string;
  objectKey: string;
  webdavUrl: string;
  webdavUsername: string;
  webdavPassword: string;
  webdavPath: string;
  s3Endpoint: string;
  s3Region: string;
  s3Bucket: string;
  s3AccessKey: string;
  s3SecretKey: string;
  s3ForcePathStyle: boolean;
  s3UseSSL: boolean;
  lastPushAt?: string;
  lastPullAt?: string;
  lastError?: string;
};

const form = reactive<SyncConfig>({
  enabled: false,
  provider: "none",
  objectKey: "lrss-subscriptions.opml",
  webdavUrl: "",
  webdavUsername: "",
  webdavPassword: "",
  webdavPath: "",
  s3Endpoint: "",
  s3Region: "auto",
  s3Bucket: "",
  s3AccessKey: "",
  s3SecretKey: "",
  s3ForcePathStyle: true,
  s3UseSSL: true,
  lastPushAt: "",
  lastPullAt: "",
  lastError: "",
});

const loading = ref(true);
const saving = ref(false);
const testing = ref(false);
const pushing = ref(false);
const pulling = ref(false);
const fetchAfterPull = ref(false);

const providerModel = computed({
  get: () => form.provider || "none",
  set: (v: string) => {
    form.provider = v;
    if (v === "none") form.enabled = false;
    else if (!form.enabled) form.enabled = true;
  },
});

const isWebDAV = computed(() => form.provider === "webdav");
const isS3 = computed(() => form.provider === "s3");
const busy = computed(() => saving.value || testing.value || pushing.value || pulling.value);

async function loadSyncModule(): Promise<any | null> {
  try {
    const api = await loadAppsvc();
    return api?.SyncService ?? null;
  } catch {
    return null;
  }
}

function assignConfig(cfg: Record<string, unknown>) {
  form.enabled = !!(cfg.enabled ?? cfg.Enabled);
  form.provider = String(cfg.provider ?? cfg.Provider ?? "none");
  form.objectKey = String(cfg.objectKey ?? cfg.ObjectKey ?? "lrss-subscriptions.opml");
  form.webdavUrl = String(cfg.webdavUrl ?? cfg.WebDAVURL ?? "");
  form.webdavUsername = String(cfg.webdavUsername ?? cfg.WebDAVUsername ?? "");
  form.webdavPassword = String(cfg.webdavPassword ?? cfg.WebDAVPassword ?? "");
  form.webdavPath = String(cfg.webdavPath ?? cfg.WebDAVPath ?? "");
  form.s3Endpoint = String(cfg.s3Endpoint ?? cfg.S3Endpoint ?? "");
  form.s3Region = String(cfg.s3Region ?? cfg.S3Region ?? "auto");
  form.s3Bucket = String(cfg.s3Bucket ?? cfg.S3Bucket ?? "");
  form.s3AccessKey = String(cfg.s3AccessKey ?? cfg.S3AccessKey ?? "");
  form.s3SecretKey = String(cfg.s3SecretKey ?? cfg.S3SecretKey ?? "");
  form.s3ForcePathStyle = !!(cfg.s3ForcePathStyle ?? cfg.S3ForcePathStyle ?? true);
  form.s3UseSSL = cfg.s3UseSSL !== false && cfg.S3UseSSL !== false;
  form.lastPushAt = String(cfg.lastPushAt ?? cfg.LastPushAt ?? "");
  form.lastPullAt = String(cfg.lastPullAt ?? cfg.LastPullAt ?? "");
  form.lastError = String(cfg.lastError ?? cfg.LastError ?? "");
}

async function load() {
  loading.value = true;
  try {
    const Sync = await loadSyncModule();
    if (Sync?.GetSyncConfig) {
      const cfg = await Sync.GetSyncConfig();
      assignConfig(cfg ?? {});
    }
  } catch (e) {
    console.warn(e);
  } finally {
    loading.value = false;
  }
}

function payload(): SyncConfig {
  return {
    enabled: form.enabled && form.provider !== "none",
    provider: form.provider,
    objectKey: form.objectKey || "lrss-subscriptions.opml",
    webdavUrl: form.webdavUrl,
    webdavUsername: form.webdavUsername,
    webdavPassword: form.webdavPassword,
    webdavPath: form.webdavPath,
    s3Endpoint: form.s3Endpoint,
    s3Region: form.s3Region || "auto",
    s3Bucket: form.s3Bucket,
    s3AccessKey: form.s3AccessKey,
    s3SecretKey: form.s3SecretKey,
    s3ForcePathStyle: form.s3ForcePathStyle,
    s3UseSSL: form.s3UseSSL,
  };
}

async function save() {
  saving.value = true;
  try {
    const Sync = await loadSyncModule();
    if (!Sync?.SetSyncConfig) {
      toast.message(t("settings.sync.backendUnavailable"));
      return;
    }
    await Sync.SetSyncConfig(payload());
    toast.success(t("settings.sync.saved"));
    await load();
  } catch (e: any) {
    toast.error(t("settings.sync.saveFailed"), {
      description: e?.message || String(e),
    });
  } finally {
    saving.value = false;
  }
}

async function testConnection() {
  testing.value = true;
  try {
    const Sync = await loadSyncModule();
    if (!Sync?.TestSyncConnection) {
      toast.message(t("settings.sync.backendUnavailable"));
      return;
    }
    const res = await Sync.TestSyncConnection(payload());
    const ok = !!(res?.ok ?? res?.OK);
    const msg = String(res?.message ?? res?.Message ?? "");
    if (ok) {
      toast.success(t("settings.sync.testOk"), { description: msg || "ok" });
    } else {
      toast.error(t("settings.sync.testFailed"), { description: msg });
    }
  } catch (e: any) {
    toast.error(t("settings.sync.testFailed"), {
      description: e?.message || String(e),
    });
  } finally {
    testing.value = false;
  }
}

async function pushRemote() {
  pushing.value = true;
  try {
    const Sync = await loadSyncModule();
    if (!Sync?.PushSubscriptions) {
      toast.message(t("settings.sync.backendUnavailable"));
      return;
    }
    // Ensure latest form is saved before push.
    if (Sync.SetSyncConfig) {
      await Sync.SetSyncConfig(payload());
    }
    const res = await Sync.PushSubscriptions();
    toast.success(t("settings.sync.pushOk"), {
      description: t("settings.sync.pushOkDesc", {
        n: res?.bytes ?? res?.Bytes ?? 0,
      }),
    });
    await load();
  } catch (e: any) {
    toast.error(t("settings.sync.pushFailed"), {
      description: e?.message || String(e),
    });
  } finally {
    pushing.value = false;
  }
}

async function pullRemote() {
  pulling.value = true;
  try {
    const Sync = await loadSyncModule();
    if (!Sync?.PullSubscriptions) {
      toast.message(t("settings.sync.backendUnavailable"));
      return;
    }
    if (Sync.SetSyncConfig) {
      await Sync.SetSyncConfig(payload());
    }
    const res = await Sync.PullSubscriptions(!!fetchAfterPull.value);
    toast.success(t("settings.sync.pullOk"), {
      description: t("settings.sync.pullOkDesc", {
        added: res?.imported ?? res?.Imported ?? 0,
        skipped: res?.skipped ?? res?.Skipped ?? 0,
        folders: res?.folders ?? res?.Folders ?? 0,
      }),
    });
    await reloadLibrary();
    await load();
  } catch (e: any) {
    toast.error(t("settings.sync.pullFailed"), {
      description: e?.message || String(e),
    });
  } finally {
    pulling.value = false;
  }
}

onMounted(() => {
  void load();
});
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.sync.group')"
      :description="t('settings.sync.groupDesc')"
    >
      <p
        class="mb-2 rounded-md border border-border/70 bg-muted/40 px-3 py-2 text-[12px] leading-relaxed text-muted-foreground"
        role="note"
      >
        {{ t("settings.sync.scopeNote") }}
      </p>

      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.sync.enable')"
          :description="t('settings.sync.enableDesc')"
        >
          <Switch
            :checked="form.enabled"
            :disabled="loading || form.provider === 'none'"
            @update:checked="(v: boolean) => (form.enabled = v)"
          />
        </SettingsRow>
      </div>

      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.sync.provider')"
          :description="t('settings.sync.providerDesc')"
        >
          <Select v-model="providerModel" :disabled="loading">
            <SelectTrigger class="h-8 w-[180px] text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">{{ t("settings.sync.providerNone") }}</SelectItem>
              <SelectItem value="webdav">{{ t("settings.sync.providerWebDAV") }}</SelectItem>
              <SelectItem value="s3">{{ t("settings.sync.providerS3") }}</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>

      <div v-if="form.provider !== 'none'" class="space-y-3 border-t border-border/50 py-3">
        <div class="space-y-1.5">
          <Label class="text-[12.5px]">{{ t("settings.sync.objectKey") }}</Label>
          <Input
            v-model="form.objectKey"
            class="h-8 text-[13px]"
            :placeholder="t('settings.sync.objectKeyPlaceholder')"
            :disabled="loading"
          />
          <p class="text-[11.5px] text-muted-foreground">
            {{ t("settings.sync.objectKeyDesc") }}
          </p>
        </div>

        <!-- WebDAV fields -->
        <template v-if="isWebDAV">
          <div class="space-y-1.5">
            <Label class="text-[12.5px]">{{ t("settings.sync.webdavUrl") }}</Label>
            <Input
              v-model="form.webdavUrl"
              class="h-8 font-mono text-[12.5px]"
              placeholder="https://dav.example.com/remote.php/dav/files/user"
              :disabled="loading"
            />
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-[12.5px]">{{ t("settings.sync.webdavUser") }}</Label>
              <Input
                v-model="form.webdavUsername"
                class="h-8 text-[13px]"
                autocomplete="username"
                :disabled="loading"
              />
            </div>
            <div class="space-y-1.5">
              <Label class="text-[12.5px]">{{ t("settings.sync.webdavPassword") }}</Label>
              <Input
                v-model="form.webdavPassword"
                type="password"
                class="h-8 text-[13px]"
                autocomplete="current-password"
                :disabled="loading"
              />
            </div>
          </div>
          <div class="space-y-1.5">
            <Label class="text-[12.5px]">{{ t("settings.sync.webdavPath") }}</Label>
            <Input
              v-model="form.webdavPath"
              class="h-8 font-mono text-[12.5px]"
              :placeholder="t('settings.sync.webdavPathPlaceholder')"
              :disabled="loading"
            />
            <p class="text-[11.5px] text-muted-foreground">
              {{ t("settings.sync.webdavPathDesc") }}
            </p>
          </div>
        </template>

        <!-- S3 / R2 / MinIO fields -->
        <template v-if="isS3">
          <div class="space-y-1.5">
            <Label class="text-[12.5px]">{{ t("settings.sync.s3Endpoint") }}</Label>
            <Input
              v-model="form.s3Endpoint"
              class="h-8 font-mono text-[12.5px]"
              placeholder="https://xxxx.r2.cloudflarestorage.com 或 http://127.0.0.1:9000"
              :disabled="loading"
            />
            <p class="text-[11.5px] text-muted-foreground">
              {{ t("settings.sync.s3EndpointDesc") }}
            </p>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-[12.5px]">{{ t("settings.sync.s3Bucket") }}</Label>
              <Input v-model="form.s3Bucket" class="h-8 text-[13px]" :disabled="loading" />
            </div>
            <div class="space-y-1.5">
              <Label class="text-[12.5px]">{{ t("settings.sync.s3Region") }}</Label>
              <Input
                v-model="form.s3Region"
                class="h-8 text-[13px]"
                placeholder="auto"
                :disabled="loading"
              />
            </div>
          </div>
          <div class="grid grid-cols-2 gap-2">
            <div class="space-y-1.5">
              <Label class="text-[12.5px]">{{ t("settings.sync.s3AccessKey") }}</Label>
              <Input
                v-model="form.s3AccessKey"
                class="h-8 font-mono text-[12.5px]"
                autocomplete="off"
                :disabled="loading"
              />
            </div>
            <div class="space-y-1.5">
              <Label class="text-[12.5px]">{{ t("settings.sync.s3SecretKey") }}</Label>
              <Input
                v-model="form.s3SecretKey"
                type="password"
                class="h-8 font-mono text-[12.5px]"
                autocomplete="off"
                :disabled="loading"
              />
            </div>
          </div>
          <div class="py-1">
            <SettingsRow
              :title="t('settings.sync.s3PathStyle')"
              :description="t('settings.sync.s3PathStyleDesc')"
            >
              <Switch
                :checked="form.s3ForcePathStyle"
                :disabled="loading"
                @update:checked="(v: boolean) => (form.s3ForcePathStyle = v)"
              />
            </SettingsRow>
          </div>
        </template>

        <div class="flex flex-wrap gap-2 pt-1">
          <Button
            size="sm"
            variant="secondary"
            class="h-8"
            :disabled="busy || loading"
            @click="save"
          >
            <Loader2 v-if="saving" class="size-3.5 animate-spin" />
            {{ t("settings.sync.save") }}
          </Button>
          <Button
            size="sm"
            variant="outline"
            class="h-8"
            :disabled="busy || loading || form.provider === 'none'"
            @click="testConnection"
          >
            <Loader2 v-if="testing" class="size-3.5 animate-spin" />
            <PlugZap v-else class="size-3.5" />
            {{ t("settings.sync.test") }}
          </Button>
        </div>
      </div>
    </SettingsGroup>

    <SettingsGroup
      v-if="form.provider !== 'none'"
      :title="t('settings.sync.actionsGroup')"
      :description="t('settings.sync.actionsGroupDesc')"
    >
      <div class="flex flex-wrap gap-2 py-3">
        <Button
          size="sm"
          class="h-8"
          :disabled="busy || loading || !form.enabled"
          @click="pushRemote"
        >
          <Loader2 v-if="pushing" class="size-3.5 animate-spin" />
          <CloudUpload v-else class="size-3.5" />
          {{ t("settings.sync.push") }}
        </Button>
        <Button
          size="sm"
          variant="outline"
          class="h-8"
          :disabled="busy || loading || !form.enabled"
          @click="pullRemote"
        >
          <Loader2 v-if="pulling" class="size-3.5 animate-spin" />
          <CloudDownload v-else class="size-3.5" />
          {{ t("settings.sync.pull") }}
        </Button>
      </div>
      <div class="py-1">
        <SettingsRow
          :title="t('settings.sync.fetchAfterPull')"
          :description="t('settings.sync.fetchAfterPullDesc')"
        >
          <Switch
            :checked="fetchAfterPull"
            :disabled="busy"
            @update:checked="(v: boolean) => (fetchAfterPull = v)"
          />
        </SettingsRow>
      </div>
      <div
        v-if="form.lastPushAt || form.lastPullAt || form.lastError"
        class="space-y-1 border-t border-border/50 py-3 text-[12px] text-muted-foreground"
      >
        <p v-if="form.lastPushAt">
          {{ t("settings.sync.lastPush", { t: form.lastPushAt }) }}
        </p>
        <p v-if="form.lastPullAt">
          {{ t("settings.sync.lastPull", { t: form.lastPullAt }) }}
        </p>
        <p v-if="form.lastError" class="text-destructive">
          {{ t("settings.sync.lastError", { e: form.lastError }) }}
        </p>
      </div>
    </SettingsGroup>
  </div>
</template>
