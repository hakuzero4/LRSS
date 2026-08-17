/**
 * Pure helpers for folder sidebar context-menu actions.
 * Kept free of Vue/Wails so they can be unit-tested and reused by the store.
 */

import type { CollectionId, FolderDisplayMode, SmartDisplayModes, StartupCollectionId } from "@/types/rss";

export const SMART_DISPLAY_IDS = [
  "unread",
  "today",
  "starred",
  "recent",
  "all",
  "kept",
] as const satisfies readonly StartupCollectionId[];

export function isSmartDisplayCollection(id: string): id is StartupCollectionId {
  return (SMART_DISPLAY_IDS as readonly string[]).includes(id);
}

export type FolderMenuAction =
  | "open"
  | "toggleExpand"
  | "markAllRead"
  | "refresh"
  | "addFeed"
  | "rename"
  | "delete";

/** P0 menu actions in display order (separators handled in UI). */
export const FOLDER_MENU_ACTIONS: readonly FolderMenuAction[] = [
  "open",
  "toggleExpand",
  "markAllRead",
  "refresh",
  "addFeed",
  "rename",
  "delete",
] as const;

export function folderCollectionId(folderId: string): CollectionId {
  return `folder:${folderId}`;
}

/** Feed IDs belonging to a folder (used by refresh-folder). */
export function feedIdsInFolder(
  folderId: string,
  feeds: ReadonlyArray<{ id: string; folderId?: string | null }>,
): string[] {
  const id = folderId.trim();
  if (!id) return [];
  return feeds.filter((f) => (f.folderId ?? "") === id).map((f) => f.id);
}

export function normalizeFolderDisplayMode(raw: unknown): FolderDisplayMode {
  const s = String(raw ?? "").trim().toLowerCase();
  if (s === "cards" || s === "card" || s === "gallery" || s === "grid") return "cards";
  return "list";
}

/** Layout for the current collection: folder mode, a feed's parent folder, or a smart list. */
export function resolveCollectionDisplayMode(
  collectionId: string,
  folders: ReadonlyArray<{ id: string; displayMode?: string | null }>,
  feeds: ReadonlyArray<{ id: string; folderId?: string | null }>,
  smartModes?: Partial<Record<string, string>> | null,
): FolderDisplayMode {
  const col = (collectionId ?? "").trim();
  if (col === "kept" || col.startsWith("kept:")) {
    return normalizeFolderDisplayMode(smartModes?.kept);
  }
  if (isSmartDisplayCollection(col)) {
    return normalizeFolderDisplayMode(smartModes?.[col]);
  }
  if (col.startsWith("folder:")) {
    const id = col.slice("folder:".length);
    const folder = folders.find((f) => f.id === id);
    return normalizeFolderDisplayMode(folder?.displayMode);
  }
  if (col.startsWith("feed:")) {
    const feedId = col.slice("feed:".length);
    const feed = feeds.find((f) => f.id === feedId);
    const folderId = feed?.folderId ?? "";
    if (!folderId) return "list";
    const folder = folders.find((f) => f.id === folderId);
    return normalizeFolderDisplayMode(folder?.displayMode);
  }
  return "list";
}

export function parseSmartDisplayModes(raw: unknown): SmartDisplayModes {
  if (!raw || typeof raw !== "object") return {};
  const o = raw as Record<string, unknown>;
  const out: SmartDisplayModes = {};
  for (const id of SMART_DISPLAY_IDS) {
    const pascal = id.charAt(0).toUpperCase() + id.slice(1);
    const v = o[id] ?? o[pascal];
    if (v == null || String(v).trim() === "") continue;
    out[id] = normalizeFolderDisplayMode(v);
  }
  return out;
}

export function canToggleDisplayMode(
  collectionId: string,
  feeds: ReadonlyArray<{ id: string; folderId?: string | null }>,
): boolean {
  const col = (collectionId ?? "").trim();
  if (col === "kept" || col.startsWith("kept:")) return true;
  if (isSmartDisplayCollection(col)) return true;
  return folderIdForDisplayMode(col, feeds) !== "";
}

/** Folder whose displayMode applies to this collection; empty for smart lists / unfiled feeds. */
export function folderIdForDisplayMode(
  collectionId: string,
  feeds: ReadonlyArray<{ id: string; folderId?: string | null }>,
): string {
  const col = (collectionId ?? "").trim();
  if (col.startsWith("folder:")) return col.slice("folder:".length);
  if (col.startsWith("feed:")) {
    const feedId = col.slice("feed:".length);
    return (feeds.find((f) => f.id === feedId)?.folderId ?? "").trim();
  }
  return "";
}

/** First usable image for a card: enclosure URL, then first <img> in HTML. */
export function articleCardImage(article: {
  imageUrl?: string | null;
  contentHtml?: string | null;
  summary?: string | null;
}): string {
  const direct = (article.imageUrl ?? "").trim();
  if (direct && !direct.startsWith("data:")) return direct;
  return firstHtmlImage(article.contentHtml) || firstHtmlImage(article.summary);
}

function firstHtmlImage(html: string | null | undefined): string {
  if (!html) return "";
  const m = /<img\b[^>]*\bsrc\s*=\s*["']([^"']+)["']/i.exec(html);
  const src = (m?.[1] ?? "").trim();
  if (!src || src.startsWith("data:")) return "";
  return src;
}

/** Unread sum for feeds in a folder. */
export function unreadInFolder(
  folderId: string,
  feeds: ReadonlyArray<{ folderId?: string | null; unreadCount?: number }>,
): number {
  const id = folderId.trim();
  if (!id) return 0;
  let n = 0;
  for (const f of feeds) {
    if ((f.folderId ?? "") === id) n += f.unreadCount ?? 0;
  }
  return n;
}
