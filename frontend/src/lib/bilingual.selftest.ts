/**
 * Run: npx tsx src/lib/bilingual.selftest.ts
 */
import { parseBilingualPairs } from "./bilingual.ts";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const marked = `
<<o>> As long as you're alive, there's no bad ending.
<<t>> 只要你还活着，就没有真正的结局。

<<o>> I believe comedians never truly retire.
<<t>> 我相信喜剧演员永远不会真正退休。
`;
const pairs = parseBilingualPairs(marked);
assert(pairs.length === 2, "two pairs");
assert(pairs[0]!.original.includes("alive"), "orig");
assert(pairs[0]!.translation.includes("活着"), "zh");
assert(pairs[1]!.translation.includes("喜剧"), "pair2");

const loose = "Hello.\n你好。\n\nWorld.\n世界。";
const p2 = parseBilingualPairs(loose);
assert(p2.length >= 2, "fallback blocks");

console.log("bilingual.selftest: OK");
