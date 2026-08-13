<script setup lang="ts">
// Root shell — routes live under AppLayout.
// Web access with invalid/missing token: only WebAuthBlocked (no library UI).
import { onBeforeMount, ref } from "vue";
import { loadAppsvc } from "@/lib/backend";
import { applyDesktopMica } from "@/lib/mica";
import { isWebMode, webAuthState } from "@/lib/webMode";
import WebAuthBlocked from "@/views/WebAuthBlocked.vue";

const gateReady = ref(false);

onBeforeMount(async () => {
  try {
    await loadAppsvc();
  } finally {
    applyDesktopMica(isWebMode(), {
      enabled: true,
    });
    gateReady.value = true;
  }
});
</script>

<template>
  <div v-if="!gateReady" class="h-full min-h-screen w-full bg-background" />
  <WebAuthBlocked v-else-if="webAuthState === 'unauthorized'" />
  <RouterView v-else />
</template>
