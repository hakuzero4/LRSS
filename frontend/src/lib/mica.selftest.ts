/**
 * Run: npx tsx src/lib/mica.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { shouldEnableMica } from "./mica";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

assert(
  shouldEnableMica({ webMode: false, userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" }) ===
    true,
  "windows desktop enables mica",
);
assert(
  shouldEnableMica({
    webMode: false,
    userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
    prefEnabled: false,
  }) === false,
  "appearance pref off disables mica",
);
assert(
  shouldEnableMica({
    webMode: false,
    userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
    hardwareAcceleration: false,
  }) === false,
  "hardware accel off disables mica",
);
assert(
  shouldEnableMica({ webMode: true, userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" }) ===
    false,
  "web access stays opaque",
);
assert(
  shouldEnableMica({ webMode: false, userAgent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0)" }) ===
    false,
  "mac has no mica class",
);
assert(
  shouldEnableMica({
    webMode: false,
    userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
    reducedTransparency: true,
  }) === false,
  "reduced transparency disables mica wash",
);

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const repo = join(root, "..", "..");
const css = readFileSync(join(root, "style.css"), "utf8");
const html = readFileSync(join(root, "..", "index.html"), "utf8");
const mainTs = readFileSync(join(root, "main.ts"), "utf8");
const appVue = readFileSync(join(root, "App.vue"), "utf8");
const goMain = readFileSync(join(repo, "main.go"), "utf8");
const micaGo = readFileSync(join(repo, "internal", "desktop", "mica.go"), "utf8");
const appearance = readFileSync(join(root, "components/settings/panels/AppearancePanel.vue"), "utf8");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const uiGo = readFileSync(join(repo, "internal", "settings", "ui.go"), "utf8");

assert(css.includes("html.mica"), "css has mica surface tokens");
assert(css.includes("html.mica.dark"), "css has dark mica tokens");
assert(
  /html\.mica\s+\.pane-chrome\s*\{[^}]*background:\s*transparent/.test(css),
  "mica pane headers are transparent",
);
assert(css.includes('[data-slot="dialog-overlay"]'), "mica dims dialog overlay");
assert(css.includes("backdrop-filter: none"), "mica overlay does not smear text");
assert(css.includes('[data-slot="dialog-content"]'), "mica dialogs use solid tokens");
assert(html.includes('classList.add("mica")') || html.includes("classList.add('mica')"), "index.html applies mica early");
assert(html.includes("__LRSS_WEB__"), "index.html skips mica when web bootstrap is present");
assert(mainTs.includes("applyDesktopMica"), "main.ts reapplies mica");
assert(appVue.includes("applyDesktopMica"), "App.vue drops mica after web bootstrap");
assert(goMain.includes("desktop.ApplyMica"), "main.go enables Wails Mica");
assert(goMain.includes("uiPrefs.MicaBackdrop"), "main.go reads mica pref");
assert(goMain.includes("ApplyWindowMicaFrom"), "main.go toggles mica on SetUIPrefs");
assert(micaGo.includes("application.Mica"), "desktop.ApplyMica sets BackdropType Mica");
assert(micaGo.includes("BackgroundTypeTranslucent"), "mica requires translucent background");
assert(appearance.includes("onMicaBackdrop"), "appearance has mica switch");
assert(appearance.includes("settings.appearance.mica"), "appearance mica i18n");
assert(store.includes("micaBackdrop: settings.micaBackdrop"), "store persists mica pref");
assert(uiGo.includes("MicaBackdrop"), "UIPrefs has MicaBackdrop");

console.log("mica.selftest: OK");
