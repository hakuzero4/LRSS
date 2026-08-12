<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { APP_VERSION, GITHUB_REPO_URL, GITHUB_RELEASES_URL } from "@/lib/appMeta";
import { checkForUpdate, type UpdateCheckResult } from "@/lib/checkUpdate";
import { loadAppsvc } from "@/lib/backend";
import { isWebMode } from "@/lib/webMode";
import { openExternalLink } from "@/lib/openLink";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import { Button } from "@/components/ui/button";

const { t } = useI18n();
const { feeds, smartCounts } = useRssStore();

const checking = ref(false);
const installing = ref(false);
const pendingUpdate = ref<Extract<UpdateCheckResult, { status: "updateAvailable" }> | null>(
  null,
);

const summary = computed(() =>
  t("settings.about.summary", {
    feeds: feeds.value.length,
    articles: smartCounts.all,
    unread: smartCounts.unread,
  }),
);

const busy = computed(() => checking.value || installing.value);

function openDocs() {
  void openExternalLink(GITHUB_REPO_URL, { forceBrowser: true });
}

function openReleases() {
  void openExternalLink(GITHUB_RELEASES_URL, { forceBrowser: true });
}

function mapBackendCheck(raw: any): UpdateCheckResult {
  const status = String(raw?.status ?? raw?.Status ?? "");
  const current = String(raw?.current ?? raw?.Current ?? APP_VERSION);
  const latest = String(raw?.latest ?? raw?.Latest ?? "");
  const htmlUrl = String(raw?.htmlUrl ?? raw?.HTMLURL ?? raw?.HtmlUrl ?? GITHUB_RELEASES_URL);
  const message = String(raw?.message ?? raw?.Message ?? "");
  const name = String(raw?.name ?? raw?.Name ?? "");
  if (status === "updateAvailable") {
    return {
      status: "updateAvailable",
      current,
      latest,
      htmlUrl,
      name: name || undefined,
      canInstall: !!(raw?.canInstall ?? raw?.CanInstall),
      asset: String(raw?.asset ?? raw?.Asset ?? "") || undefined,
    };
  }
  if (status === "upToDate") {
    return { status: "upToDate", current, latest, htmlUrl };
  }
  return { status: "error", current, message: message || "error" };
}

async function checkViaBackend(): Promise<UpdateCheckResult | null> {
  if (isWebMode()) return null;
  try {
    const api = await loadAppsvc();
    const fn = api?.UpdateService?.CheckForUpdate;
    if (typeof fn !== "function") return null;
    const raw = await fn();
    return mapBackendCheck(raw);
  } catch {
    return null;
  }
}

async function onCheckUpdate() {
  if (busy.value) return;
  checking.value = true;
  pendingUpdate.value = null;
  try {
    let result = await checkViaBackend();
    if (!result) {
      result = await checkForUpdate();
    }
    if (result.status === "upToDate") {
      toast.success(t("settings.about.updateUpToDateTitle"), {
        description: t("settings.about.updateUpToDateDesc", {
          version: result.current,
        }),
      });
      return;
    }
    if (result.status === "updateAvailable") {
      pendingUpdate.value = {
        ...result,
        canInstall: result.canInstall !== false && !isWebMode(),
      } as any;
      toast.message(t("settings.about.updateAvailableTitle"), {
        description: t("settings.about.updateAvailableDesc", {
          current: result.current,
          latest: result.latest,
        }),
        duration: 10_000,
      });
      return;
    }
    const code = result.message;
    const desc =
      code === "no_releases"
        ? t("settings.about.updateNoReleases")
        : code.startsWith("http_")
          ? t("settings.about.updateHttpError", { status: code.replace("http_", "") })
          : t("settings.about.updateNetworkError", { msg: code });
    toast.error(t("settings.about.updateFailedTitle"), {
      description: desc,
      action: {
        label: t("settings.about.openRelease"),
        onClick: () => openReleases(),
      },
    });
  } finally {
    checking.value = false;
  }
}

