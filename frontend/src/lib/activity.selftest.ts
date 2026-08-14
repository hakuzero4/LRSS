/**
 * Run: npx tsx src/lib/activity.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const layout = readFileSync(join(root, "layouts/AppLayout.vue"), "utf8");
const bar = readFileSync(join(root, "components/layout/ActivityBar.vue"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");

assert(store.includes("JobActivity"), "store JobActivity type");
assert(store.includes("startJobActivityPoll"), "store polls activity");
assert(store.includes("FeedService?.JobActivity"), "store calls FeedService.JobActivity");
assert(store.includes("articlesAdded"), "activity carries insert counter");
assert(store.includes("syncLibraryAfterNewArticles"), "poll refreshes unread after inserts");
assert(store.includes("mergeLatestArticles"), "unread list merges new rows");
assert(zh.includes("refreshFoundNew"), "zh toast for new articles");
assert(en.includes("refreshFoundNew"), "en toast for new articles");
assert(layout.includes("ActivityBar"), "AppLayout mounts ActivityBar");
assert(layout.includes("AssistantPane"), "AppLayout mounts global assistant");
assert(bar.includes("activity.refreshCurrent"), "bar uses refresh copy");
assert(bar.includes("activity.briefingGenerating"), "bar uses briefing copy");
assert(bar.includes("nextDueText"), "bar shows next auto-refresh countdown");
assert(bar.includes("pickNextRefresh"), "bar uses nextRefresh helper");
assert(zh.includes("nextDue:"), "zh next-due copy");
assert(en.includes("nextDue:"), "en next-due copy");
assert(zh.includes("autoRefreshOff:"), "zh auto-refresh-off copy");
assert(!bar.includes('v-if="visible"'), "status bar stays mounted when idle");
assert(bar.includes("selectedArticle"), "status bar reads the open article");
assert(bar.includes("currentText"), "status bar shows open-item meta on the right");
assert(bar.includes("setNsfwMode"), "status bar owns NSFW / show-all toggle");
assert(bar.includes("nav.nsfwVisible"), "status bar shows 全部显示");
assert(bar.includes("nav.officeMode"), "status bar shows office/NSFW mode");
assert(zh.includes("activity:"), "zh activity i18n");
assert(en.includes("activity:"), "en activity i18n");

console.log("activity.selftest: OK");
