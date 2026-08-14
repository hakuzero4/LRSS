/**
 * Attaches kube.io Liquid Glass SVG filters to [data-lg] surfaces.
 * Maps are size-bucketed and cached. Scrolling article/feed rows are never registered.
 */

import {
  type GlassKind,
  GLASS_PRESETS,
  applyLiquidGlassClass,
  bucketSize,
  mapsToPngDataUrl,
  prefersReducedTransparency,
  renderGlassMaps,
  shouldEnableLiquidGlass,
  supportsSvgBackdropFilter,
} from "./liquidGlass";

const ATTR = "data-lg";
const ID_ATTR = "data-lg-id";

type RuntimeOpts = {
  prefEnabled?: boolean;
  hardwareAcceleration?: boolean;
};

type LiveSurface = {
  el: HTMLElement;
  kind: GlassKind;
  filterId: string;
  lastKey: string;
};

const KINDS = new Set<GlassKind>(["panel", "bar", "dialog", "search"]);

function parseKind(raw: string | null): GlassKind | null {
  if (!raw) return null;
  return KINDS.has(raw as GlassKind) ? (raw as GlassKind) : null;
}

function readRadius(el: HTMLElement): number {
  try {
    const cs = getComputedStyle(el);
    const n = parseFloat(cs.borderTopLeftRadius || "0");
    return Number.isFinite(n) ? n : 22;
  } catch {
    return 22;
  }
}

function measure(el: HTMLElement): { w: number; h: number } {
  try {
    const r = el.getBoundingClientRect();
    return { w: r.width || el.offsetWidth || 0, h: r.height || el.offsetHeight || 0 };
  } catch {
    return { w: 0, h: 0 };
  }
}

let seq = 0;
let host: HTMLElement | null = null;
let svg: SVGSVGElement | null = null;
let defs: SVGDefsElement | null = null;
let mo: MutationObserver | null = null;
let ro: ResizeObserver | null = null;
let started = false;
let layoutOn = false;
let refractionOn = false;
let raf = 0;
const live = new Map<HTMLElement, LiveSurface>();
const pending = new Set<HTMLElement>();
const mapCache = new Map<string, { disp: string; spec: string; scale: number }>();
const MAX_CACHE = 24;

function cacheGet(key: string) {
  const hit = mapCache.get(key);
  if (!hit) return null;
  mapCache.delete(key);
  mapCache.set(key, hit);
  return hit;
}

function cacheSet(key: string, val: { disp: string; spec: string; scale: number }) {
  if (mapCache.has(key)) mapCache.delete(key);
  mapCache.set(key, val);
  while (mapCache.size > MAX_CACHE) {
    const first = mapCache.keys().next().value;
    if (first === undefined) break;
    mapCache.delete(first);
  }
}

function ensureSvg() {
  if (svg) return;
  svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("focusable", "false");
  svg.classList.add("lg-svg");
  svg.setAttribute("width", "0");
  svg.setAttribute("height", "0");
  // Body-level: overflow-hidden ancestors clip 0×0 hosts and break url(#filter).
  svg.style.cssText =
    "position:fixed;left:0;top:0;width:0;height:0;overflow:visible;pointer-events:none;z-index:-1;";
  defs = document.createElementNS("http://www.w3.org/2000/svg", "defs");
  svg.appendChild(defs);
  document.body.appendChild(svg);
}

