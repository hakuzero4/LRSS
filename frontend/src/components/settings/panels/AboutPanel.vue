<script setup lang="ts">
import { Sparkles } from "@lucide/vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import { openExternalLink } from "@/lib/openLink";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import { Button } from "@/components/ui/button";

const { t } = useI18n();
const { feeds, smartCounts } = useRssStore();

const summary = computed(() =>
  t("settings.about.summary", {
    feeds: feeds.value.length,
    articles: smartCounts.all,
    unread: smartCounts.unread,
  }),
);

function openDocs() {
  void openExternalLink("https://github.com/wailsapp/wails", {
    forceBrowser: true,
  });
}
</script>

<template>
  <div class="space-y-7">
    <div class="flex items-start gap-4">
      <div class="brand-mark size-14 shrink-0 rounded-2xl !w-14 !h-14">
        <Sparkles class="!size-7" />
      </div>
      <div class="min-w-0 pt-0.5">
        <h3 class="text-[17px] font-semibold tracking-tight">LRSS</h3>
        <p class="mt-0.5 text-[12.5px] text-muted-foreground">
          {{ t("settings.about.version", { version: "0.1.0" }) }}
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
          disabled
          :title="t('settings.unavailable.comingSoon')"
        >
          {{ t("settings.about.checkUpdate") }}
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
        {{ t("settings.unavailable.aboutNote") }}
      </p>
    </SettingsGroup>
  </div>
</template>
