/**
 * HTTP appsvc adapter for browser web-access mode.
 * Method names match Wails bindings so useRssStore can call the same paths.
 */

import { getWebToken, isWailsHost, setWebAuthState } from "./webMode";

/** Thrown when /api/* returns a non-2xx status (status preserved). */
export class HttpApiError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "HttpApiError";
    this.status = status;
  }
}

function isUnauthorizedStatus(status: number, msg: string): boolean {
  if (status === 401 || status === 403) return true;
  const m = msg.toLowerCase();
  return m === "unauthorized" || m.includes("unauthorized");
}

async function apiFetch(
  path: string,
  init?: RequestInit,
): Promise<Response> {
  const headers = new Headers(init?.headers);
  if (!headers.has("Content-Type") && init?.body) {
    headers.set("Content-Type", "application/json");
  }
  const token = getWebToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  const res = await fetch(path, { ...init, headers });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const j = (await res.json()) as { error?: string };
      if (j?.error) msg = j.error;
    } catch {
      /* ignore */
    }
    const errMsg = msg || `HTTP ${res.status}`;
    if (isUnauthorizedStatus(res.status, errMsg)) {
      // Token missing/wrong/rotated — flip gate so App shows only the auth page.
      setWebAuthState("unauthorized");
    }
    throw new HttpApiError(res.status, errMsg);
  }
  return res;
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await apiFetch(path);
  return (await res.json()) as T;
}

async function postJSON<T>(path: string, body: unknown): Promise<T> {
  const res = await apiFetch(path, {
    method: "POST",
    body: JSON.stringify(body ?? {}),
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  if (!text) return undefined as T;
  return JSON.parse(text) as T;
}

/** Probe /api/meta; returns adapter or null. Sets webAuthState on outcome. */
export async function tryHttpAppsvc(): Promise<Record<string, unknown> | null> {
  try {
    if (typeof window !== "undefined" && isWailsHost(window.location.hostname)) {
      return null;
    }
    const res = await apiFetch("/api/meta");
    if (!res.ok) return null;
    const ct = (res.headers.get("content-type") || "").toLowerCase();
    if (!ct.includes("json")) return null;
    const meta = (await res.json()) as { mode?: string; web?: boolean };
    // Vite/Wails origin answers { mode: "wails", web: false }; only the
    // web-access server advertises mode=web.
    if (meta?.mode !== "web" && !meta?.web) return null;
  } catch (e) {
    if (e instanceof HttpApiError && isUnauthorizedStatus(e.status, e.message)) {
      setWebAuthState("unauthorized");
    }
    return null;
  }

  setWebAuthState("ok");
  return {
    __lrssTransport: "http",
    FeedService: {
      ListFolders: () => getJSON("/api/folders"),
      ListFeeds: () => getJSON("/api/feeds"),
    },
    ArticleService: {
      List: (collection: string, limit: number, offset: number) => {
        const q = new URLSearchParams({
          collection: collection || "unread",
          limit: String(limit ?? 50),
          offset: String(offset ?? 0),
        });
        return getJSON(`/api/articles?${q}`);
      },
      Get: (id: string) => getJSON(`/api/articles/${encodeURIComponent(id)}`),
      SmartCounts: () => getJSON("/api/smart-counts"),
      SetRead: (id: string, read: boolean) =>
        postJSON(`/api/articles/${encodeURIComponent(id)}/read`, { read }),
      SetStarred: (id: string, starred: boolean) =>
        postJSON(`/api/articles/${encodeURIComponent(id)}/star`, { starred }),
      RecordOpened: (id: string) =>
        postJSON(`/api/articles/${encodeURIComponent(id)}/opened`, {}),
      MarkAllRead: (collection: string) =>
        postJSON("/api/articles/mark-all-read", { collection }),
      FetchFullContent: (id: string) =>
        postJSON(`/api/articles/${encodeURIComponent(id)}/fetch-full`, {}),
    },
    SettingsService: {
      GetUIPrefs: () => getJSON("/api/ui-prefs"),
      GetLibraryConfig: () => getJSON("/api/library-config"),
      // Mutations intentionally absent (management read-only)
    },
    BriefingService: {
      List: () => getJSON("/api/briefings"),
      Get: (id: string) => getJSON(`/api/briefings/${encodeURIComponent(id)}`),
      SetRead: (id: string, read: boolean) =>
        postJSON(`/api/briefings/${encodeURIComponent(id)}/read`, { read }),
      SetStarred: (id: string, starred: boolean) =>
        postJSON(`/api/briefings/${encodeURIComponent(id)}/star`, { starred }),
      Retry: (id: string) =>
        postJSON(`/api/briefings/${encodeURIComponent(id)}/retry`, {}),
      Delete: (id: string) =>
        postJSON(`/api/briefings/${encodeURIComponent(id)}/delete`, {}),
    },
    SearchService: {
      Search: (query: string, mode: string, limit: number) => {
        const q = new URLSearchParams({
          q: query ?? "",
          mode: mode ?? "",
          limit: String(limit ?? 20),
        });
        return getJSON(`/api/search?${q}`);
      },
    },
    // Reader toolbar AI tools (same method names as Wails AIService)
    AIService: {
      IsLLMConfigured: async () => {
        const r = await getJSON<{ configured?: boolean }>("/api/ai/configured");
        return !!(r?.configured);
      },
      Summarize: (articleId: string, locale: string) =>
        postJSON("/api/ai/summarize", { articleId, locale }),
      Translate: (articleId: string, targetLang: string) =>
        postJSON("/api/ai/translate", { articleId, targetLang }),
      TranslateSelection: (text: string, targetLang: string) =>
        postJSON("/api/ai/translate-selection", { text, targetLang }),
      Ask: (articleId: string, question: string, locale: string) =>
        postJSON("/api/ai/ask", { articleId, question, locale }),
      SuggestFolders: (articleId: string, locale: string) =>
        postJSON("/api/ai/suggest-folders", { articleId, locale }),
      ClassifyPromo: (articleId: string, locale: string) =>
        postJSON("/api/ai/classify", { articleId, locale }),
      DetectContentFullness: (articleId: string) =>
        postJSON("/api/ai/detect-fullness", { articleId }),
      EnsureFullContent: (articleId: string) =>
        postJSON("/api/ai/ensure-full", { articleId }),
      ClearTranslation: (articleId: string) =>
        postJSON("/api/ai/clear-translation", { articleId }),
      ApplySuggestedFolder: (articleId: string, folderId: string) =>
        postJSON("/api/ai/apply-folder", { articleId, folderId }),
    },
  };
}
