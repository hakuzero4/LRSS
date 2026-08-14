/**
 * NSFW / office-mode helpers — real pure tests.
 * Run: npx tsx src/lib/nsfw.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  filterArticlesByNsfwMode,
  filterFeedsForSidebar,
  filterFoldersForSidebar,
  shouldExcludeNsfwArticles,
} from "./nsfw";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const folders = [
  { id: "f1", isNsfw: false },
  { id: "f2", isNsfw: true },
];

const feeds = [
  { id: "a", isNsfw: false, folderId: "f1" },
  { id: "b", isNsfw: true, folderId: "f1" },
  { id: "c", isNsfw: true },
  { id: "d", isNsfw: false, folderId: "f2" }, // clean feed, NSFW folder
];

assert(filterFoldersForSidebar(folders, true).length === 2, "nsfw on shows all folders");
assert(
  filterFoldersForSidebar(folders, false)
    .map((f) => f.id)
    .join(",") === "f1",
  "office hides nsfw folders",
);

assert(filterFeedsForSidebar(feeds, true, folders).length === 4, "nsfw on shows all feeds");
assert(
  filterFeedsForSidebar(feeds, false, folders)
    .map((f) => f.id)
    .join(",") === "a",
  "office hides nsfw feeds + feeds in nsfw folders",
);

assert(shouldExcludeNsfwArticles(true) === false, "show mode no exclude");
assert(shouldExcludeNsfwArticles(false) === true, "office exclude");

const articles = [
  { id: "1", feedId: "a" },
  { id: "2", feedId: "b" },
  { id: "3", feedId: "c" },
  { id: "4", feedId: "d" },
];
assert(filterArticlesByNsfwMode(articles, feeds, true, folders).length === 4, "articles all");
assert(
  filterArticlesByNsfwMode(articles, feeds, false, folders)
    .map((a) => a.id)
    .join(",") === "1",
  "articles office hide feed+folder nsfw",
);

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const activity = readFileSync(join(root, "components/layout/ActivityBar.vue"), "utf8");
const general = readFileSync(join(root, "components/settings/panels/GeneralPanel.vue"), "utf8");
const feedsPanel = readFileSync(join(root, "components/settings/panels/FeedsPanel.vue"), "utf8");

assert(store.includes("nsfwMode"), "store nsfwMode");
assert(store.includes("setFeedNsfw"), "store setFeedNsfw");
assert(store.includes("setFolderNsfw"), "store setFolderNsfw");
assert(store.includes("sidebarFeeds"), "store sidebarFeeds");
assert(store.includes("sidebarFolders"), "store sidebarFolders");
assert(store.includes("filterFeedsForSidebar"), "store uses filter");
assert(sidebar.includes("sidebarFeeds"), "sidebar uses sidebarFeeds");
assert(sidebar.includes("sidebarFolders"), "sidebar uses sidebarFolders");
assert(sidebar.includes("setFeedNsfw") || sidebar.includes("onFeedNsfwToggle"), "sidebar nsfw menu");
assert(
  sidebar.includes("setFolderNsfw") || sidebar.includes("onFolderNsfwToggle"),
  "sidebar folder nsfw menu",
);
assert(!sidebar.includes("onToggleOfficeMode"), "office/NSFW mode is not in the sidebar");
assert(!sidebar.includes("nav.nsfwVisible"), "show-all chip is not in the sidebar");
assert(activity.includes("setNsfwMode"), "footer toggles nsfwMode");
assert(activity.includes("nav.nsfwVisible"), "footer shows 全部显示");
assert(activity.includes("nav.officeMode"), "footer shows office mode");
assert(general.includes("nsfwMode"), "general toggle");
assert(feedsPanel.includes("editNsfw") || feedsPanel.includes("setFeedNsfw"), "feeds edit nsfw");
// settings list uses raw feeds not sidebarFeeds
assert(feedsPanel.includes("feeds.value") || feedsPanel.includes("sortedFeeds"), "settings full list");

console.log("nsfw.selftest: OK");
