/**
 * Browser (web access) mode helpers.
 * Web mode: same SPA over HTTP, no settings/management, star+read allowed.
 */

import { ref } from "vue";

const TOKEN_KEY = "lrss.webToken";
const MODE_KEY = "lrss.webMode";

/** Reactive flag for v-if gates (settings / feed management). */
export const webMode = ref(false);

/**
 * Web access token probe result.
 * - pending: loadAppsvc not finished
 * - ok: API reachable (token valid or auth disabled)
 * - unauthorized: 401 / missing-or-wrong token
 * - none: not in web HTTP mode (desktop / no backend)
 */
export type WebAuthState = "pending" | "ok" | "unauthorized" | "none";

export const webAuthState = ref<WebAuthState>("pending");

export function setWebAuthState(state: WebAuthState): void {
  webAuthState.value = state;
}

export function clearWebToken(): void {
  try {
    if (typeof sessionStorage !== "undefined") {
      sessionStorage.removeItem(TOKEN_KEY);
    }
  } catch {
    /* ignore */
  }
}

function readForcedWeb(): boolean {
  try {
    if ((globalThis as unknown as { __LRSS_WEB__?: boolean }).__LRSS_WEB__) return true;
  } catch {
    /* ignore */
  }
  try {
    if (typeof sessionStorage !== "undefined" && sessionStorage.getItem(MODE_KEY) === "1") {
      return true;
    }
  } catch {
    /* ignore */
  }
  return false;
}

// Eager: server-injected flag is available before Vue mounts.
if (typeof window !== "undefined" && readForcedWeb()) {
  webMode.value = true;
}

/** Capture ?token= into sessionStorage and strip it from the URL. */
export function captureWebTokenFromURL(): void {
  if (typeof window === "undefined") return;
  try {
    const u = new URL(window.location.href);
    const tok = u.searchParams.get("token");
    if (tok) {
      sessionStorage.setItem(TOKEN_KEY, tok);
      u.searchParams.delete("token");
      const next = u.pathname + u.search + u.hash;
      window.history.replaceState(null, "", next || "/");
    }
  } catch {
    /* ignore */
  }
}

export function getWebToken(): string {
  if (typeof sessionStorage === "undefined") return "";
  return sessionStorage.getItem(TOKEN_KEY) ?? "";
}

export function setWebModeFlag(on: boolean): void {
  webMode.value = on;
  try {
    if (on) sessionStorage.setItem(MODE_KEY, "1");
    else sessionStorage.removeItem(MODE_KEY);
  } catch {
    /* ignore */
  }
}

/** Synchronous: true in browser web-access mode. */
export function isWebMode(): boolean {
  return webMode.value || readForcedWeb();
}

/** Wails v3 serves the desktop WebView from this host (dev + production). */
export function isWailsHost(hostname: string): boolean {
  const h = (hostname || "").toLowerCase();
  return h === "wails.localhost" || h.endsWith(".wails.localhost");
}

export function isWailsRuntime(): boolean {
  try {
    if (typeof window !== "undefined" && isWailsHost(window.location.hostname)) {
      return true;
    }
    // Wails v2 leftover; v3 does not set window.wails.
    const w = window as unknown as { wails?: unknown };
    if (w.wails) return true;
  } catch {
    /* ignore */
  }
  return false;
}
