/**
 * Next auto-refresh estimate for the status bar.
 * Mirrors internal/service FeedRefreshDue / EffectiveRefreshMinutes / phase.
 */

export type RefreshableFeed = {
  id: string;
  title: string;
  lastFetchedAt?: string;
  isPaused?: boolean;
  refreshIntervalMinutes?: number;
};

export type NextRefresh = {
  feedId: string;
  title: string;
  at: number;
  /** Whole minutes until due; 0 = already due / overdue. */
  minutes: number;
};

const FNV_OFFSET = 2166136261;
const FNV_PRIME = 16777619;

/** FNV-1a 32 — same as Go hash/fnv.New32a. */
export function fnv1a32(s: string): number {
  let h = FNV_OFFSET;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i) & 0xff;
    h = Math.imul(h, FNV_PRIME) >>> 0;
  }
  return h >>> 0;
}

export function effectiveRefreshMinutes(
  feed: RefreshableFeed,
  defaultMinutes: number,
): number {
  const per = feed.refreshIntervalMinutes ?? 0;
  if (per > 0) return clampInterval(per);
  return clampInterval(defaultMinutes);
}

function clampInterval(n: number): number {
  if (!Number.isFinite(n) || n < 5) return 5;
  if (n > 180) return 180;
  return Math.floor(n);
}

export function refreshPhaseMinutes(feedId: string, interval: number): number {
  if (interval <= 1) return 0;
  return fnv1a32(feedId) % interval;
}

function parseLastFetched(raw: string | undefined): number | null {
  const s = String(raw ?? "").trim();
  if (!s) return null;
  const t = Date.parse(s);
  return Number.isNaN(t) ? null : t;
}

function minuteEpoch(ms: number): number {
  return Math.floor(ms / 60_000);
}

/**
 * Earliest time this feed becomes eligible for auto-refresh at/after `now`.
 * Never-fetched and 2×-overdue feeds are due immediately.
 */
export function nextRefreshAt(
  feed: RefreshableFeed,
  defaultMinutes: number,
  now: number,
): number {
  const interval = effectiveRefreshMinutes(feed, defaultMinutes);
  const intervalMs = interval * 60_000;
  const last = parseLastFetched(feed.lastFetchedAt);
  if (last == null) return now;

  const age = now - last;
  if (age >= intervalMs * 2) return now;

  const ageDueAt = last + intervalMs;
  const start = Math.max(now, ageDueAt);
  const catchUpAt = last + intervalMs * 2;
  const phase = refreshPhaseMinutes(feed.id, interval);

  const startMin = minuteEpoch(start);
  for (let i = 0; i <= interval; i++) {
    const m = startMin + i;
    const at = m * 60_000;
    if (at >= catchUpAt) return catchUpAt;
    if (m % interval === phase) return Math.max(at, start);
  }
  return Math.min(catchUpAt, start + intervalMs);
}

export function remainingMinutes(at: number, now: number): number {
  const ms = at - now;
  if (ms <= 0) return 0;
  return Math.max(1, Math.ceil(ms / 60_000));
}

/** Soonest non-paused feed. Overdue feeds win; then earliest `at`, then title. */
export function pickNextRefresh(
  feeds: readonly RefreshableFeed[],
  defaultMinutes: number,
  now: number,
): NextRefresh | null {
  let best: NextRefresh | null = null;
  for (const f of feeds) {
    if (f.isPaused) continue;
    const id = String(f.id ?? "").trim();
    if (!id) continue;
    const at = nextRefreshAt(f, defaultMinutes, now);
    const minutes = remainingMinutes(at, now);
    const title = String(f.title ?? "").trim() || id;
    const cand: NextRefresh = { feedId: id, title, at, minutes };
    if (!best) {
      best = cand;
      continue;
    }
    if (cand.at !== best.at) {
      if (cand.at < best.at) best = cand;
      continue;
    }
    if (cand.title.localeCompare(best.title) < 0) best = cand;
  }
  return best;
}
