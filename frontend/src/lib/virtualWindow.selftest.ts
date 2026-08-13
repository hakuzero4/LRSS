/**
 * Run: npx tsx src/lib/virtualWindow.selftest.ts
 */
import { virtualWindow } from "./virtualWindow.ts";

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const empty = virtualWindow({ count: 0, scrollTop: 0, viewportH: 280, itemH: 68 });
assert(empty.start === 0 && empty.end === 0 && empty.totalH === 0, "empty");

const top = virtualWindow({
  count: 400,
  scrollTop: 0,
  viewportH: 280,
  itemH: 68,
  overscan: 4,
});
assert(top.start === 0, "top start");
assert(top.end < 40, `top window small, end=${top.end}`);
assert(top.totalH === 400 * 68, "total height");

const mid = virtualWindow({
  count: 400,
  scrollTop: 68 * 50,
  viewportH: 280,
  itemH: 68,
  overscan: 4,
});
assert(mid.start >= 40 && mid.start <= 50, `mid start ${mid.start}`);
assert(mid.end - mid.start < 30, `mid span ${mid.end - mid.start}`);
assert(mid.padTop === mid.start * 68, "padTop");

const bottom = virtualWindow({
  count: 20,
  scrollTop: 99999,
  viewportH: 280,
  itemH: 68,
  overscan: 8,
});
assert(bottom.end === 20, "clamped end");
assert(bottom.start <= bottom.end, "start <= end when overscrolled");

console.log("virtualWindow.selftest: OK");
