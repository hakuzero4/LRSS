import { createI18n } from "vue-i18n";
import enUS from "./locales/en-US";
import zhCN from "./locales/zh-CN";

export const LOCALE_STORAGE_KEY = "lrss.locale";

export type AppLocale = "zh-CN" | "en-US";

export const SUPPORTED_LOCALES: AppLocale[] = ["zh-CN", "en-US"];

export function isAppLocale(value: string): value is AppLocale {
  return value === "zh-CN" || value === "en-US";
}

/** Map navigator / stored language tags to a supported app locale. */
export function resolveLocale(tag?: string | null): AppLocale {
  if (tag && isAppLocale(tag)) return tag;
  const raw = (tag ?? "").toLowerCase();
  if (raw.startsWith("zh")) return "zh-CN";
  if (raw.startsWith("en")) return "en-US";
  return "zh-CN";
}

export function detectInitialLocale(): AppLocale {
  try {
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
    if (stored && isAppLocale(stored)) return stored;
  } catch {
    /* ignore */
  }
  const nav =
    typeof navigator !== "undefined"
      ? navigator.language || (navigator as { userLanguage?: string }).userLanguage
      : undefined;
  return resolveLocale(nav);
}

/**
 * Apply UI language (vue-i18n + localStorage + document.lang).
 * Safe outside setup(); used when loading UIPrefs from backend / Web access.
 */
export function applyAppLocale(next: string | null | undefined): AppLocale {
  const resolved = resolveLocale(next);
  try {
    i18n.global.locale.value = resolved;
  } catch {
    /* i18n not ready */
  }
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, resolved);
  } catch {
    /* ignore */
  }
  if (typeof document !== "undefined") {
    document.documentElement.lang = resolved;
  }
  return resolved;
}

const i18n = createI18n({
  legacy: false,
  locale: detectInitialLocale(),
  fallbackLocale: "zh-CN",
  messages: {
    "zh-CN": zhCN,
    "en-US": enUS,
  },
});

export default i18n;
