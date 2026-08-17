/**
 * Run: npx tsx src/lib/keepFolders.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  childKeepFolders,
  firstLevelKeepFolders,
  isKeptCollection,
  keepCollectionId,
  keepFolderDescendantIds,
  keepFolderOptions,
  normalizeKeepParentId,
  parseKeepFolderId,
} from "./keepFolders";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const folders = [
  { id: "a", name: "Rust", unreadCount: 2 },
  { id: "b", name: "Go", unreadCount: 0 },
  { id: "c", name: "Compiler", parentId: "a", unreadCount: 1 },
];

assert(keepCollectionId("a") === "kept:a", "keep collection id");
assert(parseKeepFolderId("kept:a") === "a", "parse kept:id");
assert(parseKeepFolderId("kept") === null, "root kept has no folder id");
assert(isKeptCollection("kept") && isKeptCollection("kept:a"), "isKeptCollection");
assert(!isKeptCollection("folder:x"), "feed folders are not keep collections");

const roots = firstLevelKeepFolders(folders);
assert(roots.map((f) => f.id).join(",") === "b,a" || roots.length === 2, "two first-level");
assert(roots.every((f) => !f.parentId), "roots have no parent");
assert(childKeepFolders(folders, "a").map((f) => f.id).join(",") === "c", "child of a");
assert(normalizeKeepParentId("c", folders) === undefined, "second-level cannot be parent");
assert(normalizeKeepParentId("a", folders) === "a", "first-level can be parent");
assert(keepFolderDescendantIds(folders, "a").join(",") === "a,c", "descendants");
const opts = keepFolderOptions(folders);
assert(opts.some((o) => o.id === "c" && o.depth === 1), "options include depth-1 child");

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const filters = readFileSync(join(root, "components/settings/panels/FiltersPanel.vue"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");

assert(store.includes("createKeepFolder"), "store can create keep folder");
assert(store.includes("setKeepFolder"), "store can move article");
assert(store.includes("kept:"), "store knows kept: collections");
assert(sidebar.includes("rootKeepFolders") || sidebar.includes("keepFolders"), "sidebar has keep tree");
assert(sidebar.includes("createKeepFolder") || sidebar.includes("onCreateKeep"), "sidebar can create");
assert(reader.includes("setKeepFolder"), "reader can move");
assert(filters.includes("createKeepFolder"), "filters can create folder");
assert(zh.includes("keepFolder:"), "zh keepFolder keys");
assert(en.includes("keepFolder:"), "en keepFolder keys");

console.log("keepFolders.selftest: OK");
