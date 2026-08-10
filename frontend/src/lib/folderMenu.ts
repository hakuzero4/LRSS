/**
 * Pure helpers for folder sidebar context-menu actions.
 * Kept free of Vue/Wails so they can be unit-tested and reused by the store.
 */

import type { CollectionId } from "@/types/rss";

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
