/** Lazy load Wails-generated appsvc bindings. */

import { plainText } from "./format";

export async function loadAppsvc(): Promise<any | null> {
  try {
    const mod = await import("../../bindings/lrss/internal/appsvc/index.js");
    return mod;
  } catch {
    return null;
  }
}

export function mapArticle(a: any) {
  if (!a) return a;
  const rawSummary = a.summary ?? a.Summary ?? "";
  const contentHtml = a.contentHtml ?? a.ContentHTML ?? "";
  // Always expose plain summary (legacy rows may still store HTML).
  let summary = plainText(rawSummary, 320);
  // Drop lead-in that duplicates the article body.
  const bodyText = plainText(contentHtml, 400);
  if (summary && bodyText && (bodyText === summary || bodyText.startsWith(summary.slice(0, Math.min(80, summary.length))))) {
    // Keep a short teaser only when much shorter than body; else hide in list via empty.
    if (summary.length > bodyText.length * 0.85) {
      summary = bodyText.slice(0, 200).replace(/\s+\S*$/, "") + (bodyText.length > 200 ? "…" : "");
    }
  }
  return {
    id: a.id,
    feedId: a.feedId,
    title: plainText(a.title ?? a.Title ?? ""),
    author: a.author ?? a.Author ?? undefined,
    summary,
    contentHtml,
    url: a.url ?? a.URL ?? "",
    publishedAt: a.publishedAt ?? a.PublishedAt ?? a.fetchedAt ?? a.FetchedAt ?? new Date().toISOString(),
    read: !!(a.isRead ?? a.read),
    starred: !!(a.isStarred ?? a.starred),
    imageUrl: a.imageUrl ?? a.ImageURL ?? undefined,
  };
}

export function mapFeed(f: any) {
  if (!f) return f;
  const fav = f.faviconUrl ?? f.favicon ?? undefined;
  const intervalRaw = f.refreshIntervalMinutes ?? f.RefreshIntervalMinutes ?? 0;
  const interval =
    typeof intervalRaw === "number" && Number.isFinite(intervalRaw)
      ? Math.max(0, Math.floor(intervalRaw))
      : 0;
  const lastErr = f.lastError ?? f.LastError;
  return {
    id: f.id,
    title: f.title ?? "",
    siteUrl: f.siteUrl ?? f.feedUrl ?? "",
    feedUrl: f.feedUrl ?? "",
    favicon: typeof fav === "string" && fav.trim() ? fav.trim() : undefined,
    folderId: f.folderId ?? undefined,
    unreadCount: f.unreadCount ?? 0,
    lastFetchedAt: f.lastFetchedAt ?? f.LastFetchedAt ?? "",
    isPaused: !!(f.isPaused ?? f.IsPaused),
    refreshIntervalMinutes: interval,
    lastError: typeof lastErr === "string" && lastErr.trim() ? lastErr.trim() : undefined,
    isNsfw: !!(f.isNsfw ?? f.IsNsfw),
  };
}

export function mapFolder(f: any, feeds: { id: string; folderId?: string }[]) {
  const feedIds = feeds.filter((x) => x.folderId === f.id).map((x) => x.id);
  return {
    id: f.id,
    name: f.name ?? "",
    feedIds,
    isNsfw: !!(f.isNsfw ?? f.IsNsfw),
  };
}
