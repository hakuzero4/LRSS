<script setup lang="ts">
import { onMounted } from "vue";
import { useI18n } from "vue-i18n";
import AppSidebar from "@/components/layout/AppSidebar.vue";
import ActivityBar from "@/components/layout/ActivityBar.vue";
import AssistantPane from "@/components/article/AssistantPane.vue";
import { LiquidGlassHost } from "@/components/ui/liquid-glass";
import AddFeedDialog from "@/components/feed/AddFeedDialog.vue";
import FeedEditDialog from "@/components/feed/FeedEditDialog.vue";
import SettingsDialog from "@/components/settings/SettingsDialog.vue";
import { useTheme } from "@/composables/useTheme";
import { useKeyboardShortcuts } from "@/composables/useKeyboardShortcuts";
import { useRssStore } from "@/composables/useRssStore";
import { Button } from "@/components/ui/button";
import { Toaster } from "@/components/ui/sonner";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";

const { t } = useI18n();
const { bootstrapError, backendReady, reloadLibrary, zenMode, webMode, assistant } = useRssStore();

useTheme();
useKeyboardShortcuts();

onMounted(() => {
  document.documentElement.classList.add("h-full");
  document.body.classList.add("h-full", "overflow-hidden");
});

async function onRetryBootstrap() {
  await reloadLibrary();
}
</script>

<template>
  <div class="app-shell lg-scene flex h-screen w-screen flex-col overflow-hidden bg-background text-foreground">
    <LiquidGlassHost />
    <div
      v-if="bootstrapError && !backendReady"
      class="absolute top-0 right-0 left-0 z-50 flex items-center justify-center gap-3 border-b border-destructive/30 bg-destructive/10 px-4 py-2 text-[12.5px] text-destructive"
      role="alert"
    >
      <span class="min-w-0 truncate font-medium">
        {{ t("shell.backendError") }}
        —
        {{ t("shell.backendErrorHint", { msg: bootstrapError }) }}
      </span>
      <Button type="button" size="sm" variant="outline" class="h-7 shrink-0" @click="onRetryBootstrap">
        {{ t("shell.retry") }}
      </Button>
    </div>

    <!-- Columns share one baseline. Status strip is window chrome, not a fourth card. -->
    <div class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <template v-if="zenMode">
        <div class="flex min-h-0 min-w-0 w-full flex-1 overflow-hidden">
          <main class="main-stage flex min-h-0 min-w-0 w-full flex-1 flex-col overflow-hidden">
            <RouterView />
          </main>
          <aside
            v-if="assistant.open"
            class="min-h-0 w-[min(36%,28rem)] min-w-[16rem] max-w-[42%] shrink-0 border-l border-border/70"
          >
            <AssistantPane />
          </aside>
        </div>
      </template>

      <ResizablePanelGroup
        v-else
        id="lrss-shell"
        direction="horizontal"
        auto-save-id="lrss-shell"
        class="min-h-0 w-full flex-1"
      >
        <ResizablePanel
          id="sidebar"
          :default-size="18"
          :min-size="12"
          :max-size="36"
          class="min-w-0"
        >
          <AppSidebar />
        </ResizablePanel>

        <ResizableHandle with-handle />

        <ResizablePanel
          id="main"
          :default-size="assistant.open ? 54 : 82"
          :min-size="36"
          class="min-w-0"
        >
          <main class="main-stage flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
            <RouterView />
          </main>
        </ResizablePanel>

        <template v-if="assistant.open">
          <ResizableHandle with-handle />
          <ResizablePanel
            id="assistant"
            :default-size="28"
            :min-size="18"
            :max-size="42"
            class="min-w-0"
          >
            <AssistantPane />
          </ResizablePanel>
        </template>
      </ResizablePanelGroup>

      <ActivityBar />
    </div>

    <!-- Settings first; nested feed dialogs after so equal-z portals still paint on top. -->
    <SettingsDialog v-if="!webMode" />
    <AddFeedDialog v-if="!webMode" />
    <FeedEditDialog v-if="!webMode" />
    <Toaster
      class="pointer-events-auto"
      position="top-center"
      close-button-position="top-right"
      :rich-colors="true"
      :close-button="true"
    />
  </div>
</template>
