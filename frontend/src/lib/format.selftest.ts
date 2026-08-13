/**
 * Run: npx tsx src/lib/format.selftest.ts
 */
import { relativeTime } from "./format.ts";

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const now = Date.now();
const minAgo = new Date(now - 5 * 60 * 1000).toISOString();
const a = relativeTime(minAgo, now);
const b = relativeTime(minAgo, now);
assert(a === b, "stable");
assert(a.length > 0, "non-empty");

console.log("format.selftest: OK");
