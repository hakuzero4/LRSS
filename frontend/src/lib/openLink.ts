/**
 * Open a URL according to Reading settings (openLinksInBrowser).
 * Desktop (Wails): prefers `Browser.OpenURL` (system browser).
 * Web access / pure browser: open a new tab immediately — never call the
 * Wails runtime first (async IPC miss + window.open loses the user gesture
 * and gets popup-blocked).
 */

import { normalizeOpenableUrl } from "./readingSettings";
import { isWebMode } from "./webMode";

export type OpenExternalResult =
  | { ok: false; reason: "empty" | "blocked" }
  | { ok: true; method: "system" | "window"; href: string };

export type OpenExternalDeps = {
  /** Injected for tests; defaults to Wails Browser.OpenURL when available. */
  openSystemBrowser?: (url: string) => Promise<void>;
  /** Injected for tests; defaults to window.open / anchor click. */
  openWindow?: (url: string, target?: string, features?: string) => void;
};

/** Open in a new tab without relying on window.open after an async gap. */
function openInNewTab(
  href: string,
  openWindow?: (url: string, target?: string, features?: string) => void,
): void {
  if (openWindow) {
    openWindow(href, "_blank", "noopener,noreferrer");
    return;
  }
  try {
    // Anchor click is more reliable than window.open after microtasks, and
    // still works when the call is the first step of an async click handler.
    const a = document.createElement("a");
    a.href = href;
    a.target = "_blank";
    a.rel = "noopener noreferrer";
    a.style.display = "none";
    document.body.appendChild(a);
    a.click();
    a.remove();
    return;
  } catch {
    /* fall through */
  }
  window.open(href, "_blank", "noopener,noreferrer");
}

/**
 * @param opts.forceBrowser - when true (default), try system browser first
 *   on desktop. Mirrors settings.openLinksInBrowser from call sites.
 *   In web mode this always opens a new browser tab.
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
  // Browser web-access SPA: skip Wails so we never await a failed
  // /wails/runtime POST (that gap gets window.open popup-blocked).
  if (useBrowser && !isWebMode()) {
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
      /* not in Wails / runtime unavailable — fall through to tab open */
    }
  }

  openInNewTab(href, deps?.openWindow);
  return { ok: true, method: "window", href };
}
