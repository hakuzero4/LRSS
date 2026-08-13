/**
 * Self-test for folder menu pure helpers + structural wiring checks.
 * Run: npx tsx src/lib/folderMenu.selftest.ts
 * Exit 0 only when all assertions pass.
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  FOLDER_MENU_ACTIONS,
  articleCardImage,
  feedIdsInFolder,
  folderCollectionId,
  folderIdForDisplayMode,
  normalizeFolderDisplayMode,
  resolveCollectionDisplayMode,
  unreadInFolder,
} from "./folderMenu";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// —— pure helpers (shipped code) ——
assert(folderCollectionId("abc") === "folder:abc", "folderCollectionId");
assert(
  JSON.stringify(feedIdsInFolder("f1", [
    { id: "a", folderId: "f1" },
    { id: "b", folderId: "f2" },
    { id: "c", folderId: "f1" },
    { id: "d" },
  ])) === JSON.stringify(["a", "c"]),
  "feedIdsInFolder",
);
assert(normalizeFolderDisplayMode("cards") === "cards", "cards");
assert(normalizeFolderDisplayMode("gallery") === "cards", "gallery alias");
assert(normalizeFolderDisplayMode("") === "list", "empty is list");
assert(
  resolveCollectionDisplayMode(
    "folder:f1",
    [{ id: "f1", displayMode: "cards" }],
    [],
  ) === "cards",
  "folder collection cards",
);
assert(
  resolveCollectionDisplayMode(
    "feed:a",
    [{ id: "f1", displayMode: "cards" }],
    [{ id: "a", folderId: "f1" }],
  ) === "cards",
  "feed inherits folder cards",
);
assert(
  resolveCollectionDisplayMode("unread", [{ id: "f1", displayMode: "cards" }], []) ===
    "list",
  "smart lists stay list",
);
assert(folderIdForDisplayMode("folder:f1", []) === "f1", "folder id from collection");
assert(
  folderIdForDisplayMode("feed:a", [{ id: "a", folderId: "f1" }]) === "f1",
  "folder id from feed",
);
assert(folderIdForDisplayMode("unread", []) === "", "smart list has no folder");
assert(
  articleCardImage({ imageUrl: "https://ex.com/a.jpg" }) === "https://ex.com/a.jpg",
  "direct image",
);
assert(
  articleCardImage({
    contentHtml: '<p>x</p><img src="https://ex.com/b.png" alt="">',
  }) === "https://ex.com/b.png",
  "html image",
);

assert(
  unreadInFolder("f1", [
    { folderId: "f1", unreadCount: 3 },
    { folderId: "f1", unreadCount: 2 },
    { folderId: "x", unreadCount: 9 },
  ]) === 5,
  "unreadInFolder",
);
assert(
  FOLDER_MENU_ACTIONS.includes("open") &&
    FOLDER_MENU_ACTIONS.includes("markAllRead") &&
    FOLDER_MENU_ACTIONS.includes("refresh") &&
    FOLDER_MENU_ACTIONS.includes("addFeed") &&
    FOLDER_MENU_ACTIONS.includes("rename") &&
    FOLDER_MENU_ACTIONS.includes("delete") &&
    FOLDER_MENU_ACTIONS.includes("toggleExpand"),
  "P0 actions present",
);

// —— structural: store + sidebar wire real APIs ——
const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");

for (const name of [
  "renameFolder",
  "deleteFolder",
  "markFolderRead",
  "refreshFolderFeeds",
  "openAddFeedInFolder",
  "addFeedFromURL",
  "MarkAllRead",
  "RenameFolder",
  "SetFolderDisplayMode",
  "DeleteFolder",
  "RefreshFeed",
]) {
  assert(store.includes(name), `store must reference ${name}`);
}

assert(
  store.includes("MarkAllRead") && store.includes("folderCollectionId"),
  "markFolderRead must call MarkAllRead via folderCollectionId",
);
assert(store.includes("AddFeed") && store.includes("addFeedTargetFolderId"), "add feed with folder target");
assert(store.includes("feedIdsInFolder"), "refreshFolderFeeds uses feedIdsInFolder");

assert(sidebar.includes("ContextMenu"), "sidebar ContextMenu");
assert(sidebar.includes("renameFolder"), "sidebar renameFolder");
assert(sidebar.includes("deleteFolder"), "sidebar deleteFolder");
assert(sidebar.includes("markFolderRead"), "sidebar markFolderRead");
assert(sidebar.includes("refreshFolderFeeds"), "sidebar refreshFolderFeeds");
assert(sidebar.includes("openAddFeedInFolder"), "sidebar openAddFeedInFolder");
assert(sidebar.includes("setFolderDisplayMode") || sidebar.includes("onFolderDisplayMode"), "sidebar display mode");
const list = readFileSync(join(root, "components/article/ArticleList.vue"), "utf8");
assert(list.includes("toggleDisplayMode") && list.includes("LayoutGrid"), "list header display toggle");

for (const key of [
  "folderMenu.open",
  "folderMenu.expand",
  "folderMenu.collapse",
  "folderMenu.markAllRead",
  "folderMenu.refresh",
  "folderMenu.addFeed",
  "folderMenu.rename",
  "folderMenu.delete",
  "folderMenu.deleteConfirmTitle",
  "folderMenu.deleteConfirmBody",
  "folderMenu.displayMode",
  "folderMenu.displayCards",
]) {
  // i18n files use nested objects; keys appear as property names
  const leaf = key.split(".").pop()!;
  assert(zh.includes(leaf), `zh-CN missing ${leaf}`);
  assert(en.includes(leaf), `en-US missing ${leaf}`);
}

console.log("folderMenu.selftest: OK");
