/** Accent presets and CSS variable application for the brand color. */

export type AccentPresetId = "purple" | "blue" | "teal" | "orange";

export const ACCENT_PRESETS: Record<
  AccentPresetId,
  { light: string; dark: string; labelKey: string }
> = {
  purple: {
    light: "#7c5cfc",
    dark: "#9b87ff",
    labelKey: "settings.appearance.accentPurple",
  },
  blue: {
    light: "#2563eb",
    dark: "#60a5fa",
    labelKey: "settings.appearance.accentBlue",
  },
  teal: {
    light: "#0d9488",
    dark: "#2dd4bf",
    labelKey: "settings.appearance.accentTeal",
  },
  orange: {
    light: "#ea580c",
    dark: "#fb923c",
    labelKey: "settings.appearance.accentOrange",
  },
};

export const ACCENT_PRESET_IDS = Object.keys(ACCENT_PRESETS) as AccentPresetId[];

const HEX_RE = /^#([0-9a-fA-F]{6})$/;

export function isAccentPresetId(v: string): v is AccentPresetId {
  return v in ACCENT_PRESETS;
}

export function isHexColor(v: string): boolean {
  return HEX_RE.test(v.trim());
}

/** Accept preset id or #RRGGBB. */
export function isValidAccent(v: unknown): v is string {
  return typeof v === "string" && (isAccentPresetId(v) || isHexColor(v));
}

/** Normalize to #rrggbb lowercase or a preset id. */
export function normalizeAccent(v: string, fallback: string = "purple"): string {
  const s = v.trim();
  if (isAccentPresetId(s)) return s;
  if (isHexColor(s)) return s.toLowerCase();
  // Expand #rgb
  const short = s.match(/^#([0-9a-fA-F]{3})$/);
  if (short) {
    const [r, g, b] = short[1].split("");
    return `#${r}${r}${g}${g}${b}${b}`.toLowerCase();
  }
  return fallback;
}

export function resolveAccentHex(accent: string, isDark: boolean): string {
  const a = normalizeAccent(accent);
  if (isHexColor(a)) return a;
  if (isAccentPresetId(a)) {
    return isDark ? ACCENT_PRESETS[a].dark : ACCENT_PRESETS[a].light;
  }
  return isDark ? ACCENT_PRESETS.purple.dark : ACCENT_PRESETS.purple.light;
}

function hexToRgb(hex: string): { r: number; g: number; b: number } | null {
  const m = hex.trim().match(/^#([0-9a-fA-F]{6})$/);
  if (!m) return null;
  const n = parseInt(m[1], 16);
  return { r: (n >> 16) & 255, g: (n >> 8) & 255, b: n & 255 };
}

function relativeLuminance(hex: string): number {
  const rgb = hexToRgb(hex);
  if (!rgb) return 0.5;
  const lin = [rgb.r, rgb.g, rgb.b].map((c) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : Math.pow((s + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2];
}

/** Soft surface tint derived from primary (for --accent). */
export function softAccentSurface(hex: string, isDark: boolean): string {
  const rgb = hexToRgb(hex);
  if (!rgb) return isDark ? "#2a2440" : "#f0edff";
  if (isDark) {
    return `color-mix(in srgb, ${hex} 22%, #1a1d26)`;
  }
  return `color-mix(in srgb, ${hex} 14%, #ffffff)`;
}

export function accentForeground(hex: string, isDark: boolean): string {
  const L = relativeLuminance(hex);
  // Soft label color on accent surface
  if (isDark) {
    return L > 0.35 ? hex : "#ddd6fe";
  }
  // Darken primary for readable text on light soft surface
  return `color-mix(in srgb, ${hex} 75%, #1a1d26)`;
}

export function primaryForeground(hex: string): string {
  return relativeLuminance(hex) > 0.55 ? "#16131f" : "#ffffff";
}

/** Write CSS custom properties used by Tailwind / app chrome. */
export function applyAccentToDocument(accent: string, isDark: boolean): void {
  if (typeof document === "undefined") return;
  const root = document.documentElement;
  const hex = resolveAccentHex(accent, isDark);
  const fg = primaryForeground(hex);
  const soft = softAccentSurface(hex, isDark);
  const softFg = accentForeground(hex, isDark);

  root.style.setProperty("--primary", hex);
  root.style.setProperty("--primary-foreground", fg);
  root.style.setProperty("--ring", hex);
  root.style.setProperty("--accent", soft);
  root.style.setProperty("--accent-foreground", softFg);
  root.style.setProperty("--sidebar-primary", hex);
  root.style.setProperty("--sidebar-primary-foreground", fg);
  root.style.setProperty("--sidebar-ring", hex);
  root.style.setProperty("--chart-1", hex);
  root.dataset.accent = isHexColor(normalizeAccent(accent))
    ? "custom"
    : normalizeAccent(accent);
}
