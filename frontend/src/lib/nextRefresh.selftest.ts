/**
 * Run: npx tsx src/lib/nextRefresh.selftest.ts
 */
import {
  effectiveRefreshMinutes,
  fnv1a32,
  nextRefreshAt,
  pickNextRefresh,
  refreshPhaseMinutes,
  remainingMinutes,
} from "./nextRefresh";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// Matches Go hash/fnv.New32a + refreshPhaseMinutes("feed-a", 15).
assert(fnv1a32("feed-a") === 144130555, "fnv1a32 feed-a");
assert(refreshPhaseMinutes("feed-a", 15) === 10, "phase feed-a / 15");
assert(refreshPhaseMinutes("x", 1) === 0, "interval 1 phase is 0");

assert(effectiveRefreshMinutes({ id: "a", title: "A" }, 30) === 30, "global default");
assert(effectiveRefreshMinutes({ id: "a", title: "A" }, 2) === 5, "global floor");
assert(effectiveRefreshMinutes({ id: "a", title: "A", refreshIntervalMinutes: 15 }, 60) === 15, "per-feed");
assert(effectiveRefreshMinutes({ id: "a", title: "A", refreshIntervalMinutes: 1 }, 60) === 5, "per-feed floor");

const now = Date.parse("2026-01-01T12:00:00Z");

const young = {
  id: "feed-a",
  title: "A",
  refreshIntervalMinutes: 15,
  lastFetchedAt: "2026-01-01T11:50:00Z",
};
assert(nextRefreshAt(young, 30, now) > now, "10m-old 15m interval not due yet");

const aged = {
  id: "feed-a",
  title: "A",
  refreshIntervalMinutes: 15,
  lastFetchedAt: "2026-01-01T11:40:00Z",
};
let dueOnce = false;
for (let min = 0; min < 15; min++) {
  const probe = now + min * 60_000;
  if (nextRefreshAt(aged, 30, probe) <= probe) {
    dueOnce = true;
    break;
  }
}
assert(dueOnce, "age-ok feed becomes due on a phase slot within interval");

const overdue = {
  id: "feed-a",
  title: "A",
  refreshIntervalMinutes: 15,
  lastFetchedAt: "2026-01-01T11:00:00Z",
};
assert(nextRefreshAt(overdue, 30, now) <= now, "2x overdue is due now");

const never = { id: "feed-a", title: "Fresh" };
assert(nextRefreshAt(never, 30, now) <= now, "never fetched is due now");

assert(remainingMinutes(now - 1000, now) === 0, "past → 0");
assert(remainingMinutes(now + 10_000, now) === 1, "10s → 1 minute");
assert(remainingMinutes(now + 90_000, now) === 2, "90s → 2 minutes");

const picked = pickNextRefresh(
  [
    { id: "later", title: "Later", lastFetchedAt: "2026-01-01T11:50:00Z", refreshIntervalMinutes: 60 },
    { id: "soon", title: "Soon", lastFetchedAt: "2026-01-01T11:00:00Z", refreshIntervalMinutes: 15 },
    { id: "paused", title: "Paused", isPaused: true, lastFetchedAt: "2026-01-01T10:00:00Z" },
  ],
  30,
  now,
);
assert(picked?.feedId === "soon", `soonest is overdue Soon, got ${picked?.feedId}`);
assert(picked?.minutes === 0, "overdue minutes is 0");

const none = pickNextRefresh(
  [{ id: "p", title: "P", isPaused: true, lastFetchedAt: "2026-01-01T10:00:00Z" }],
  30,
  now,
);
assert(none === null, "all paused → null");

console.log("nextRefresh.selftest: OK");
