/**
 * Run: npx tsx src/lib/liquidGlass.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  GLASS_PRESETS,
  buildRadiusTable,
  bucketSize,
  convexSquircle,
  displacementAt,
  renderGlassMaps,
  sdRoundedBox,
  isChromiumBackdropHost,
  shouldEnableLiquidGlass,
  surfaceHeight,
} from "./liquidGlass";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

assert(convexSquircle(0) < 0.05, "squircle starts near 0 at the rim");
assert(convexSquircle(1) > 0.98, "squircle is flat at the bezel end");
assert(surfaceHeight("concave", 0) > 0.9, "concave is high at the rim");
assert(surfaceHeight("lip", 0) < surfaceHeight("lip", 0.35), "lip rises then dips");

const dEdge = displacementAt("convex-squircle", 0.15, 16, 1.5);
const dInner = displacementAt("convex-squircle", 0.95, 16, 1.5);
assert(Number.isFinite(dEdge) && Number.isFinite(dInner), "displacement is finite");
assert(Math.abs(dEdge) >= Math.abs(dInner), "bezel displaces more than the flat interior");

const table = buildRadiusTable("convex-squircle", 16, 1.5);
assert(table.samples.length === 127, "127 radius samples (8-bit map)");
assert(table.maxAbs > 0, "non-zero max displacement");

const maps = renderGlassMaps({
  width: 64,
  height: 40,
  radius: 12,
  preset: GLASS_PRESETS.panel,
});
assert(maps.displacement.length === 64 * 40 * 4, "disp buffer size");
assert(maps.specular.length === 64 * 40 * 4, "spec buffer size");
assert(maps.scale > 0, "filter scale > 0");

const cx = 32;
const cy = 20;
const center = (cy * 64 + cx) * 4;
assert(maps.displacement[center] === 128, "flat interior X is neutral");
assert(maps.displacement[center + 1] === 128, "flat interior Y is neutral");
assert(maps.displacement[center + 3] === 255, "interior is opaque in the map");

// Mid-left bezel: inward displacement is +X → R > 128
const left = (cy * 64 + 3) * 4;
assert(maps.displacement[left] > 128, "left bezel pushes samples inward (+X)");
assert(maps.displacement[left + 3] === 255, "bezel is in-map");

assert(sdRoundedBox(0, 0, 32, 20, 12) < 0, "center is inside the rounded rect");
assert(sdRoundedBox(40, 0, 32, 20, 12) > 0, "outside point is positive");

const b = bucketSize(311, 803, 176);
assert(b.w % 8 === 0 && b.h % 8 === 0, "bucket is 8px");
assert(Math.max(b.w, b.h) <= 176, "bucket respects cap");

assert(
  shouldEnableLiquidGlass({ prefEnabled: false }).layout === false,
  "pref off disables layout",
);
assert(
  shouldEnableLiquidGlass({ reducedTransparency: true }).layout === false,
  "reduced transparency disables glass",
);
assert(
  shouldEnableLiquidGlass({ hardwareAcceleration: false }).refraction === false,
  "gpu off disables refraction",
);
assert(
  shouldEnableLiquidGlass({ supportsSvgBackdrop: false }).refraction === false,
  "no svg backdrop → blur fallback",
);
assert(shouldEnableLiquidGlass({}).layout === true, "default layout on");
assert(shouldEnableLiquidGlass({}).refraction === true, "default refraction on");
assert(
  isChromiumBackdropHost("Mozilla/5.0 Chrome/128.0.0.0 Edg/128.0.0.0") === true,
  "webview2/edge is a chromium backdrop host",
);
assert(
  isChromiumBackdropHost("Mozilla/5.0 Firefox/130.0") === false,
  "firefox is not a chromium backdrop host",
);

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const repo = join(root, "..", "..");
const css = readFileSync(join(root, "style.css"), "utf8");
const appearance = readFileSync(
  join(root, "components/settings/panels/AppearancePanel.vue"),
  "utf8",
);
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const uiGo = readFileSync(join(repo, "internal", "settings", "ui.go"), "utf8");
const engine = readFileSync(join(root, "lib/liquidGlass.ts"), "utf8");
const runtime = readFileSync(join(root, "lib/liquidGlassRuntime.ts"), "utf8");

assert(css.includes("html.liquid-glass"), "css has liquid-glass tokens");
assert(css.includes("--lg-tint"), "css has glass tint tokens");
assert(css.includes("lg-scene"), "css has liquid-glass scene hook");
assert(
  /\.list-stack\s*\{[^}]*display:\s*flex/.test(css) &&
    !/html\.liquid-glass\s+\.list-stack\s*\{[^}]*display:\s*flex/.test(css),
  "list-stack is a flex column even without liquid-glass",
);
assert(!css.includes("overflow-y: scroll"), "no classic-windows overflow-y:scroll rail");
assert(!css.includes("scrollbar-width: auto"), "no OS-classic scrollbar-width override");
assert(
  !/\*\s*\{\s*scrollbar-width/.test(css),
  "no universal scrollbar-width (it disables ::-webkit-scrollbar)",
);
assert(
  !css.includes("--lg-inset"),
  "large panes are flush, not inset floating cards",
);
assert(!appearance.includes("onLiquidGlass"), "appearance does not expose a liquid glass switch");
assert(!appearance.includes("LiquidGlassPreview"), "appearance has no refraction preview");
assert(runtime.includes("userSpaceOnUse"), "filters use element pixel space");
assert(store.includes("liquidGlass: settings.liquidGlass"), "store persists pref");
assert(uiGo.includes("LiquidGlass"), "UIPrefs has LiquidGlass");
assert(engine.includes("convex-squircle"), "engine uses Apple squircle");
assert(engine.includes("feDisplacementMap") || runtime.includes("feDisplacementMap"), "svg displacement");
assert(runtime.includes('data-lg'), "runtime binds data-lg surfaces");
assert(!runtime.includes("article-row"), "rows are not refracted");

console.log("liquidGlass.selftest: OK");
