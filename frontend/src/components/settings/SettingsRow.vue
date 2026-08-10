<script setup lang="ts">
defineProps<{
  title: string;
  description?: string;
}>();
</script>

<template>
  <div class="settings-row gap-4 py-1">
    <div class="min-w-0 flex-1">
      <p class="text-[13px] font-medium leading-snug">{{ title }}</p>
      <p v-if="description" class="mt-0.5 text-[12px] leading-relaxed text-muted-foreground">
        {{ description }}
      </p>
    </div>
    <div class="flex shrink-0 items-center justify-end">
      <slot />
    </div>
  </div>
</template>
