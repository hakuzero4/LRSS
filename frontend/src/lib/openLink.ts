/**
 * Open a URL according to Reading settings (openLinksInBrowser).
 * Prefers Wails `Browser.OpenURL` (system browser); falls back to window.open.
 */

import { normalizeOpenableUrl } from "./readingSettings";

export type OpenExternalResult =
  | { ok: false; reason: "empty" | "blocked" }
  | { ok: true; method: "system" | "window"; href: string };

export type OpenExternalDeps = {
  /** Injected for tests; defaults to Wails Browser.OpenURL when available. */
  openSystemBrowser?: (url: string) => Promise<void>;
  /** Injected for tests; defaults to window.open. */
  openWindow?: (url: string, target?: string, features?: string) => void;
};

/**
 * @param opts.forceBrowser - when true (default), try system browser first.
 *   Mirrors settings.openLinksInBrowser from call sites.
 */
export async function openExternalLink(
  url: string,
  opts?: { forceBrowser?: boolean },
  deps?: OpenExternalDeps,
): Promise<OpenExternalResult> {
  const href = normalizeOpenableUrl(url);
  if (!href) {
    return { ok: false, reason: /^\s*javascript:/i.test(String(url ?? "")) ? "blocked" : "empty" };
  }

  const useBrowser = opts?.forceBrowser !== false;

  if (useBrowser) {
    const systemOpen = deps?.openSystemBrowser;
    if (systemOpen) {
      await systemOpen(href);
      return { ok: true, method: "system", href };
    }
    try {
      const mod = await import("@wailsio/runtime");
      const open = mod.Browser?.OpenURL;
      if (typeof open === "function") {
        await open(href);
        return { ok: true, method: "system", href };
      }
    } catch {
      /* not in Wails / runtime unavailable (pure Vite preview) */
    }
  }

  const winOpen =
    deps?.openWindow ??
    ((u: string, t?: string, f?: string) => {
      window.open(u, t, f);
    });
  winOpen(href, "_blank", "noopener,noreferrer");
  return { ok: true, method: "window", href };
}
