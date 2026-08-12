<script setup lang="ts">
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { APP_VERSION, GITHUB_REPO_URL, GITHUB_RELEASES_URL } from "@/lib/appMeta";
import { checkForUpdate } from "@/lib/checkUpdate";
import { openExternalLink } from "@/lib/openLink";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import { Button } from "@/components/ui/button";

const { t } = useI18n();
const { feeds, smartCounts } = useRssStore();

const checking = ref(false);

const summary = computed(() =>
  t("settings.about.summary", {
    feeds: feeds.value.length,
    articles: smartCounts.all,
    unread: smartCounts.unread,
  }),
);

function openDocs() {
  void openExternalLink(GITHUB_REPO_URL, { forceBrowser: true });
}

function openReleases() {
  void openExternalLink(GITHUB_RELEASES_URL, { forceBrowser: true });
}

async function onCheckUpdate() {
  if (checking.value) return;
  checking.value = true;
  try {
    const result = await checkForUpdate();
    if (result.status === "upToDate") {
      toast.success(t("settings.about.updateUpToDateTitle"), {
        description: t("settings.about.updateUpToDateDesc", {
          version: result.current,
        }),
      });
      return;
    }
    if (result.status === "updateAvailable") {
      toast.message(t("settings.about.updateAvailableTitle"), {
        description: t("settings.about.updateAvailableDesc", {
          current: result.current,
          latest: result.latest,
        }),
        action: {
          label: t("settings.about.openRelease"),
          onClick: () => {
            void openExternalLink(result.htmlUrl, { forceBrowser: true });
          },
        },
        duration: 12_000,
      });
      return;
    }
    // error
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
          :disabled="checking"
          @click="onCheckUpdate"
        >
          {{ checking ? t("settings.about.checkingUpdate") : t("settings.about.checkUpdate") }}
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
