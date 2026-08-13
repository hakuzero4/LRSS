/** Lazy load Wails-generated appsvc bindings, or HTTP adapter in web mode. */

import { formatAuthor, plainText } from "./format";
import { normalizeFolderDisplayMode } from "./folderMenu";
import { tryHttpAppsvc } from "./httpAppsvc";
import {
  captureWebTokenFromURL,
  isWailsRuntime,
  isWebMode,
  setWebAuthState,
  setWebModeFlag,
  webAuthState,
} from "./webMode";

let cached: any | null | undefined;

function forcedWebMode(): boolean {
  try {
    return !!(globalThis as unknown as { __LRSS_WEB__?: boolean }).__LRSS_WEB__;
  } catch {
    return false;
  }
}

export async function loadAppsvc(): Promise<any | null> {
  if (cached !== undefined) return cached;

  captureWebTokenFromURL();

  // Browser web-access: server injects window.__LRSS_WEB__ into index.html.
  // Prefer HTTP adapter so bundled Wails bindings are never used in a real browser.
  if (forcedWebMode()) {
    const http = await tryHttpAppsvc();
    if (http) {
      setWebModeFlag(true);
      cached = http;
      return cached;
    }
    setWebModeFlag(true);
    // tryHttpAppsvc already sets unauthorized on 401; leave other failures as-is
    // so a down server is not mislabeled as a bad token.
    if (webAuthState.value === "pending") {
      setWebAuthState("none");
    }
    cached = null;
    return null;
  }

  // Desktop WebView (wails.localhost): always use generated bindings.
  // Do not probe /api/meta here — that path is only on the web-access server.
  if (isWailsRuntime()) {
    try {
      const mod = await import("../../bindings/lrss/internal/appsvc/index.js");
      if (mod && (mod.FeedService || mod.SettingsService || mod.ArticleService)) {
        setWebModeFlag(false);
        setWebAuthState("ok");
        cached = mod;
        return cached;
      }
    } catch {
      /* bindings missing */
    }
    if (webAuthState.value !== "unauthorized") {
      setWebAuthState("none");
    }
    cached = null;
    return null;
  }

  // Real browser (web access or Vite preview): HTTP adapter on this origin.
  const http = await tryHttpAppsvc();
  if (http) {
    setWebModeFlag(true);
    cached = http;
    return cached;
  }

  setWebModeFlag(false);
  if (webAuthState.value !== "unauthorized") {
    setWebAuthState("none");
  }
  cached = null;
  return null;
}

/** True after loadAppsvc resolved to the HTTP adapter. */
export function isHttpBackend(): boolean {
  return isWebMode();
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
    displayMode: normalizeFolderDisplayMode(f.displayMode ?? f.DisplayMode),
  };
}
