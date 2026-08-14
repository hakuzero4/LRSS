<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import { startLiquidGlassRuntime, stopLiquidGlassRuntime } from "@/lib/liquidGlassRuntime";

const { settings } = useRssStore();
const root = ref<HTMLElement | null>(null);

function sync() {
  if (!root.value) return;
  startLiquidGlassRuntime(root.value, {
    prefEnabled: settings.liquidGlass,
    hardwareAcceleration: settings.hardwareAcceleration,
  });
}

onMounted(sync);
onUnmounted(() => {
  stopLiquidGlassRuntime();
});

watch(
  () => [settings.liquidGlass, settings.hardwareAcceleration],
  sync,
);
</script>

<template>
  <div ref="root" class="lg-host" aria-hidden="true" />
</template>
