<script setup lang="ts">
import { onMounted } from "vue";
import { useI18n } from "vue-i18n";
import AppSidebar from "@/components/layout/AppSidebar.vue";
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
const { bootstrapError, backendReady, reloadLibrary, zenMode } = useRssStore();

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
  <div class="app-shell flex h-screen w-screen overflow-hidden bg-background text-foreground">
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

    <!-- Zen: reader only (sidebar + list hidden in ReaderView) -->
    <template v-if="zenMode">
      <main class="flex h-full min-h-0 min-w-0 w-full flex-1 flex-col overflow-hidden">
        <RouterView />
      </main>
    </template>

    <ResizablePanelGroup
      v-else
      id="lrss-shell"
      direction="horizontal"
      auto-save-id="lrss-shell"
      class="h-full w-full"
    >
      <!-- Left: feed sidebar -->
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

      <!-- Center + right: routed reader (list + article) -->
      <ResizablePanel id="main" :default-size="82" :min-size="40" class="min-w-0">
        <main class="flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
          <RouterView />
        </main>
      </ResizablePanel>
    </ResizablePanelGroup>

    <AddFeedDialog />
    <FeedEditDialog />
    <SettingsDialog />
    <Toaster
      class="pointer-events-auto"
      position="top-center"
      close-button-position="top-right"
      :rich-colors="true"
      :close-button="true"
    />
  </div>
</template>
