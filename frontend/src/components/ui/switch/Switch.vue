<script setup lang="ts">
import type { SwitchRootEmits, SwitchRootProps } from "reka-ui";
import type { HTMLAttributes } from "vue";
import { computed } from "vue";
import { reactiveOmit } from "@vueuse/core";
import { SwitchRoot, SwitchThumb, useForwardPropsEmits } from "reka-ui";
import { cn } from "@/lib/utils";

/**
 * Reka Switch uses modelValue / update:modelValue.
 * Call sites in this project historically used checked / update:checked
 * (Radix-style). Support both so toggles actually write back to settings.
 */
const props = withDefaults(
  defineProps<
    SwitchRootProps & {
      class?: HTMLAttributes["class"];
      size?: "sm" | "default";
      /** Alias for modelValue (legacy API used across settings panels). */
      checked?: boolean | null;
    }
  >(),
  {
    size: "default",
  },
);

const emits = defineEmits<
  SwitchRootEmits & {
    "update:checked": [payload: boolean];
  }
>();

const delegatedProps = reactiveOmit(props, "class", "size", "checked", "modelValue");

const forwarded = useForwardPropsEmits(delegatedProps, emits);

const model = computed<boolean>({
  get() {
    if (props.modelValue !== undefined && props.modelValue !== null) {
      return props.modelValue === (props.trueValue ?? true);
    }
    if (props.checked !== undefined && props.checked !== null) {
      return !!props.checked;
    }
    return false;
  },
  set(v: boolean) {
    const trueV = (props.trueValue ?? true) as boolean;
    const falseV = (props.falseValue ?? false) as boolean;
    const next = v ? trueV : falseV;
    emits("update:modelValue", next as never);
    emits("update:checked", !!v);
  },
});
</script>

<template>
  <SwitchRoot
    v-slot="slotProps"
    data-slot="switch"
    :data-size="size"
    v-bind="forwarded"
    :model-value="model"
    @update:model-value="(v) => (model = v === (props.trueValue ?? true))"
    :class="
      cn(
        'data-checked:bg-primary data-unchecked:bg-input focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive dark:aria-invalid:border-destructive/50 dark:data-unchecked:bg-input/80 shrink-0 rounded-full border border-transparent focus-visible:ring-3 aria-invalid:ring-3 data-[size=default]:h-[18.4px] data-[size=default]:w-8 data-[size=sm]:h-3.5 data-[size=sm]:w-6 peer group/switch relative inline-flex items-center transition-all outline-none after:absolute after:-inset-x-3 after:-inset-y-2 data-disabled:cursor-not-allowed data-disabled:opacity-50',
        props.class,
      )
    "
  >
    <SwitchThumb
      data-slot="switch-thumb"
      class="bg-background dark:data-unchecked:bg-foreground dark:data-checked:bg-primary-foreground rounded-full group-data-[size=default]/switch:size-4 group-data-[size=sm]/switch:size-3 group-data-[size=default]/switch:data-checked:translate-x-[calc(100%-2px)] group-data-[size=sm]/switch:data-checked:translate-x-[calc(100%-2px)] group-data-[size=default]/switch:data-unchecked:translate-x-0 group-data-[size=sm]/switch:data-unchecked:translate-x-0 pointer-events-none block ring-0 transition-transform"
    >
      <slot name="thumb" v-bind="slotProps" />
    </SwitchThumb>
  </SwitchRoot>
</template>
