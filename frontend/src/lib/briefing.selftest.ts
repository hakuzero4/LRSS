/**
 * Structural wiring for 智能汇报 / smart briefing.
 * Run: npx tsx src/lib/briefing.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const panel = readFileSync(join(root, "components/settings/panels/AIFeaturesPanel.vue"), "utf8");
const view = readFileSync(join(root, "views/ReaderView.vue"), "utf8");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const types = readFileSync(join(root, "types/rss.ts"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");
const list = readFileSync(join(root, "components/briefing/BriefingList.vue"), "utf8");
const reader = readFileSync(join(root, "components/briefing/BriefingReader.vue"), "utf8");
const articleList = readFileSync(join(root, "components/article/ArticleList.vue"), "utf8");

assert(sidebar.includes('id: "briefing"'), "AppSidebar includes id briefing");
assert(sidebar.includes("smartBriefing"), "AppSidebar gates briefing on smartBriefing");

assert(panel.includes("smartBriefing"), "AIFeaturesPanel includes smartBriefing");

assert(view.includes("collectionId") && view.includes("briefing"), "ReaderView special-cases briefing collection");
assert(view.includes("BriefingList"), "ReaderView hosts BriefingList");
assert(view.includes("BriefingReader"), "ReaderView hosts BriefingReader");

assert(store.includes("reloadBriefings"), "store has reloadBriefings");
assert(store.includes("reloadBriefingCount"), "store has reloadBriefingCount");
assert(!store.includes('List("briefing")'), 'store must not List("briefing")');
assert(!store.includes("List('briefing')"), "store must not List('briefing')");
assert(store.includes('id === "briefing"') || store.includes("id === 'briefing'"), "selectCollection branches on briefing");
assert(store.includes('collectionId.value === "briefing"'), "reloadArticles skips briefing collection");
assert(store.includes("v !== \"briefing\"") || store.includes("v !== 'briefing'"), "isSmartCollection rejects briefing");
assert(store.includes("smartBriefing: settings.smartBriefing"), "buildUIPrefs persists smartBriefing");
assert(store.includes("smartBriefing") && store.includes("SmartBriefing"), "applyUIPrefs pickBool smartBriefing");

assert(types.includes("smartBriefing"), "types include smartBriefing");
assert(types.includes("export interface Briefing"), "types include Briefing");
assert(types.includes('| "briefing"') || types.includes("| 'briefing'"), "SmartCollectionId includes briefing");

assert(list.includes("selectBriefing"), "BriefingList selects briefings");
assert(list.includes("ContextMenu") && list.includes("deleteBriefing"), "BriefingList context-menu delete");
assert(list.includes("deleteUnstarredBriefings") && list.includes("briefing.clear"), "BriefingList header can clear unstarred");
assert(list.includes("deleteStarredBlocked"), "BriefingList blocks deleting starred rows");
assert(reader.includes("selectArticle"), "BriefingReader opens articles from bullets");
assert(reader.includes("keepBriefing"), "cite from briefing keeps briefing selection");
assert(view.includes("backToBriefing") || view.includes("briefing.backToBriefing"), "return-to-briefing control");
assert(view.includes("briefingActive"), "ReaderView opens briefing reader from starred");
assert(store.includes("deleteUnstarredBriefings"), "store can clear unstarred briefings");
assert(store.includes("starredBriefings"), "store exposes starred briefings for 收藏");
assert(store.includes("composeStarredBadge"), "starred badge includes briefings");
assert(articleList.includes("visibleStarredBriefings"), "收藏 list renders starred briefings");
assert(articleList.includes("selectBriefing"), "收藏 list opens a starred briefing");

for (const [name, src] of [
  ["zh", zh],
  ["en", en],
] as const) {
  for (const k of [
    "briefing:",
    "smartBriefing:",
    "backToBriefing:",
    "watch:",
    "omitted:",
    "clearTitle:",
    "deleteStarredBlocked:",
    "inStarred:",
  ]) {
    assert(src.includes(k), `${name} i18n has ${k}`);
  }
}

console.log("briefing.selftest: OK");
