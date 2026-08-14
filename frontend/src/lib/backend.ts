/** Lazy load Wails-generated appsvc bindings, or HTTP adapter in web mode. */

import { formatAuthor, plainText } from "./format";
import { normalizeFolderDisplayMode } from "./folderMenu";
import { tryHttpAppsvc } from "./httpAppsvc";
import type {
  Briefing,
  BriefingBullet,
  BriefingCite,
  BriefingPayload,
  BriefingTheme,
} from "@/types/rss";
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
    id: String(a.id ?? a.ID ?? a.Id ?? ""),
    feedId: String(a.feedId ?? a.FeedID ?? a.FeedId ?? ""),
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

function pickStr(obj: Record<string, unknown>, ...keys: string[]): string {
  for (const k of keys) {
    if (!(k in obj)) continue;
    const v = obj[k];
    if (v == null) continue;
    const s = String(v).trim();
    if (s) return s;
  }
  return "";
}

function pickNum(obj: Record<string, unknown>, ...keys: string[]): number {
  for (const k of keys) {
    if (!(k in obj)) continue;
    const v = obj[k];
    if (typeof v === "number" && Number.isFinite(v)) return Math.max(0, Math.floor(v));
    if (typeof v === "string" && v.trim() !== "" && Number.isFinite(Number(v))) {
      return Math.max(0, Math.floor(Number(v)));
    }
  }
  return 0;
}

function emptyBriefingPayload(): BriefingPayload {
  return { overview: "", themes: [], watch: [], sourceIds: [] };
}

function mapBriefingCite(raw: unknown): BriefingCite | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const articleId = pickStr(o, "articleId", "ArticleId", "article_id", "ArticleID");
  if (!articleId) return null;
  return {
    articleId,
    title: pickStr(o, "title", "Title") || articleId,
    feedTitle: pickStr(o, "feedTitle", "FeedTitle", "feed_title"),
  };
}

function mapBriefingBullet(raw: unknown): BriefingBullet | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const citesRaw = o.cites ?? o.Cites ?? [];
  const cites = Array.isArray(citesRaw)
    ? citesRaw.map(mapBriefingCite).filter((c): c is BriefingCite => !!c)
    : [];
  const articleId =
    pickStr(o, "articleId", "ArticleId", "article_id", "ArticleID") || cites[0]?.articleId || "";
  if (!articleId) return null;
  return {
    articleId,
    title: pickStr(o, "title", "Title") || cites[0]?.title || articleId,
    feedTitle: pickStr(o, "feedTitle", "FeedTitle", "feed_title") || cites[0]?.feedTitle || "",
    point: pickStr(o, "point", "Point"),
    cites,
  };
}

function mapBriefingTheme(raw: unknown): BriefingTheme | null {
  if (!raw || typeof raw !== "object") return null;
  const o = raw as Record<string, unknown>;
  const bulletsRaw = o.bullets ?? o.Bullets ?? [];
  const bullets = Array.isArray(bulletsRaw)
    ? bulletsRaw.map(mapBriefingBullet).filter((b): b is BriefingBullet => !!b)
    : [];
  return {
    title: pickStr(o, "title", "Title"),
    bullets,
  };
}

function parseBriefingPayload(raw: unknown): BriefingPayload {
  let obj: unknown = raw;
  if (typeof raw === "string") {
    const s = raw.trim();
    if (!s) return emptyBriefingPayload();
    try {
      obj = JSON.parse(s);
    } catch {
      return emptyBriefingPayload();
    }
  }
  if (!obj || typeof obj !== "object") return emptyBriefingPayload();
  const o = obj as Record<string, unknown>;
  const themesRaw = o.themes ?? o.Themes ?? [];
  const watchRaw = o.watch ?? o.Watch ?? [];
  const idsRaw = o.sourceIds ?? o.SourceIDs ?? o.SourceIds;
  return {
    overview: pickStr(o, "overview", "Overview"),
    themes: Array.isArray(themesRaw)
      ? themesRaw.map(mapBriefingTheme).filter((t): t is BriefingTheme => !!t)
      : [],
    watch: Array.isArray(watchRaw)
      ? watchRaw.map(mapBriefingBullet).filter((b): b is BriefingBullet => !!b)
      : [],
    sourceIds: Array.isArray(idsRaw)
      ? idsRaw.map((x) => String(x ?? "").trim()).filter(Boolean)
      : [],
  };
}

function normalizeBriefingStatus(raw: unknown): Briefing["status"] {
  const s = String(raw ?? "").trim().toLowerCase();
  if (s === "pending" || s === "ready" || s === "error") return s;
  return "ready";
}

/** Map a backend briefing (camelCase or PascalCase) into the frontend Briefing shape. */
export function mapBriefing(raw: any): Briefing {
  if (!raw || typeof raw !== "object") {
    return {
      id: "",
      createdAt: "",
      status: "error",
      locale: "",
      overview: "",
      articleCount: 0,
      omittedCount: 0,
      isRead: false,
      isStarred: false,
      payload: emptyBriefingPayload(),
    };
  }
  const o = raw as Record<string, unknown>;
  const payload = parseBriefingPayload(o.payload ?? o.Payload);
  const overview = pickStr(o, "overview", "Overview") || payload.overview;
  const err = pickStr(o, "error", "Error");
  const model = pickStr(o, "model", "Model");
  return {
    id: pickStr(o, "id", "ID", "Id"),
    createdAt: pickStr(o, "createdAt", "CreatedAt", "created_at") || new Date().toISOString(),
    status: normalizeBriefingStatus(o.status ?? o.Status),
    locale: pickStr(o, "locale", "Locale"),
    model: model || undefined,
    overview,
    error: err || undefined,
    articleCount: pickNum(o, "articleCount", "ArticleCount", "article_count"),
    omittedCount: pickNum(o, "omittedCount", "OmittedCount", "omitted_count"),
    isRead: !!(o.isRead ?? o.IsRead ?? o.read),
    isStarred: !!(o.isStarred ?? o.IsStarred ?? o.starred),
    payload: {
      overview: payload.overview || overview,
      themes: payload.themes,
      watch: payload.watch,
      sourceIds: payload.sourceIds,
    },
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
