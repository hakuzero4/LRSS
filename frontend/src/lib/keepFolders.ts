/**
 * Pure helpers for the 精选 keep-folder tree (sidebar + reader + filters).
 */

import type { CollectionId, KeepFolder } from "@/types/rss";

/** Collapse-map key for the virtual 精选 root (not a real folder id). */
export const KEPT_ROOT_COLLAPSE_ID = "__kept__";

export function keepCollectionId(folderId: string): CollectionId {
  return `kept:${folderId}`;
}

/** Folder id from `kept:ID`, or null for the root `kept` collection / other ids. */
export function parseKeepFolderId(collectionId: string): string | null {
  if (!collectionId.startsWith("kept:")) return null;
  const id = collectionId.slice("kept:".length).trim();
  return id || null;
}

export function isKeptCollection(id: string): boolean {
  return id === "kept" || id.startsWith("kept:");
}

export function sortKeepFolders(folders: readonly KeepFolder[]): KeepFolder[] {
  return folders.slice().sort((a, b) => {
    const so = (a.sortOrder ?? 0) - (b.sortOrder ?? 0);
    if (so !== 0) return so;
    return a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
  });
}

export function firstLevelKeepFolders(folders: readonly KeepFolder[]): KeepFolder[] {
  return sortKeepFolders(folders.filter((f) => !f.parentId));
}

export function childKeepFolders(
  folders: readonly KeepFolder[],
  parentId: string,
): KeepFolder[] {
  const id = parentId.trim();
  if (!id) return [];
  return sortKeepFolders(folders.filter((f) => (f.parentId ?? "") === id));
}

/** First-level parent only; second-level or unknown ids become root. */
export function normalizeKeepParentId(
  parentId: string | undefined | null,
  folders: readonly KeepFolder[],
): string | undefined {
  const id = String(parentId ?? "").trim();
  if (!id) return undefined;
  const parent = folders.find((f) => f.id === id);
  if (!parent || parent.parentId) return undefined;
  return id;
}

/** Folder + descendants (parent first). */
export function keepFolderDescendantIds(
  folders: readonly KeepFolder[],
  id: string,
): string[] {
  const root = id.trim();
  if (!root) return [];
  const out: string[] = [root];
  for (const f of folders) {
    if ((f.parentId ?? "") === root) {
      out.push(...keepFolderDescendantIds(folders, f.id));
    }
  }
  return out;
}

export type KeepFolderOption = {
  id: string;
  name: string;
  depth: number;
};

/** Depth-first options for reader / settings lists. */
export function keepFolderOptions(folders: readonly KeepFolder[]): KeepFolderOption[] {
  const out: KeepFolderOption[] = [];
  for (const root of firstLevelKeepFolders(folders)) {
    out.push({ id: root.id, name: root.name, depth: 0 });
    for (const child of childKeepFolders(folders, root.id)) {
      out.push({ id: child.id, name: child.name, depth: 1 });
    }
  }
  return out;
}
