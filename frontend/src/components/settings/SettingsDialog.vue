<script setup lang="ts">
import {
  Bell,
  BookOpenText,
  Brain,
  Filter,
  Info,
  Keyboard,
  Paintbrush,
  RefreshCw,
  Rss,
  Settings2,
  SlidersHorizontal,
  Sparkles,
} from "@lucide/vue";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import type { SettingsSectionId } from "@/types/rss";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import AboutPanel from "@/components/settings/panels/AboutPanel.vue";
import AdvancedPanel from "@/components/settings/panels/AdvancedPanel.vue";
import AppearancePanel from "@/components/settings/panels/AppearancePanel.vue";
import FeedsPanel from "@/components/settings/panels/FeedsPanel.vue";
import FiltersPanel from "@/components/settings/panels/FiltersPanel.vue";
import GeneralPanel from "@/components/settings/panels/GeneralPanel.vue";
import NotificationsPanel from "@/components/settings/panels/NotificationsPanel.vue";
import ReadingPanel from "@/components/settings/panels/ReadingPanel.vue";
import AIFeaturesPanel from "@/components/settings/panels/AIFeaturesPanel.vue";
import SearchAIPanel from "@/components/settings/panels/SearchAIPanel.vue";
import ShortcutsPanel from "@/components/settings/panels/ShortcutsPanel.vue";
import SyncPanel from "@/components/settings/panels/SyncPanel.vue";

const { t } = useI18n();
const { settingsOpen, closeSettings, loadUIPrefs, persistUIPrefs } = useRssStore();

const activeSection = ref<SettingsSectionId>("general");

watch(settingsOpen, (open) => {
  if (open) {
    activeSection.value = "general";
    // Flush pending saves first, then re-sync from SQLite (avoids stale overwrite).
    void (async () => {
      await Promise.resolve(persistUIPrefs(true));
      await loadUIPrefs();
    })();
  } else {
    // Flush any pending debounced writes when leaving settings.
    void persistUIPrefs(true);
  }
});

const navItems = computed(() => {
  const items: {
    id: SettingsSectionId;
    label: string;
    icon: typeof Settings2;
  }[] = [
    { id: "general", label: t("settings.nav.general"), icon: Settings2 },
    { id: "appearance", label: t("settings.nav.appearance"), icon: Paintbrush },
    { id: "reading", label: t("settings.nav.reading"), icon: BookOpenText },
    { id: "feeds", label: t("settings.nav.feeds"), icon: Rss },
    { id: "filters", label: t("settings.nav.filters"), icon: Filter },
    { id: "search_ai", label: t("settings.nav.search_ai"), icon: Brain },
    { id: "ai_features", label: t("settings.nav.ai_features"), icon: Sparkles },
    { id: "sync", label: t("settings.nav.sync"), icon: RefreshCw },
    { id: "shortcuts", label: t("settings.nav.shortcuts"), icon: Keyboard },
    { id: "notifications", label: t("settings.nav.notifications"), icon: Bell },
    { id: "advanced", label: t("settings.nav.advanced"), icon: SlidersHorizontal },
    { id: "about", label: t("settings.nav.about"), icon: Info },
  ];
  return items;
});

const sectionTitle = computed(
  () => navItems.value.find((i) => i.id === activeSection.value)?.label ?? t("settings.title"),
);

const sectionHint = computed(() => {
  const key = `settings.hints.${activeSection.value}` as const;
  return t(key);
});
</script>

<template>
  <Dialog :open="settingsOpen" @update:open="(v) => !v && closeSettings()">
    <DialogContent
      class="flex h-[min(640px,calc(100vh-2rem))] w-full max-w-[min(860px,calc(100vw-2rem))] flex-col gap-0 overflow-hidden p-0 sm:max-w-[min(860px,calc(100vw-2rem))]"
    >
      <DialogTitle class="sr-only">{{ t("settings.title") }}</DialogTitle>
      <DialogDescription class="sr-only">
        {{ t("settings.description") }}
      </DialogDescription>

      <div class="flex min-h-0 flex-1">
        <!-- Left nav -->
        <aside
          class="settings-nav flex min-h-0 w-[200px] shrink-0 flex-col overflow-hidden border-r border-border/70 bg-muted/30"
        >
          <div class="shrink-0 px-4 pt-4 pb-2">
            <p class="text-[13px] font-semibold tracking-tight">{{ t("settings.title") }}</p>
          </div>
          <div class="scroll-pane flex-1 px-2 pb-3">
            <nav class="space-y-0.5" :aria-label="t('settings.navAria')">
              <button
                v-for="item in navItems"
                :key="item.id"
                type="button"
                :class="
                  cn(
                    'nav-row w-full',
                    activeSection === item.id && 'nav-row-active',
                  )
                "
                :aria-current="activeSection === item.id ? 'page' : undefined"
                @click="activeSection = item.id"
              >
                <component :is="item.icon" class="nav-icon" />
                <span class="min-w-0 flex-1 truncate text-left">{{ item.label }}</span>
              </button>
            </nav>
          </div>
        </aside>

        <!-- Right content -->
        <section class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
          <header class="shrink-0 border-b border-border px-6 py-4">
            <div class="flex items-center gap-2">
              <component
                :is="navItems.find((i) => i.id === activeSection)?.icon ?? Sparkles"
                class="size-4 text-primary"
              />
              <h2 class="text-[15px] font-semibold tracking-tight">
                {{ sectionTitle }}
              </h2>
            </div>
            <p class="mt-1 text-[12.5px] text-muted-foreground">{{ sectionHint }}</p>
          </header>

          <div class="scroll-pane flex-1">
            <div class="px-6 py-5">
              <GeneralPanel v-if="activeSection === 'general'" />
              <AppearancePanel v-else-if="activeSection === 'appearance'" />
              <ReadingPanel v-else-if="activeSection === 'reading'" />
              <FeedsPanel v-else-if="activeSection === 'feeds'" />
              <FiltersPanel v-else-if="activeSection === 'filters'" />
              <SearchAIPanel v-else-if="activeSection === 'search_ai'" />
              <AIFeaturesPanel v-else-if="activeSection === 'ai_features'" />
              <SyncPanel v-else-if="activeSection === 'sync'" />
              <ShortcutsPanel v-else-if="activeSection === 'shortcuts'" />
              <NotificationsPanel v-else-if="activeSection === 'notifications'" />
              <AdvancedPanel v-else-if="activeSection === 'advanced'" />
              <AboutPanel v-else-if="activeSection === 'about'" />
            </div>
          </div>
        </section>
      </div>
    </DialogContent>
  </Dialog>
</template>