function upsertFilter(
  id: string,
  dispUrl: string,
  specUrl: string,
  elW: number,
  elH: number,
  scale: number,
  blur: number,
) {
  if (!defs) return;
  let filter = defs.querySelector<SVGFilterElement>(`#${CSS.escape(id)}`);
  if (!filter) {
    filter = document.createElementNS("http://www.w3.org/2000/svg", "filter");
    filter.setAttribute("id", id);
    defs.appendChild(filter);
  }
  // userSpaceOnUse + explicit px: backdrop-filter does not auto-fit the element
  // (kube.io). objectBoundingBox + 100% often yields a no-op warp.
  const w = Math.max(1, Math.round(elW));
  const h = Math.max(1, Math.round(elH));
  filter.setAttribute("color-interpolation-filters", "sRGB");
  filter.setAttribute("filterUnits", "userSpaceOnUse");
  filter.setAttribute("primitiveUnits", "userSpaceOnUse");
  filter.setAttribute("x", "0");
  filter.setAttribute("y", "0");
  filter.setAttribute("width", String(w));
  filter.setAttribute("height", String(h));
  filter.innerHTML =
    `<feImage href="${dispUrl}" x="0" y="0" width="${w}" height="${h}" preserveAspectRatio="none" result="disp"/>` +
    `<feDisplacementMap in="SourceGraphic" in2="disp" scale="${scale.toFixed(2)}" xChannelSelector="R" yChannelSelector="G" result="refract"/>` +
    `<feGaussianBlur in="refract" stdDeviation="${blur.toFixed(2)}" result="blurred"/>` +
    `<feImage href="${specUrl}" x="0" y="0" width="${w}" height="${h}" preserveAspectRatio="none" result="spec"/>` +
    `<feBlend in="blurred" in2="spec" mode="screen"/>`;
}

function applyFallback(el: HTMLElement, kind: GlassKind) {
  const blur = GLASS_PRESETS[kind].blur + 10;
  el.style.setProperty("--lg-filter", `blur(${blur}px) saturate(1.35)`);
}

function applySurface(el: HTMLElement) {
  if (!layoutOn) return;
  const kind = parseKind(el.getAttribute(ATTR));
  if (!kind) return;
  if (!refractionOn) {
    applyFallback(el, kind);
    return;
  }
  const { w, h } = measure(el);
  if (w < 8 || h < 8) {
    applyFallback(el, kind);
    return;
  }
  const radius = readRadius(el);
  const native = kind === "search" || w * h < 48000;
  const bucket = native
    ? { w: Math.max(8, Math.round(w)), h: Math.max(8, Math.round(h)), scale: 1 }
    : bucketSize(w, h);
  const mapRadius = radius * (bucket.w / w);
  const preset = GLASS_PRESETS[kind];
  const key = `${kind}:${bucket.w}x${bucket.h}:e${Math.round(w)}x${Math.round(h)}:r${Math.round(mapRadius)}`;

  let rec = live.get(el);
  if (!rec) {
    const id = `lgf-${++seq}`;
    el.setAttribute(ID_ATTR, id);
    rec = { el, kind, filterId: id, lastKey: "" };
    live.set(el, rec);
  }

  if (rec.lastKey === key) return;

  let maps = cacheGet(key);
  if (!maps) {
    const rendered = renderGlassMaps({
      width: bucket.w,
      height: bucket.h,
      radius: mapRadius,
      preset,
    });
    const disp = mapsToPngDataUrl(rendered.displacement, rendered.width, rendered.height);
    const spec = mapsToPngDataUrl(rendered.specular, rendered.width, rendered.height);
    if (!disp || !spec) {
      applyFallback(el, kind);
      return;
    }
    maps = { disp, spec, scale: rendered.scale };
    cacheSet(key, maps);
  }

  const scale =
    maps.scale * ((w / bucket.w + h / bucket.h) / 2);
  upsertFilter(rec.filterId, maps.disp, maps.spec, w, h, scale, preset.blur);
  rec.lastKey = key;
  el.style.setProperty(
    "--lg-filter",
    `url(#${rec.filterId}) blur(${preset.blur}px) saturate(1.32)`,
  );
}

function flush() {
  raf = 0;
  for (const el of pending) {
    if (!el.isConnected) {
      detach(el);
      continue;
    }
    applySurface(el);
  }
  pending.clear();
}

function queue(el: HTMLElement) {
  pending.add(el);
  if (!raf) raf = requestAnimationFrame(flush);
}

