/**
 * Run: npx tsx src/lib/filterStatus.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  formatKeepConfidence,
  keepConfidenceThreshold,
  parseKeepLog,
  profileIsEmpty,
  profilePreview,
  smartKeepState,
} from "./filterStatus";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

assert(smartKeepState(false, true) === "off", "off");
assert(smartKeepState(true, false) === "need-model", "need model");
assert(smartKeepState(true, true) === "on", "on");
assert(keepConfidenceThreshold("loose") === 0.55, "loose");
assert(keepConfidenceThreshold("strict") === 0.85, "strict");
assert(keepConfidenceThreshold("standard") === 0.7, "standard");
assert(profileIsEmpty("  "), "empty profile");
assert(!profileIsEmpty("Go and Rust"), "set profile");
assert(profilePreview("abcdefghij", 6) === "abcde…", `preview ${profilePreview("abcdefghij", 6)}`);
assert(formatKeepConfidence(0.82) === "82", "pct");
assert(formatKeepConfidence(0) === "", "zero conf hidden");

const log = parseKeepLog([
  { articleId: "a", title: "T", outcome: "kept", gate: "ai", reason: "fit", confidence: 0.9, folder: "Go" },
  { ArticleID: "b", Title: "Skip", Outcome: "skipped", Gate: "keyword", Reason: "ads" },
  { articleId: "a", title: "dup" },
]);
assert(log.length === 2, `log len ${log.length}`);
assert(log[0].folder === "Go" && log[0].outcome === "kept", "kept row");
assert(log[1].gate === "keyword" && log[1].reason === "ads", "skip row");

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const filters = readFileSync(join(root, "components/settings/panels/FiltersPanel.vue"), "utf8");
const list = readFileSync(join(root, "components/article/ArticleList.vue"), "utf8");
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const bar = readFileSync(join(root, "components/layout/ActivityBar.vue"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");

assert(filters.includes("activeRules"), "filters panel lists live rules");
assert(filters.includes("keepLog"), "filters panel shows keep log");
assert(list.includes("hiddenByFilters"), "list can peek hidden rows");
assert(reader.includes("keepExplainText"), "reader explains keep/skip");
assert(bar.includes("keepText"), "activity bar shows keep progress");
assert(zh.includes("activeTitle:") && en.includes("activeTitle:"), "i18n active title");
assert(zh.includes("logTitle:") && en.includes("logTitle:"), "i18n log title");
assert(zh.includes("keepJudging:") && en.includes("keepJudging:"), "i18n keep activity");
assert(zh.includes("errTimeout:") && en.includes("errTimeout:"), "i18n keep timeout");
assert(filters.includes("keepErrorText"), "filters panel shows keep errors");

console.log("filterStatus.selftest: OK");
