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
assert(layout.includes("ActivityBar"), "AppLayout mounts ActivityBar");
assert(bar.includes("activity.refreshCurrent"), "bar uses refresh copy");
assert(bar.includes("activity.briefingGenerating"), "bar uses briefing copy");
assert(zh.includes("activity:"), "zh activity i18n");
assert(en.includes("activity:"), "en activity i18n");

console.log("activity.selftest: OK");
