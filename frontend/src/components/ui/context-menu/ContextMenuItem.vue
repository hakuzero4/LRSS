<script setup lang="ts">
import type { ContextMenuItemEmits, ContextMenuItemProps } from "reka-ui";
import type { HTMLAttributes } from "vue";
import { reactiveOmit } from "@vueuse/core";
import { ContextMenuItem, useForwardPropsEmits } from "reka-ui";
import { cn } from "@/lib/utils";

const props = withDefaults(
  defineProps<
    ContextMenuItemProps & {
      class?: HTMLAttributes["class"];
      inset?: boolean;
      variant?: "default" | "destructive";
    }
  >(),
  { variant: "default" },
);
const emits = defineEmits<ContextMenuItemEmits>();
const delegatedProps = reactiveOmit(props, "class", "inset", "variant");
const forwarded = useForwardPropsEmits(delegatedProps, emits);
</script>

<template>
  <ContextMenuItem
    data-slot="context-menu-item"
    :data-inset="inset ? '' : undefined"
    :data-variant="variant"
    v-bind="forwarded"
    :class="
      cn(
        'relative flex cursor-default select-none items-center gap-1.5 rounded-md px-2 py-1.5 text-[13px] outline-hidden',
        'focus:bg-accent focus:text-accent-foreground',
        'data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
        'data-[variant=destructive]:text-destructive data-[variant=destructive]:focus:bg-destructive/10 data-[variant=destructive]:focus:text-destructive',
        '[&_svg]:pointer-events-none [&_svg:not([class*=size-])]:size-4 [&_svg]:shrink-0',
        props.class,
      )
    "
  >
    <slot />
  </ContextMenuItem>
</template>
