<script setup lang="ts">
defineProps<{
  title: string;
  description?: string;
}>();
</script>

<template>
  <section class="space-y-3">
    <header class="space-y-0.5">
      <h3 class="text-[13px] font-semibold tracking-tight">{{ title }}</h3>
      <p v-if="description" class="text-[12px] leading-relaxed text-muted-foreground">
        {{ description }}
      </p>
    </header>
    <div
      class="divide-y divide-border/70 rounded-xl border border-border/70 bg-card/50 px-3.5 shadow-sm shadow-black/[0.02]"
    >
      <slot />
    </div>
  </section>
</template>
