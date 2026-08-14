/**
 * Apple Liquid Glass (WWDC 2025) — refraction math + displacement maps.
 * Follows https://kube.io/blog/liquid-glass-css-svg/ :
 *   surface function → Snell → polar displacement → RG feDisplacementMap
 *   + rim specular, combined in an SVG filter used as backdrop-filter.
 */

export type SurfaceKind = "convex-circle" | "convex-squircle" | "concave" | "lip";

export type GlassKind = "panel" | "bar" | "dialog" | "search";

export type GlassPreset = {
  surface: SurfaceKind;
  /** Bezel width in CSS pixels (clamped to a fraction of the short side). */
  bezel: number;
  /** Optical thickness in px — scales displacement magnitude. */
  thickness: number;
  ior: number;
  /** Extra Gaussian blur after refraction (px). */
  blur: number;
  /** Multiplier on the physical max displacement (0–1.5). */
  refraction: number;
  specularOpacity: number;
  /** Light direction in degrees (0 = +X, −90 = up). */
  specularAngle: number;
};

export const GLASS_PRESETS: Record<GlassKind, GlassPreset> = {
  panel: {
    surface: "convex-squircle",
    bezel: 18,
    thickness: 16,
    ior: 1.5,
    blur: 0.7,
    refraction: 0.85,
    specularOpacity: 0.38,
    specularAngle: -58,
  },
  bar: {
    surface: "convex-squircle",
    bezel: 8,
    thickness: 10,
    ior: 1.5,
    blur: 0.45,
    refraction: 0.7,
    specularOpacity: 0.3,
    specularAngle: -58,
  },
  dialog: {
    surface: "convex-squircle",
    bezel: 20,
    thickness: 14,
    ior: 1.5,
    blur: 1.1,
    refraction: 0.5,
    specularOpacity: 0.26,
    specularAngle: -50,
  },
  search: {
    surface: "convex-squircle",
    bezel: 12,
    thickness: 18,
    ior: 1.5,
    blur: 0.15,
    refraction: 1.25,
    specularOpacity: 0.48,
    specularAngle: -50,
  },
};

export const RADIUS_SAMPLES = 127;

export function clamp(n: number, lo: number, hi: number): number {
  return n < lo ? lo : n > hi ? hi : n;
}

export function mix(a: number, b: number, t: number): number {
  return a + (b - a) * t;
}

export function smootherstep(x: number): number {
  const t = clamp(x, 0, 1);
  return t * t * t * (t * (t * 6 - 15) + 10);
}

/** y = sqrt(1 − (1 − x)^2) — spherical dome. */
export function convexCircle(x: number): number {
  const t = 1 - clamp(x, 0, 1);
  return Math.sqrt(Math.max(0, 1 - t * t));
}

/** y = (1 − (1 − x)^4)^(1/4) — Apple squircle, softer flat→curve. */
export function convexSquircle(x: number): number {
  const t = 1 - clamp(x, 0, 1);
  return Math.pow(Math.max(0, 1 - t ** 4), 0.25);
}

export function surfaceHeight(kind: SurfaceKind, x: number): number {
  const xx = clamp(x, 0, 1);
  switch (kind) {
    case "convex-circle":
      return convexCircle(xx);
    case "convex-squircle":
      return convexSquircle(xx);
    case "concave":
      return 1 - convexSquircle(xx);
    case "lip":
      return mix(convexSquircle(xx), 1 - convexSquircle(xx), smootherstep(xx));
  }
}

export function surfaceDerivative(kind: SurfaceKind, x: number): number {
  const d = 0.001;
  return (surfaceHeight(kind, x + d) - surfaceHeight(kind, x - d)) / (2 * d);
}

/**
 * One-refraction displacement along the inward normal (px).
 * Incident ray is orthogonal to the background plane (kube.io constraints).
 * Convex profiles keep the sample inside the glass.
 */
export function displacementAt(
  kind: SurfaceKind,
  x: number,
  thickness: number,
  ior: number,
): number {
  const h = surfaceHeight(kind, x) * thickness;
  const dh = surfaceDerivative(kind, x);
  const theta1 = Math.atan(dh);
  const sin2 = Math.sin(theta1) / Math.max(1.0001, ior);
  if (Math.abs(sin2) >= 1) return 0;
  const theta2 = Math.asin(sin2);
  return Math.tan(theta1 - theta2) * h;
}

