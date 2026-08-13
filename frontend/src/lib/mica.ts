/**
 * Windows 11 Mica: the desktop WebView is translucent so DWM can show
 * the wallpaper tint. Web access (browser) must stay opaque.
 */

export function isWindowsDesktopUA(userAgent: string): boolean {
  return /Windows/i.test(userAgent);
}

export function shouldEnableMica(opts: {
  webMode: boolean;
  userAgent: string;
  reducedTransparency?: boolean;
  prefEnabled?: boolean;
  hardwareAcceleration?: boolean;
}): boolean {
  if (opts.webMode) return false;
  if (opts.prefEnabled === false) return false;
  if (opts.hardwareAcceleration === false) return false;
  if (opts.reducedTransparency) return false;
  return isWindowsDesktopUA(opts.userAgent);
}

function isForcedWeb(): boolean {
  try {
    return !!(globalThis as unknown as { __LRSS_WEB__?: boolean }).__LRSS_WEB__;
  } catch {
    return false;
  }
}

function prefersReducedTransparency(): boolean {
  try {
    return Boolean(window.matchMedia?.("(prefers-reduced-transparency: reduce)")?.matches);
  } catch {
    return false;
  }
}

export type MicaPref = {
  enabled?: boolean;
  hardwareAcceleration?: boolean;
};

/** Toggle `html.mica`. Safe to call before and after backend bootstrap. */
export function applyDesktopMica(webMode = isForcedWeb(), pref: MicaPref = {}): boolean {
  const on = shouldEnableMica({
    webMode,
    userAgent: typeof navigator !== "undefined" ? navigator.userAgent : "",
    reducedTransparency: prefersReducedTransparency(),
    prefEnabled: pref.enabled,
    hardwareAcceleration: pref.hardwareAcceleration,
  });
  if (typeof document !== "undefined") {
    document.documentElement.classList.toggle("mica", on);
  }
  return on;
}
