/**
 * Pure helpers for UI honesty / wiring gaps (settings + shell).
 * Unit-tested without Vue shell.
 */

export type FeedMenuAction =
  | "open"
  | "refresh"
  | "markAllRead"
  | "rename"
  | "pause"
  | "unpause"
  | "moveUnfiled"
  | "moveToFolder"
  | "delete";

/** P0 feed context-menu actions (moveToFolder is dynamic when folders exist). */
export const FEED_MENU_ACTIONS: readonly FeedMenuAction[] = [
  "open",
  "refresh",
  "markAllRead",
  "rename",
  "pause",
  "unpause",
  "moveUnfiled",
  "delete",
] as const;

/**
 * Resolve folder for a new subscription.
 * Explicit target (context menu / picker) wins; else default folder setting.
 */
export function resolveAddFeedFolderId(
  explicitFolderId: string | null | undefined,
  defaultFolderId: string | null | undefined,
): string {
  const ex = (explicitFolderId ?? "").trim();
  if (ex) return ex;
  return (defaultFolderId ?? "").trim();
}

/** Use backend SearchService when connected and query is non-empty. */
export function shouldUseBackendSearch(
  backendReady: boolean,
  query: string | null | undefined,
): boolean {
  return !!backendReady && (query ?? "").trim().length > 0;
}

/** Sidebar density class from prefs. */
export function compactSidebarClass(compact: boolean): string {
  return compact ? "sidebar-compact" : "";
}

/** Empty-list reason for honest empty states. */
export type EmptyListReason = "no-feeds" | "no-matches" | "empty-collection";

/**
 * Classify why the article list is empty.
 * - no-feeds: library has zero subscriptions
 * - no-matches: user search/filters removed everything (not default hideDuplicateTitles)
 * - empty-collection: collection simply has no items
 * articleCountInView > 0 is a no-op safety (caller usually only asks when empty).
 */
export function resolveEmptyListReason(opts: {
  feedCount: number;
  articleCountInView: number;
  hasSearchQuery: boolean;
  /** User-applied filters that can zero the list (NOT default-on options like hideDuplicateTitles). */
  hasActiveFilters: boolean;
}): EmptyListReason {
  if (opts.articleCountInView > 0) return "empty-collection";
  if (opts.feedCount === 0) return "no-feeds";
  if (opts.hasSearchQuery || opts.hasActiveFilters) return "no-matches";
  return "empty-collection";
}

/**
 * Resolve the selected article from one or more pools (collection page + search hits).
 * Backend search results often are not in the current collection `articles` page.
 */
export function resolveSelectedArticle<T extends { id: string }>(
  id: string | null | undefined,
  ...pools: Array<readonly T[] | null | undefined>
): T | null {
  if (!id) return null;
  for (const pool of pools) {
    if (!pool || pool.length === 0) continue;
    const hit = pool.find((a) => a.id === id);
    if (hit) return hit;
  }
  return null;
}

/**
 * Merge an updated article into every pool that already contains it.
 * If only present in search hits (not collection page), still updates the hit row.
 * Returns true if at least one pool was updated.
 */
export function mergeArticleIntoPools<T extends { id: string }>(
  article: T,
  ...pools: Array<T[] | null | undefined>
): boolean {
  let updated = false;
  for (const pool of pools) {
    if (!pool) continue;
    const idx = pool.findIndex((a) => a.id === article.id);
    if (idx >= 0) {
      pool[idx] = { ...pool[idx], ...article };
      updated = true;
    }
  }
  return updated;
}