export type RadiusTable = {
  /** 127 samples, x = i / 126 (0 = outer edge). */
  samples: Float64Array;
  maxAbs: number;
};

export function buildRadiusTable(
  kind: SurfaceKind,
  thickness: number,
  ior: number,
): RadiusTable {
  const samples = new Float64Array(RADIUS_SAMPLES);
  let maxAbs = 0;
  for (let i = 0; i < RADIUS_SAMPLES; i++) {
    const x = i / (RADIUS_SAMPLES - 1);
    const d = displacementAt(kind, x, thickness, ior);
    samples[i] = d;
    const a = Math.abs(d);
    if (a > maxAbs) maxAbs = a;
  }
  return { samples, maxAbs: maxAbs || 1 };
}

export function sampleRadius(table: RadiusTable, x: number): number {
  const t = clamp(x, 0, 1) * (RADIUS_SAMPLES - 1);
  const i = Math.floor(t);
  const f = t - i;
  const a = table.samples[i] ?? 0;
  const b = table.samples[Math.min(RADIUS_SAMPLES - 1, i + 1)] ?? a;
  return mix(a, b, f);
}

/** Signed distance of a rounded rectangle (positive outside). */
export function sdRoundedBox(
  px: number,
  py: number,
  hw: number,
  hh: number,
  r: number,
): number {
  const rr = clamp(r, 0, Math.min(hw, hh));
  const qx = Math.abs(px) - hw + rr;
  const qy = Math.abs(py) - hh + rr;
  const ox = Math.max(qx, 0);
  const oy = Math.max(qy, 0);
  return Math.hypot(ox, oy) + Math.min(Math.max(qx, qy), 0) - rr;
}

export function sdfGradient(
  px: number,
  py: number,
  hw: number,
  hh: number,
  r: number,
): { x: number; y: number } {
  const e = 0.6;
  const dx = sdRoundedBox(px + e, py, hw, hh, r) - sdRoundedBox(px - e, py, hw, hh, r);
  const dy = sdRoundedBox(px, py + e, hw, hh, r) - sdRoundedBox(px, py - e, hw, hh, r);
  const len = Math.hypot(dx, dy) || 1;
  return { x: dx / len, y: dy / len };
}

export type MapBuffers = {
  width: number;
  height: number;
  /** RGBA, R=X G=Y, 128 = none. */
  displacement: Uint8ClampedArray;
  /** RGBA specular rim (screen-blend). */
  specular: Uint8ClampedArray;
  /** Physical max |displacement| in map pixels (feDisplacementMap scale). */
  scale: number;
};

export type RenderMapOpts = {
  width: number;
  height: number;
  radius: number;
  preset: GlassPreset;
};

export function renderGlassMaps(opts: RenderMapOpts): MapBuffers {
  const width = Math.max(2, Math.round(opts.width));
  const height = Math.max(2, Math.round(opts.height));
  const radius = clamp(opts.radius, 0, Math.min(width, height) / 2);
  const { preset } = opts;
  const bezel = clamp(preset.bezel, 2, Math.min(width, height) * 0.42);
  const table = buildRadiusTable(preset.surface, preset.thickness, preset.ior);
  const hw = width / 2;
  const hh = height / 2;
  const disp = new Uint8ClampedArray(width * height * 4);
  const spec = new Uint8ClampedArray(width * height * 4);
  const lightX = Math.cos((preset.specularAngle * Math.PI) / 180);
  const lightY = Math.sin((preset.specularAngle * Math.PI) / 180);
  const invMax = 1 / table.maxAbs;

  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4;
      const px = x + 0.5 - hw;
      const py = y + 0.5 - hh;
      const sd = sdRoundedBox(px, py, hw, hh, radius);
      if (sd > 0.5) {
        disp[i] = 128;
        disp[i + 1] = 128;
        disp[i + 2] = 128;
        disp[i + 3] = 0;
        spec[i + 3] = 0;
        continue;
      }
      const distIn = Math.max(0, -sd);
      const t = distIn / bezel;
      const n = sdfGradient(px, py, hw, hh, radius);
      if (t >= 1) {
        disp[i] = 128;
        disp[i + 1] = 128;
        disp[i + 2] = 128;
        disp[i + 3] = 255;
      } else {
        const mag = sampleRadius(table, t) * invMax; // −1..1, convex → inward
        // Convex displacement is toward the interior = opposite the outward SDF gradient.
        const dx = -n.x * mag;
        const dy = -n.y * mag;
        disp[i] = clamp(Math.round(128 + dx * 127), 0, 255);
        disp[i + 1] = clamp(Math.round(128 + dy * 127), 0, 255);
        disp[i + 2] = 128;
        disp[i + 3] = 255;
      }
      const rim = Math.max(0, 1 - t);
      const lit = Math.pow(Math.max(0, n.x * lightX + n.y * lightY), 7);
      const a = rim * rim * lit * preset.specularOpacity;
      const v = clamp(Math.round(a * 255), 0, 255);
      spec[i] = v;
      spec[i + 1] = v;
      spec[i + 2] = v;
      spec[i + 3] = v;
    }
  }

  return {
    width,
    height,
    displacement: disp,
    specular: spec,
    scale: table.maxAbs * preset.refraction,
  };
}

