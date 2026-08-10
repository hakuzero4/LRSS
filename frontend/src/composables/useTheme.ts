import { onMounted, onUnmounted, ref, watch } from "vue";
import { useRssStore } from "@/composables/useRssStore";

const media = window.matchMedia("(prefers-color-scheme: dark)");

function resolveDark(theme: "system" | "light" | "dark") {
  if (theme === "system") return media.matches;
  return theme === "dark";
}

export function useTheme() {
  const { settings } = useRssStore();
  const isDark = ref(resolveDark(settings.theme));

  function apply() {
    isDark.value = resolveDark(settings.theme);
    document.documentElement.classList.toggle("dark", isDark.value);
  }

  function onSystemChange() {
    if (settings.theme === "system") apply();
  }

  onMounted(() => {
    apply();
    media.addEventListener("change", onSystemChange);
  });

  onUnmounted(() => {
    media.removeEventListener("change", onSystemChange);
  });

  watch(() => settings.theme, apply);

  return { isDark, apply };
}
