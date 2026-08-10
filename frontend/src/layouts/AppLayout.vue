<script setup lang="ts">
import { onMounted } from "vue";
import AppSidebar from "@/components/layout/AppSidebar.vue";
import AddFeedDialog from "@/components/feed/AddFeedDialog.vue";
import SettingsDialog from "@/components/settings/SettingsDialog.vue";
import { useTheme } from "@/composables/useTheme";
import { useKeyboardShortcuts } from "@/composables/useKeyboardShortcuts";
import { Toaster } from "@/components/ui/sonner";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";

useTheme();
useKeyboardShortcuts();

onMounted(() => {
  document.documentElement.classList.add("h-full");
  document.body.classList.add("h-full", "overflow-hidden");
});
</script>

<template>
  <div class="app-shell flex h-screen w-screen overflow-hidden bg-background text-foreground">
    <ResizablePanelGroup
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
    <SettingsDialog />
    <Toaster position="top-center" rich-colors close-button />
  </div>
</template>
