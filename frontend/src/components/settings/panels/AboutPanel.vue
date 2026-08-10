<script setup lang="ts">
import { Sparkles } from "@lucide/vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import { Button } from "@/components/ui/button";

const { t } = useI18n();
const { feeds, articles, smartCounts } = useRssStore();

const summary = computed(() =>
  t("settings.about.summary", {
    feeds: feeds.value.length,
    articles: articles.value.length,
    unread: smartCounts.value.unread,
  }),
);
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
        <Button variant="outline" size="sm" type="button">
          {{ t("settings.about.docs") }}
        </Button>
        <Button variant="outline" size="sm" type="button">
          {{ t("settings.about.checkUpdate") }}
        </Button>
        <Button variant="outline" size="sm" type="button">
          {{ t("settings.about.licenses") }}
        </Button>
      </div>
    </SettingsGroup>
  </div>
</template>
