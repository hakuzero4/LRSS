<script lang="ts" setup>
import type { ToasterProps } from "vue-sonner";

import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
  XIcon,
} from "@lucide/vue";
import { reactiveOmit } from "@vueuse/core";
import { Toaster as Sonner } from "vue-sonner";
import { cn } from "@/lib/utils";

const props = withDefaults(defineProps<ToasterProps>(), {
  theme: "system",
  position: "top-center",
  closeButton: true,
  // vue-sonner defaults to top-left; put dismiss control on the right.
  closeButtonPosition: "top-right",
  richColors: true,
  expand: false,
  visibleToasts: 3,
  gap: 10,
  offset: 16,
});

const delegatedProps = reactiveOmit(props, "class", "toastOptions");
</script>

<template>
  <Sonner
    :class="cn('toaster group pointer-events-auto', props.class)"
    :style="{
      '--normal-bg': 'var(--popover)',
      '--normal-text': 'var(--popover-foreground)',
      '--normal-border': 'var(--border)',
      '--border-radius': 'var(--radius)',
    }"
    :toast-options="{
      ...(props.toastOptions ?? {}),
      classes: {
        toast: cn(
          'group toast group-[.toaster]:bg-popover group-[.toaster]:text-popover-foreground',
          'group-[.toaster]:border-border group-[.toaster]:shadow-lg',
          'group-[.toaster]:rounded-xl group-[.toaster]:border',
          'group-[.toaster]:px-4 group-[.toaster]:py-3.5',
          'group-[.toaster]:min-w-[280px] group-[.toaster]:max-w-[min(420px,calc(100vw-2rem))]',
          'group-[.toaster]:gap-2',
        ),
        title: 'text-[13.5px] font-semibold tracking-tight text-foreground',
        description: 'text-[12.5px] text-muted-foreground leading-relaxed',
        actionButton:
          'group-[.toast]:bg-primary group-[.toast]:text-primary-foreground',
        cancelButton:
          'group-[.toast]:bg-muted group-[.toast]:text-muted-foreground',
        closeButton:
          'group-[.toast]:border-border group-[.toast]:bg-background group-[.toast]:text-foreground/70',
        success: 'group-[.toaster]:border-emerald-500/25',
        error: 'group-[.toaster]:border-destructive/30',
        info: 'group-[.toaster]:border-primary/20',
        ...(props.toastOptions?.classes ?? {}),
      },
    }"
    v-bind="delegatedProps"
  >
    <template #success-icon>
      <CircleCheckIcon class="size-4 text-emerald-600 dark:text-emerald-400" />
    </template>
    <template #info-icon>
      <InfoIcon class="size-4 text-primary" />
    </template>
    <template #warning-icon>
      <TriangleAlertIcon class="size-4 text-amber-600 dark:text-amber-400" />
    </template>
    <template #error-icon>
      <OctagonXIcon class="size-4 text-destructive" />
    </template>
    <template #loading-icon>
      <Loader2Icon class="size-4 animate-spin text-muted-foreground" />
    </template>
    <template #close-icon>
      <XIcon class="size-3.5" />
    </template>
  </Sonner>
</template>