export function bucketSize(
  w: number,
  h: number,
  cap = 176,
): { w: number; h: number; scale: number } {
  const aw = Math.max(1, w);
  const ah = Math.max(1, h);
  const scale = Math.min(1, cap / Math.max(aw, ah));
  const bw = Math.max(8, Math.round((aw * scale) / 8) * 8);
  const bh = Math.max(8, Math.round((ah * scale) / 8) * 8);
  return { w: bw, h: bh, scale };
}

export function mapsToPngDataUrl(data: Uint8ClampedArray, w: number, h: number): string {
  if (typeof document === "undefined") {
    return "";
  }
  const canvas = document.createElement("canvas");
  canvas.width = w;
  canvas.height = h;
  const ctx = canvas.getContext("2d");
  if (!ctx) return "";
  ctx.putImageData(new ImageData(data, w, h), 0, 0);
  return canvas.toDataURL("image/png");
}

export type LiquidGlassEnable = {
  /** Floating chrome + tint (even if we only blur). */
  layout: boolean;
  /** SVG displacement as backdrop-filter. */
  refraction: boolean;
};

export function shouldEnableLiquidGlass(opts: {
  prefEnabled?: boolean;
  hardwareAcceleration?: boolean;
  reducedTransparency?: boolean;
  supportsSvgBackdrop?: boolean;
}): LiquidGlassEnable {
  if (opts.prefEnabled === false) return { layout: false, refraction: false };
  if (opts.reducedTransparency) return { layout: false, refraction: false };
  const refraction =
    opts.hardwareAcceleration !== false && opts.supportsSvgBackdrop !== false;
  return { layout: true, refraction };
}

export function isChromiumBackdropHost(userAgent = ""): boolean {
  const ua = userAgent || (typeof navigator !== "undefined" ? navigator.userAgent : "");
  if (/Firefox|FxIOS/i.test(ua)) return false;
  return /Chrome\/|Chromium\/|Edg\/|EdgA\/|EdgiOS\//i.test(ua);
}

export function supportsSvgBackdropFilter(): boolean {
  try {
    if (typeof CSS === "undefined" || typeof CSS.supports !== "function") {
      return isChromiumBackdropHost();
    }
    const blurOk =
      CSS.supports("backdrop-filter", "blur(1px)") ||
      CSS.supports("-webkit-backdrop-filter", "blur(1px)");
    if (!blurOk) return false;
    // Chrome's CSS.supports("backdrop-filter", "url(#x)") is unreliable.
    // WebView2 / Edge / Chrome all accept SVG filters as backdrop-filter.
    if (isChromiumBackdropHost()) return true;
    return (
      CSS.supports("backdrop-filter", "url(#lg-probe)") ||
      CSS.supports("-webkit-backdrop-filter", "url(#lg-probe)")
    );
  } catch {
    return isChromiumBackdropHost();
  }
}

export function prefersReducedTransparency(): boolean {
  try {
    return Boolean(
      typeof window !== "undefined" &&
        window.matchMedia?.("(prefers-reduced-transparency: reduce)")?.matches,
    );
  } catch {
    return false;
  }
}

/** Toggle `html.liquid-glass`. Safe before and after bootstrap. */
export function applyLiquidGlassClass(enabled: boolean): void {
  if (typeof document === "undefined") return;
  document.documentElement.classList.toggle("liquid-glass", enabled);
}
