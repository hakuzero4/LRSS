/**
 * Run: npx tsx src/lib/keepTree.selftest.ts
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
} from "./keepFolders.ts";
import { applyShowUnreadOnly } from "./readingSettings.ts";
import { mapArticle, mapKeepFolder, mapKeepFolders } from "./backend.ts";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const folders = [
  { id: "a", name: "Rust", sortOrder: 1 },
  { id: "b", name: "Go", parentId: "a", sortOrder: 0 },
  { id: "c", name: "News", sortOrder: 0 },
];

assert(keepCollectionId("abc") === "kept:abc", "keepCollectionId");
assert(parseKeepFolderId("kept") === null, "parse root");
assert(parseKeepFolderId("kept:xyz") === "xyz", "parse child");
assert(isKeptCollection("kept") && isKeptCollection("kept:x"), "isKeptCollection");
assert(!isKeptCollection("starred"), "not kept");
assert(firstLevelKeepFolders(folders).map((f) => f.id).join(",") === "c,a", "first-level sort");
assert(childKeepFolders(folders, "a").map((f) => f.id).join(",") === "b", "children");
assert(normalizeKeepParentId("a", folders) === "a", "valid parent");
assert(normalizeKeepParentId("b", folders) === undefined, "2nd-level not parent");
assert(keepFolderDescendantIds(folders, "a").join(",") === "a,b", "descendants");
assert(
  keepFolderOptions(folders)
    .map((o) => `${o.depth}:${o.id}`)
    .join(",") === "0:c,0:a,1:b",
  "options tree",
);

const mapped = mapArticle({
  id: "a1",
  feedId: "f1",
  title: "T",
  summary: "s",
  contentHtml: "<p>body distinct</p>",
  KeepFolderID: "kf1",
  isKept: true,
});
assert(mapped.keepFolderId === "kf1", "mapArticle KeepFolderID");

const kf = mapKeepFolder({ ID: "x", Name: "Lang", ParentID: "p", UnreadCount: 3 });
assert(kf.id === "x" && kf.name === "Lang" && kf.parentId === "p" && kf.unreadCount === 3, "mapKeepFolder");
assert(mapKeepFolders({ folders: [{ id: "1", name: "A" }] }).length === 1, "mapKeepFolders wrap");

assert(
  applyShowUnreadOnly([{ id: "1", read: true }], true, "kept:abc").length === 1,
  "kept: folder exempt from unread-only",
);

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const types = readFileSync(join(root, "types/rss.ts"), "utf8");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const filters = readFileSync(join(root, "components/settings/panels/FiltersPanel.vue"), "utf8");
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");
const http = readFileSync(join(root, "lib/httpAppsvc.ts"), "utf8");

assert(types.includes("`kept:${string}`") && types.includes("KeepFolder") && types.includes("keepFolderId"), "types");
assert(store.includes("keepFolders") && store.includes("kept:") && store.includes("createKeepFolder"), "store");
assert(store.includes("setKeepFolder") && store.includes("reloadKeepFolders"), "store methods");
assert(sidebar.includes("rootKeepFolders") && sidebar.includes("keepFolders"), "sidebar tree");
assert(sidebar.includes("createKeepFolder") || filters.includes("createKeepFolder"), "can create folder");
assert(reader.includes("setKeepFolder"), "reader setKeepFolder");
for (const key of ["new:", "rename:", "delete:", "deleteConfirm:", "moveTo:", "root:", "emptyName:"]) {
  assert(zh.includes(key), `zh keepFolder.${key}`);
  assert(en.includes(key), `en keepFolder.${key}`);
}
assert(zh.includes("keepFolder:") && en.includes("keepFolder:"), "keepFolder locale blocks");
assert(http.includes("ListKeepFolders") && http.includes("SetKeepFolder"), "http stubs");

console.log("keepTree.selftest: OK");