function detach(el: HTMLElement) {
  const rec = live.get(el);
  if (rec && defs) {
    defs.querySelector(`#${CSS.escape(rec.filterId)}`)?.remove();
  }
  live.delete(el);
  pending.delete(el);
  el.style.removeProperty("--lg-filter");
  el.removeAttribute(ID_ATTR);
}

function scan(root: ParentNode) {
  if (!layoutOn) return;
  if (!(root instanceof Element) && !(root instanceof Document)) return;
  const found = Array.from(root.querySelectorAll<HTMLElement>(`[${ATTR}]`));
  const nodes =
    root instanceof Element && root.hasAttribute(ATTR)
      ? [root as HTMLElement, ...found]
      : found;
  for (const el of nodes) {
    if (!(el instanceof HTMLElement)) continue;
    if (!parseKind(el.getAttribute(ATTR))) continue;
    if (!live.has(el)) {
      ro?.observe(el);
    }
    queue(el);
  }
}

function onMutations(records: MutationRecord[]) {
  for (const rec of records) {
    if (rec.type === "attributes" && rec.target instanceof HTMLElement) {
      if (rec.attributeName === ATTR) {
        if (rec.target.hasAttribute(ATTR)) queue(rec.target);
        else detach(rec.target);
      }
      continue;
    }
    rec.addedNodes.forEach((n) => {
      if (n instanceof HTMLElement) scan(n);
    });
    rec.removedNodes.forEach((n) => {
      if (!(n instanceof HTMLElement)) return;
      if (n.hasAttribute(ATTR)) detach(n);
      n.querySelectorAll?.<HTMLElement>(`[${ATTR}]`).forEach(detach);
    });
  }
}

export function resolveLiquidGlass(opts: RuntimeOpts = {}) {
  return shouldEnableLiquidGlass({
    prefEnabled: opts.prefEnabled,
    hardwareAcceleration: opts.hardwareAcceleration,
    reducedTransparency: prefersReducedTransparency(),
    supportsSvgBackdrop: supportsSvgBackdropFilter(),
  });
}

export function applyLiquidGlass(opts: RuntimeOpts = {}): boolean {
  const state = resolveLiquidGlass(opts);
  layoutOn = state.layout;
  applyLiquidGlassClass(state.layout);
  refractionOn = state.layout && state.refraction;
  if (typeof document !== "undefined") {
    document.documentElement.classList.toggle("lg-refract", refractionOn);
  }
  if (started) {
    if (!state.layout) {
      Array.from(live.keys()).forEach(detach);
    } else {
      live.forEach((s) => {
        s.lastKey = "";
        queue(s.el);
      });
      scan(document);
    }
  }
  return state.layout;
}

export function startLiquidGlassRuntime(mount: HTMLElement, opts: RuntimeOpts = {}) {
  if (started) {
    host = mount;
    applyLiquidGlass(opts);
    return;
  }
  host = mount;
  started = true;
  applyLiquidGlass(opts);
  ensureSvg();
  ro = new ResizeObserver((entries) => {
    for (const e of entries) {
      if (e.target instanceof HTMLElement) queue(e.target);
    }
  });
  mo = new MutationObserver(onMutations);
  mo.observe(document.body, {
    subtree: true,
    childList: true,
    attributes: true,
    attributeFilter: [ATTR],
  });
  scan(document);
}

export function stopLiquidGlassRuntime() {
  mo?.disconnect();
  ro?.disconnect();
  if (raf) cancelAnimationFrame(raf);
  raf = 0;
  Array.from(live.keys()).forEach(detach);
  svg?.remove();
  svg = null;
  defs = null;
  host = null;
  started = false;
  mapCache.clear();
}

export function refreshLiquidGlass() {
  live.forEach((s) => {
    s.lastKey = "";
    queue(s.el);
  });
  if (typeof document !== "undefined") scan(document);
}
