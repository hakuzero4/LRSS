/** Lazy load Wails-generated appsvc bindings. */

import { formatAuthor, plainText } from "./format";

export async function loadAppsvc(): Promise<any | null> {
  try {
    const mod = await import("../../bindings/lrss/internal/appsvc/index.js");
    return mod;
  } catch {
    return null;
  }
}

/**
 * Map a backend article into the frontend Article shape.
 * Summary is preserved in full for the reader deck (AI summaries can be long);
 * list teasers clamp at display time in ArticleListItem.
 */
export function mapArticle(a: any) {
  if (!a) return a;
  const rawSummary = a.summary ?? a.Summary ?? "";
  const contentHtml = a.contentHtml ?? a.ContentHTML ?? "";
  // Strip HTML tags if present, but do NOT hard-clamp length — AI deck text
  // is stored on summary and must survive list/get remapping.
  let summary = plainText(rawSummary);
  // Drop lead-in that duplicates the article body (feed standfirst noise).
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
    author: (() => {
      const araw = formatAuthor(a.author ?? a.Author ?? "");
      return araw || undefined;
    })(),
    summary,
    contentHtml,
    translationRaw: (() => {
      const t = a.translationRaw ?? a.TranslationRaw ?? "";
      return t ? String(t) : undefined;
    })(),
    translationLang: (() => {
      const t = a.translationLang ?? a.TranslationLang ?? "";
      return t ? String(t) : undefined;
    })(),
    url: a.url ?? a.URL ?? "",
    publishedAt: a.publishedAt ?? a.PublishedAt ?? a.fetchedAt ?? a.FetchedAt ?? new Date().toISOString(),
    read: !!(a.isRead ?? a.read),
    starred: !!(a.isStarred ?? a.starred),
    imageUrl: a.imageUrl ?? a.ImageURL ?? undefined,
    fullContentFetched: !!(a.fullContentFetched ?? a.FullContentFetched),
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
  const keepRaw = f.keepArticlesDays ?? f.KeepArticlesDays ?? 0;
  let keepDays =
    typeof keepRaw === "number" && Number.isFinite(keepRaw)
      ? Math.floor(keepRaw)
      : 0;
  if (keepDays < 0) keepDays = 0;
  if (keepDays > 0 && keepDays < 7) keepDays = 7;
  if (keepDays > 365) keepDays = 365;
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
    keepArticlesDays: keepDays,
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
