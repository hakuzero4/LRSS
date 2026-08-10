<script setup lang="ts">
import { onMounted } from "vue";
import AppSidebar from "@/components/layout/AppSidebar.vue";
import AddFeedDialog from "@/components/feed/AddFeedDialog.vue";
import SettingsDialog from "@/components/settings/SettingsDialog.vue";
import { useTheme } from "@/composables/useTheme";
import { useKeyboardShortcuts } from "@/composables/useKeyboardShortcuts";
import { Toaster } from "@/components/ui/sonner";

useTheme();
useKeyboardShortcuts();

onMounted(() => {
  document.documentElement.classList.add("h-full");
  document.body.classList.add("h-full", "overflow-hidden");
});
</script>

<template>
  <div class="app-shell flex h-screen w-screen overflow-hidden bg-background text-foreground">
    <AppSidebar />
    <main class="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden">
      <RouterView />
    </main>
    <AddFeedDialog />
    <SettingsDialog />
    <Toaster position="top-center" rich-colors close-button />
  </div>
</template>