async function onInstallUpdate() {
  if (busy.value || isWebMode()) return;
  installing.value = true;
  try {
    const api = await loadAppsvc();
    const fn = api?.UpdateService?.DownloadAndInstall;
    if (typeof fn !== "function") {
      toast.error(t("settings.about.installUnavailable"));
      if (pendingUpdate.value?.htmlUrl) {
        void openExternalLink(pendingUpdate.value.htmlUrl, { forceBrowser: true });
      }
      return;
    }
    toast.message(t("settings.about.installDownloading"), {
      description: t("settings.about.installDownloadingDesc"),
      duration: 30_000,
    });
    const raw = await fn();
    const ok = !!(raw?.ok ?? raw?.OK);
    const msg = String(raw?.message ?? raw?.Message ?? "");
    if (!ok) {
      toast.error(t("settings.about.installFailedTitle"), {
        description:
          msg === "already_latest"
            ? t("settings.about.updateUpToDateDesc", { version: APP_VERSION })
            : msg === "no_matching_asset"
              ? t("settings.about.installNoAsset")
              : t("settings.about.installFailedDesc", { msg: msg || "error" }),
        action: {
          label: t("settings.about.openRelease"),
          onClick: () => openReleases(),
        },
      });
      return;
    }
    toast.success(t("settings.about.installScheduledTitle"), {
      description: t("settings.about.installScheduledDesc"),
      duration: 8_000,
    });
    // Backend quits shortly; no further action.
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.about.installFailedTitle"), {
      description: t("settings.about.installFailedDesc", { msg }),
      action: {
        label: t("settings.about.openRelease"),
        onClick: () => openReleases(),
      },
    });
  } finally {
    installing.value = false;
  }
}
</script>

<template>
  <div class="space-y-7">
    <div class="flex items-start gap-4">
      <div class="brand-mark size-14 shrink-0 rounded-2xl !w-14 !h-14">
        <img
          src="/appicon.png"
          alt=""
          width="56"
          height="56"
          class="size-full"
          draggable="false"
        />
      </div>
      <div class="min-w-0 pt-0.5">
        <h3 class="text-[17px] font-semibold tracking-tight">LRSS</h3>
        <p class="mt-0.5 text-[12.5px] text-muted-foreground">
          {{ t("settings.about.version", { version: APP_VERSION }) }}
        </p>
        <p class="mt-2 text-[13px] leading-relaxed text-muted-foreground">
          {{ t("settings.about.blurb") }}
        </p>
        <p class="mt-1 text-[12px] text-muted-foreground">{{ summary }}</p>
      </div>
    </div>

    <SettingsGroup :title="t('settings.about.resources')">
      <div class="flex flex-wrap gap-2 py-3">
        <Button variant="outline" size="sm" type="button" @click="openDocs">
          {{ t("settings.about.docs") }}
        </Button>
        <Button
          variant="outline"
          size="sm"
          type="button"
          :disabled="busy"
          @click="onCheckUpdate"
        >
          {{ checking ? t("settings.about.checkingUpdate") : t("settings.about.checkUpdate") }}
        </Button>
        <Button
          v-if="pendingUpdate"
          size="sm"
          type="button"
          :disabled="busy || isWebMode()"
          :title="isWebMode() ? t('settings.about.installDesktopOnly') : undefined"
          @click="onInstallUpdate"
        >
          {{
            installing
              ? t("settings.about.installingUpdate")
              : t("settings.about.installUpdate", { version: pendingUpdate.latest })
          }}
        </Button>
        <Button
          v-if="pendingUpdate"
          variant="outline"
          size="sm"
          type="button"
          @click="void openExternalLink(pendingUpdate.htmlUrl, { forceBrowser: true })"
        >
          {{ t("settings.about.openRelease") }}
        </Button>
        <Button
          variant="outline"
          size="sm"
          type="button"
          disabled
          :title="t('settings.unavailable.comingSoon')"
        >
          {{ t("settings.about.licenses") }}
        </Button>
      </div>
      <p class="text-[11.5px] text-muted-foreground">
        {{ t("settings.about.resourcesNote") }}
      </p>
    </SettingsGroup>
  </div>
</template>
