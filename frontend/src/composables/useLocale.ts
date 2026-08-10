import { computed } from "vue";
import { useI18n } from "vue-i18n";
import {
  LOCALE_STORAGE_KEY,
  SUPPORTED_LOCALES,
  type AppLocale,
  isAppLocale,
  resolveLocale,
} from "@/i18n";

/**
 * App locale: get/set, persist to localStorage (`lrss.locale`),
 * and re-detect from navigator when needed.
 */
export function useLocale() {
  const { locale, t, te } = useI18n();

  const currentLocale = computed<AppLocale>({
    get: () => (isAppLocale(locale.value) ? locale.value : resolveLocale(locale.value)),
    set: (value) => setLocale(value),
  });

  function setLocale(next: AppLocale | string) {
    const resolved = isAppLocale(next) ? next : resolveLocale(next);
    locale.value = resolved;
    try {
      localStorage.setItem(LOCALE_STORAGE_KEY, resolved);
    } catch {
      /* ignore quota / private mode */
    }
    if (typeof document !== "undefined") {
      document.documentElement.lang = resolved;
    }
  }

  /** Re-read preference: storage first, else navigator (zh* → zh-CN, else en-US). */
  function detectAndApply() {
    try {
      const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
      if (stored && isAppLocale(stored)) {
        setLocale(stored);
        return stored;
      }
    } catch {
      /* ignore */
    }
    const nav =
      typeof navigator !== "undefined"
        ? navigator.language || (navigator as { userLanguage?: string }).userLanguage
        : undefined;
    const detected = resolveLocale(nav);
    setLocale(detected);
    return detected;
  }

  return {
    locale: currentLocale,
    currentLocale,
    setLocale,
    detectAndApply,
    supportedLocales: SUPPORTED_LOCALES,
    storageKey: LOCALE_STORAGE_KEY,
    t,
    te,
  };
}
