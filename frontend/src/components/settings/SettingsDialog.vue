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
import { useRssStore } from "@/composables/useRssStore";
import type { SettingsSectionId } from "@/types/rss";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import AboutPanel from "@/components/settings/panels/AboutPanel.vue";
import AdvancedPanel from "@/components/settings/panels/AdvancedPanel.vue";
import AppearancePanel from "@/components/settings/panels/AppearancePanel.vue";
import FeedsPanel from "@/components/settings/panels/FeedsPanel.vue";
import FiltersPanel from "@/components/settings/panels/FiltersPanel.vue";
import GeneralPanel from "@/components/settings/panels/GeneralPanel.vue";
import NotificationsPanel from "@/components/settings/panels/NotificationsPanel.vue";
import ReadingPanel from "@/components/settings/panels/ReadingPanel.vue";
import SearchAIPanel from "@/components/settings/panels/SearchAIPanel.vue";
import ShortcutsPanel from "@/components/settings/panels/ShortcutsPanel.vue";
import SyncPanel from "@/components/settings/panels/SyncPanel.vue";

const { settingsOpen, closeSettings } = useRssStore();

const activeSection = ref<SettingsSectionId>("general");

watch(settingsOpen, (open) => {
  if (open) activeSection.value = "general";
});

const navItems: {
  id: SettingsSectionId;
  label: string;
  icon: typeof Settings2;
}[] = [
  { id: "general", label: "通用", icon: Settings2 },
  { id: "appearance", label: "外观", icon: Paintbrush },
  { id: "reading", label: "阅读", icon: BookOpenText },
  { id: "feeds", label: "订阅", icon: Rss },
  { id: "filters", label: "过滤规则", icon: Filter },
  { id: "search_ai", label: "搜索 / AI", icon: Brain },
  { id: "sync", label: "同步", icon: RefreshCw },
  { id: "shortcuts", label: "快捷键", icon: Keyboard },
  { id: "notifications", label: "通知", icon: Bell },
  { id: "advanced", label: "高级", icon: SlidersHorizontal },
  { id: "about", label: "关于", icon: Info },
];

const sectionTitle = computed(
  () => navItems.find((i) => i.id === activeSection.value)?.label ?? "设置",
);

const sectionHint = computed(() => {
  const map: Record<SettingsSectionId, string> = {
    general: "刷新、已读行为与启动选项。",
    appearance: "主题、强调色与界面密度。",
    reading: "正文字号、宽度与列表过滤。",
    feeds: "订阅默认项与文章保留。",
    filters: "重复标题与关键词屏蔽。",
    search_ai: "向量模型与语义搜索（未配置则全文检索）。",
    sync: "跨设备同步（预览）。",
    shortcuts: "键盘操作一览。",
    notifications: "新文章提醒。",
    advanced: "性能与诊断。",
    about: "版本与资源。",
  };
  return map[activeSection.value];
});
</script>

<template>
  <Dialog :open="settingsOpen" @update:open="(v) => !v && closeSettings()">
    <DialogContent
      class="flex h-[min(640px,calc(100vh-2rem))] w-full max-w-[min(860px,calc(100vw-2rem))] flex-col gap-0 overflow-hidden p-0 sm:max-w-[min(860px,calc(100vw-2rem))]"
    >
      <DialogTitle class="sr-only">设置</DialogTitle>
      <DialogDescription class="sr-only">
        应用程序偏好设置。左侧选择分类，右侧调整选项。
      </DialogDescription>

      <div class="flex min-h-0 flex-1">
        <!-- Left nav -->
        <aside
          class="settings-nav flex w-[200px] shrink-0 flex-col border-r border-border/70 bg-muted/30"
        >
          <div class="px-4 pt-4 pb-2">
            <p class="text-[13px] font-semibold tracking-tight">设置</p>
          </div>
          <ScrollArea class="flex-1 px-2 pb-3">
            <nav class="space-y-0.5" aria-label="设置分类">
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
          </ScrollArea>
        </aside>

        <!-- Right content -->
        <section class="flex min-w-0 flex-1 flex-col bg-background">
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

          <ScrollArea class="flex-1">
            <div class="px-6 py-5">
              <GeneralPanel v-if="activeSection === 'general'" />
              <AppearancePanel v-else-if="activeSection === 'appearance'" />
              <ReadingPanel v-else-if="activeSection === 'reading'" />
              <FeedsPanel v-else-if="activeSection === 'feeds'" />
              <FiltersPanel v-else-if="activeSection === 'filters'" />
              <SearchAIPanel v-else-if="activeSection === 'search_ai'" />
              <SyncPanel v-else-if="activeSection === 'sync'" />
              <ShortcutsPanel v-else-if="activeSection === 'shortcuts'" />
              <NotificationsPanel v-else-if="activeSection === 'notifications'" />
              <AdvancedPanel v-else-if="activeSection === 'advanced'" />
              <AboutPanel v-else-if="activeSection === 'about'" />
            </div>
          </ScrollArea>
        </section>
      </div>
    </DialogContent>
  </Dialog>
</template>
