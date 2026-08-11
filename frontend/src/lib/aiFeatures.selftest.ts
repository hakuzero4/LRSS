/**
 * Structural wiring for P0/P1 AI features.
 * Run: npx tsx src/lib/aiFeatures.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const panel = readFileSync(join(root, "components/article/AIResultPanel.vue"), "utf8");
const view = readFileSync(join(root, "views/ReaderView.vue"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");
const aisvc = readFileSync(
  join(root, "../bindings/lrss/internal/appsvc/aiservice.ts"),
  "utf8",
);

for (const key of [
  "aiSummarize",
  "aiTranslate",
  "aiAsk",
  "aiDailyDigest",
  "aiSuggest",
  "aiClassify",
  "aiApplyFolder",
  "aiPanel",
]) {
  assert(store.includes(key), `store exports/uses ${key}`);
}

assert(reader.includes("onSummarize") || reader.includes("aiSummarize"), "reader summarize");
assert(reader.includes("onTranslate") || reader.includes("aiTranslate"), "reader translate");
assert(reader.includes("onAsk") || reader.includes("aiAsk"), "reader ask");
assert(reader.includes("onSuggest") || reader.includes("aiSuggest"), "reader suggest");
assert(reader.includes("onClassify") || reader.includes("aiClassify"), "reader classify");
assert(reader.includes("Sparkles") || reader.includes("ai.menu"), "reader AI menu");

assert(panel.includes("AIResultPanel") || panel.includes("ai.panelTitle") || panel.includes("content"), "AI panel");
assert(view.includes("AIResultPanel"), "ReaderView hosts AI panel");
assert(view.includes("aiPanel"), "ReaderView binds aiPanel");

assert(sidebar.includes("aiDailyDigest") || sidebar.includes("onDailyDigest"), "sidebar digest");
assert(sidebar.includes("dailyDigest") || sidebar.includes("ai.dailyDigest"), "sidebar digest label");

for (const [name, src] of [
  ["zh", zh],
  ["en", en],
] as const) {
  for (const k of [
    "summarize:",
    "translate:",
    "ask:",
    "dailyDigest:",
    "suggest:",
    "classify:",
  ]) {
    assert(src.includes(k), `${name} i18n has ${k}`);
  }
}

for (const fn of [
  "Summarize",
  "Translate",
  "Ask",
  "DailyDigest",
  "SuggestFolders",
  "ClassifyPromo",
  "ApplySuggestedFolder",
]) {
  assert(aisvc.includes(`function ${fn}`), `binding ${fn}`);
}

console.log("aiFeatures.selftest: OK");
