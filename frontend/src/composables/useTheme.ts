import { onMounted, onUnmounted, ref, watch } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import { applyAccentToDocument } from "@/lib/accent";
import { applyLiquidGlass } from "@/lib/liquidGlassRuntime";
import { applyDesktopMica } from "@/lib/mica";
import { isWebMode } from "@/lib/webMode";

const media = window.matchMedia("(prefers-color-scheme: dark)");
const reduceTransparency = window.matchMedia("(prefers-reduced-transparency: reduce)");

function resolveDark(theme: "system" | "light" | "dark") {
  if (theme === "system") return media.matches;
  return theme === "dark";
}

export function useTheme() {
  const { settings } = useRssStore();
  const isDark = ref(resolveDark(settings.theme));

  function applyMicaClass() {
    applyDesktopMica(isWebMode(), {
      enabled: settings.micaBackdrop,
      hardwareAcceleration: settings.hardwareAcceleration,
    });
  }

  function applyGlassClass() {
    applyLiquidGlass({
      prefEnabled: settings.liquidGlass,
      hardwareAcceleration: settings.hardwareAcceleration,
    });
  }

  function apply() {
    isDark.value = resolveDark(settings.theme);
    document.documentElement.classList.toggle("dark", isDark.value);
    applyAccentToDocument(settings.accent, isDark.value);
    applyMicaClass();
    applyGlassClass();
  }

  function onSystemChange() {
    if (settings.theme === "system") apply();
  }

  function onTransparencyChange() {
    applyMicaClass();
    applyGlassClass();
  }

  onMounted(() => {
    apply();
    media.addEventListener("change", onSystemChange);
    reduceTransparency.addEventListener("change", onTransparencyChange);
  });

  onUnmounted(() => {
    media.removeEventListener("change", onSystemChange);
    reduceTransparency.removeEventListener("change", onTransparencyChange);
  });

  watch(() => settings.theme, apply);
  watch(() => settings.accent, apply);
  watch(() => [settings.micaBackdrop, settings.hardwareAcceleration], applyMicaClass);
  watch(() => [settings.liquidGlass, settings.hardwareAcceleration], applyGlassClass);

  return { isDark, apply };
}
