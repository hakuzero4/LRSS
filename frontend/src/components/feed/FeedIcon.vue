<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Rss } from "@lucide/vue";
import { cn } from "@/lib/utils";

const props = withDefaults(
  defineProps<{
    src?: string | null;
    title?: string;
    class?: string;
    /** Use slightly larger rounded tile (sidebar vs list). */
    size?: "sm" | "md";
  }>(),
  { size: "sm" },
);

const failed = ref(false);

watch(
  () => props.src,
  () => {
    failed.value = false;
  },
);

const sizeClass = computed(() => (props.size === "md" ? "size-5" : "size-4"));
</script>

<template>
  <img
    v-if="src && !failed"
    :src="src"
    :alt="title ? `${title} icon` : ''"
    loading="lazy"
    decoding="async"
    referrerpolicy="no-referrer"
    :class="
      cn(
        sizeClass,
        'shrink-0 rounded-[3px] object-contain bg-muted/40',
        props.class,
      )
    "
    @error="failed = true"
  />
  <Rss
    v-else
    :class="cn(sizeClass, 'nav-icon shrink-0 text-muted-foreground', props.class)"
    aria-hidden="true"
  />
</template>

