<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import {
  ACCENT_PRESET_IDS,
  ACCENT_PRESETS,
  type AccentPresetId,
  isAccentPresetId,
  isHexColor,
  normalizeAccent,
  resolveAccentHex,
} from "@/lib/accent";
import { cn } from "@/lib/utils";
import { Input } from "@/components/ui/input";

const props = defineProps<{
  /** Preset id (purple|…) or #rrggbb */
  modelValue: string;
  /** Used to preview preset light/dark variants */
  isDark?: boolean;
}>();

const emit = defineEmits<{
  "update:modelValue": [value: string];
}>();

const { t } = useI18n();

const value = computed({
  get: () => props.modelValue,
  set: (v: string) => emit("update:modelValue", normalizeAccent(v, props.modelValue)),
});

const resolvedHex = computed(() =>
  resolveAccentHex(value.value, !!props.isDark),
);

const hexInput = ref(resolvedHex.value);

watch(
  resolvedHex,
  (h) => {
    if (hexInput.value.toLowerCase() !== h.toLowerCase()) {
      hexInput.value = h;
    }
  },
  { immediate: true },
);

function selectPreset(id: AccentPresetId) {
  value.value = id;
}

function onNativeColor(ev: Event) {
  const el = ev.target as HTMLInputElement;
  if (el?.value) {
    value.value = el.value.toLowerCase();
  }
}

function onHexBlur() {
  let raw = hexInput.value.trim();
  if (!raw.startsWith("#")) raw = `#${raw}`;
  if (isHexColor(raw) || /^#[0-9a-fA-F]{3}$/.test(raw)) {
    value.value = normalizeAccent(raw);
  } else {
    hexInput.value = resolvedHex.value;
  }
}

function isSelectedPreset(id: AccentPresetId) {
  return value.value === id;
}

function isCustomSelected() {
  return isHexColor(normalizeAccent(value.value)) && !isAccentPresetId(value.value);
}
</script>

<template>
  <div class="space-y-3">
    <div class="flex flex-wrap items-center gap-2.5">
      <button
        v-for="id in ACCENT_PRESET_IDS"
        :key="id"
        type="button"
        :title="t(ACCENT_PRESETS[id].labelKey)"
        :aria-label="t(ACCENT_PRESETS[id].labelKey)"
        :aria-pressed="isSelectedPreset(id)"
        :class="
          cn(
            'relative size-8 rounded-full border-2 shadow-sm transition-transform',
            'hover:scale-105 active:scale-95 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            isSelectedPreset(id)
              ? 'border-foreground ring-2 ring-foreground/20'
              : 'border-transparent',
          )
        "
        :style="{
          backgroundColor: isDark ? ACCENT_PRESETS[id].dark : ACCENT_PRESETS[id].light,
        }"
        @click="selectPreset(id)"
      />

      <!-- Custom color: native picker + swatch -->
      <label
        :class="
          cn(
            'relative size-8 cursor-pointer overflow-hidden rounded-full border-2 shadow-sm transition-transform',
            'hover:scale-105 active:scale-95 focus-within:ring-2 focus-within:ring-ring',
            isCustomSelected()
              ? 'border-foreground ring-2 ring-foreground/20'
              : 'border-border',
          )
        "
        :title="t('settings.appearance.accentCustom')"
        :aria-label="t('settings.appearance.accentCustom')"
      >
        <span
          class="absolute inset-0"
          :style="{
            background: isCustomSelected()
              ? resolvedHex
              : 'conic-gradient(from 0deg, #f00, #ff0, #0f0, #0ff, #00f, #f0f, #f00)',
          }"
        />
        <input
          type="color"
          class="absolute inset-0 h-full w-full cursor-pointer opacity-0"
          :value="resolvedHex"
          @input="onNativeColor"
        />
      </label>
    </div>

    <div class="flex items-center gap-2">
      <span
        class="size-6 shrink-0 rounded-md border border-border shadow-inner"
        :style="{ backgroundColor: resolvedHex }"
        aria-hidden="true"
      />
      <Input
        v-model="hexInput"
        class="h-8 max-w-[7.5rem] font-mono text-[12.5px] uppercase"
        spellcheck="false"
        maxlength="7"
        :aria-label="t('settings.appearance.accentHex')"
        @blur="onHexBlur"
        @keydown.enter="onHexBlur"
      />
      <span class="text-[12px] text-muted-foreground">
        {{
          isAccentPresetId(value)
            ? t(ACCENT_PRESETS[value].labelKey)
            : t("settings.appearance.accentCustom")
        }}
      </span>
    </div>
  </div>
</template>
