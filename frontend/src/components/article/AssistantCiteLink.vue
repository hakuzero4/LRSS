<script setup lang="ts">
import { inject, computed } from "vue";
import { citeNFromHref } from "@/lib/chatMarkdown";

const OPEN_CITE = "lrssOpenCite";

const props = defineProps<{
  node?: { url?: string };
}>();

const openCite = inject<(n: number) => void>(OPEN_CITE, () => {});
const citeN = computed(() => citeNFromHref(String(props.node?.url ?? "")));

function onClick(ev: MouseEvent) {
  ev.preventDefault();
  ev.stopPropagation();
  if (citeN.value != null) openCite(citeN.value);
}
</script>

<template>
  <button
    v-if="citeN != null"
    type="button"
    class="chat-cite"
    @click="onClick"
  >
    [{{ citeN }}]
  </button>
  <span v-else class="text-primary">
    <slot />
  </span>
</template>
