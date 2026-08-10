/**
 * Real unit tests for Settings → Reading shipped helpers.
 * Run: npx tsx src/lib/readingSettings.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { openExternalLink } from "./openLink";
import {
  applyShowUnreadOnly,
  normalizeOpenableUrl,
  readerShellClasses,
  shouldMarkReadOnScrollEnd,
} from "./readingSettings";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// —— readerShellClasses (drives ArticleReader shell) ——
const sm = readerShellClasses("sm", "narrow");
assert(sm.font === "reader-font-sm", "sm font class");
assert(sm.width === "reader-width-narrow", "narrow width class");
assert(sm.className.includes("reader-font-sm") && sm.className.includes("reader-width-narrow"), "className join");

const lg = readerShellClasses("lg", "wide");
assert(lg.font === "reader-font-lg" && lg.width === "reader-width-wide", "lg/wide");

const def = readerShellClasses("nope", undefined);
assert(def.font === "reader-font-md" && def.width === "reader-width-medium", "defaults");

// CSS must define those class tokens with visible size/width
const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const css = readFileSync(join(root, "style.css"), "utf8");
for (const token of [
  "reader-font-sm",
  "reader-font-md",
  "reader-font-lg",
  "reader-width-narrow",
  "reader-width-medium",
  "reader-width-wide",
  "--reader-body-size",
]) {
  assert(css.includes(token), `css defines ${token}`);
}
// sizes must differ across sm/md/lg
const bodySizes = [...css.matchAll(/--reader-body-size:\s*([\d.]+)px/g)].map((m) => m[1]);
assert(new Set(bodySizes).size >= 3, `expected 3 distinct body sizes, got ${bodySizes}`);

// —— applyShowUnreadOnly ——
const sample = [
  { id: "1", read: false, title: "u" },
  { id: "2", read: true, title: "r" },
  { id: "3", read: false, title: "u2" },
];
assert(
  applyShowUnreadOnly(sample, false, "all").length === 3,
  "off keeps all",
);
assert(
  applyShowUnreadOnly(sample, true, "all").map((a) => a.id).join(",") === "1,3",
  "on filters read",
);
assert(
  applyShowUnreadOnly(sample, true, "starred").length === 3,
  "starred exempt",
);
assert(
  applyShowUnreadOnly(sample, true, "unread").map((a) => a.id).join(",") === "1,3",
  "on in unread collection",
);

// —— shouldMarkReadOnScrollEnd ——
assert(
  !shouldMarkReadOnScrollEnd({
    enabled: false,
    articleId: "a",
    alreadyRead: false,
    alreadyMarkedId: null,
    scrollHeight: 1000,
    scrollTop: 900,
    clientHeight: 100,
  }),
  "disabled",
);
assert(
  !shouldMarkReadOnScrollEnd({
    enabled: true,
    articleId: "a",
    alreadyRead: true,
    alreadyMarkedId: null,
    scrollHeight: 1000,
    scrollTop: 900,
    clientHeight: 100,
  }),
  "already read",
);
assert(
  !shouldMarkReadOnScrollEnd({
    enabled: true,
    articleId: "a",
    alreadyRead: false,
    alreadyMarkedId: "a",
    scrollHeight: 1000,
    scrollTop: 900,
    clientHeight: 100,
  }),
  "already one-shot marked",
);
assert(
  !shouldMarkReadOnScrollEnd({
    enabled: true,
    articleId: "a",
    alreadyRead: false,
    alreadyMarkedId: null,
    scrollHeight: 1000,
    scrollTop: 100,
    clientHeight: 100,
  }),
  "not near bottom",
);
assert(
  shouldMarkReadOnScrollEnd({
    enabled: true,
    articleId: "a",
    alreadyRead: false,
    alreadyMarkedId: null,
    scrollHeight: 1000,
    scrollTop: 960,
    clientHeight: 100,
  }),
  "near bottom marks",
);
// thrash guard: second call with same alreadyMarkedId is false
assert(
  !shouldMarkReadOnScrollEnd({
    enabled: true,
    articleId: "a",
    alreadyRead: false,
    alreadyMarkedId: "a",
    scrollHeight: 1000,
    scrollTop: 960,
    clientHeight: 100,
  }),
  "no thrash",
);

// —— openExternalLink real helper with injected deps ——
assert(normalizeOpenableUrl("") === null, "empty url");
assert(normalizeOpenableUrl("javascript:alert(1)") === null, "js blocked");
assert(normalizeOpenableUrl(" https://ex.com ") === "https://ex.com", "trim");

const systemCalls: string[] = [];
const windowCalls: string[] = [];
const r1 = await openExternalLink(
  "https://example.com/a",
  { forceBrowser: true },
  {
    openSystemBrowser: async (u) => {
      systemCalls.push(u);
    },
    openWindow: (u) => {
      windowCalls.push(u);
    },
  },
);
assert(r1.ok && r1.method === "system" && r1.href === "https://example.com/a", "system path");
assert(systemCalls.length === 1 && windowCalls.length === 0, "system only");

const r2 = await openExternalLink(
  "https://example.com/b",
  { forceBrowser: false },
  {
    openSystemBrowser: async (u) => {
      systemCalls.push(u);
    },
    openWindow: (u) => {
      windowCalls.push(u);
    },
  },
);
assert(r2.ok && r2.method === "window", "window path when forceBrowser false");
assert(windowCalls.length === 1 && windowCalls[0] === "https://example.com/b", "window open");
assert(systemCalls.length === 1, "system not called when off");

const r3 = await openExternalLink("javascript:void(0)", { forceBrowser: true }, {
  openSystemBrowser: async () => {},
  openWindow: () => {},
});
assert(!r3.ok && r3.reason === "blocked", "js blocked result");

// —— structural: reader + store use the real helpers ——
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const shortcuts = readFileSync(join(root, "composables/useKeyboardShortcuts.ts"), "utf8");

assert(reader.includes("readerShellClasses"), "reader uses readerShellClasses");
assert(reader.includes("shouldMarkReadOnScrollEnd"), "reader uses scroll helper");
assert(reader.includes("openExternalLink"), "reader uses openExternalLink");
assert(reader.includes("onBodyClick"), "body click intercept");
assert(reader.includes("openLinksInBrowser"), "reader reads openLinksInBrowser");
assert(store.includes("applyShowUnreadOnly"), "store uses applyShowUnreadOnly");
assert(store.includes("showUnreadOnly"), "store persists/reads showUnreadOnly");
assert(store.includes("buildUIPrefs") || store.includes("persistUIPrefs"), "prefs persist path");
assert(shortcuts.includes("openExternalLink"), "shortcut uses openExternalLink");

// persist UIPrefs includes reading fields
assert(store.includes("fontSize: settings.fontSize"), "persist fontSize");
assert(store.includes("readerWidth: settings.readerWidth"), "persist readerWidth");
assert(store.includes("showUnreadOnly: settings.showUnreadOnly"), "persist showUnreadOnly");
assert(store.includes("openLinksInBrowser: settings.openLinksInBrowser"), "persist openLinks");
assert(store.includes("markAsReadOnScrollEnd: settings.markAsReadOnScrollEnd"), "persist scroll end");

console.log("readingSettings.selftest: OK");
