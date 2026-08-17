/**
 * Run: npx tsx src/lib/smartFilter.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { applyShowUnreadOnly } from "./readingSettings";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const types = readFileSync(join(root, "types/rss.ts"), "utf8");
const filters = readFileSync(join(root, "components/settings/panels/FiltersPanel.vue"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");

assert(store.includes("smartFilterEnabled"), "store has smart filter pref");
assert(store.includes("toggleKeep"), "store can toggle keep");
assert(store.includes("scanUnreadForKeep"), "store can scan unread");
assert(store.includes('id === "kept"'), "store knows kept collection");
assert(store.includes("createKeepFolder"), "store can create keep folders");
assert(store.includes("setKeepFolder"), "store can move kept articles");
assert(filters.includes("smartFilterEnabled"), "filters panel binds toggle");
assert(filters.includes("smartFilterProfile"), "filters panel has profile");
assert(filters.includes("createKeepFolder"), "filters panel can add keep folders");
assert(filters.includes("activeTitle") || filters.includes("activeRules"), "filters panel shows live rules");
assert(sidebar.includes("rootKeepFolders"), "sidebar keep tree");
assert(sidebar.includes("onCreateKeepFolder"), "sidebar can create keep folder");
assert(reader.includes("setKeepFolder"), "reader can setKeepFolder");
assert(types.includes("KeepFolder") && types.includes("keepFolderId"), "types KeepFolder");
assert(types.includes("`kept:${string}`") && types.includes("KeepFolder"), "types kept: + KeepFolder");
assert(store.includes("keepFolders") && store.includes("kept:"), "store keepFolders / kept:");
assert(sidebar.includes("rootKeepFolders") && sidebar.includes("keepFolders"), "sidebar keepFolders tree");
assert(zh.includes('kept: "精选"'), "zh nav.kept");
assert(en.includes('kept: "Picks"'), "en nav.kept");
assert(zh.includes("keepFolder:") && zh.includes("emptyName:") && zh.includes("deleteConfirm:"), "zh keepFolder keys");
assert(en.includes("keepFolder:") && en.includes("emptyName:") && en.includes("moveTo:"), "en keepFolder keys");
assert(zh.includes("root:") && en.includes("root:"), "zh+en root label");

const keptRead = applyShowUnreadOnly(
  [
    { id: "1", read: true },
    { id: "2", read: false },
  ],
  true,
  "kept",
);
assert(keptRead.length === 2, "kept collection is exempt from unread-only");

console.log("smartFilter.selftest: OK");
