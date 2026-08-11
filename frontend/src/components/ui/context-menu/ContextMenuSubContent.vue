<script setup lang="ts">
import type { ContextMenuSubContentEmits, ContextMenuSubContentProps } from "reka-ui";
import type { HTMLAttributes } from "vue";
import { reactiveOmit } from "@vueuse/core";
import { ContextMenuSubContent, useForwardPropsEmits } from "reka-ui";
import { cn } from "@/lib/utils";

const props = defineProps<
  ContextMenuSubContentProps & { class?: HTMLAttributes["class"] }
>();
const emits = defineEmits<ContextMenuSubContentEmits>();

const delegatedProps = reactiveOmit(props, "class");
const forwarded = useForwardPropsEmits(delegatedProps, emits);
</script>

<template>
  <ContextMenuSubContent
    data-slot="context-menu-sub-content"
    v-bind="forwarded"
    :class="
      cn(
        'z-50 min-w-[10rem] max-h-[min(320px,50vh)] overflow-y-auto overflow-x-hidden rounded-lg p-1 shadow-lg ring-1 ring-foreground/10',
        'bg-popover text-popover-foreground',
        'data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0',
        'data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
        'data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2',
        props.class,
      )
    "
  >
    <slot />
  </ContextMenuSubContent>
</template>
