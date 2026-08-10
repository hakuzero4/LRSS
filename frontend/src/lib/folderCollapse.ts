/**
 * Persist sidebar folder expand/collapse (collapsed = true means children hidden).
 * Storage key: `lrss.folderCollapsed` — JSON object of folderId → true for collapsed.
 */

export const FOLDER_COLLAPSE_STORAGE_KEY = "lrss.folderCollapsed";

/** Load collapsed map from localStorage. Missing / invalid → {}. */
export function loadCollapsedFolders(): Record<string, boolean> {
  try {
    const raw = localStorage.getItem(FOLDER_COLLAPSE_STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    const out: Record<string, boolean> = {};
    for (const [id, v] of Object.entries(parsed as Record<string, unknown>)) {
      if (typeof id === "string" && id.trim() && v === true) {
        out[id] = true;
      }
    }
    return out;
  } catch {
    return {};
  }
}

/** Persist only entries that are collapsed (true). */
export function saveCollapsedFolders(map: Record<string, boolean>): void {
  try {
    const slim: Record<string, true> = {};
    for (const [id, v] of Object.entries(map)) {
      if (v && id.trim()) slim[id] = true;
    }
    localStorage.setItem(FOLDER_COLLAPSE_STORAGE_KEY, JSON.stringify(slim));
  } catch {
    /* quota / private mode */
  }
}

/** Drop ids that no longer exist (deleted folders). */
export function pruneCollapsedFolders(
  map: Record<string, boolean>,
  validIds: Iterable<string>,
): Record<string, boolean> {
  const ok = new Set(validIds);
  const next: Record<string, boolean> = {};
  let changed = false;
  for (const [id, v] of Object.entries(map)) {
    if (!v) continue;
    if (ok.has(id)) next[id] = true;
    else changed = true;
  }
  if (!changed && Object.keys(next).length === Object.keys(map).filter((k) => map[k]).length) {
    return map;
  }
  return next;
}
