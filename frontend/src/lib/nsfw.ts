/**
 * NSFW / office-mode feed & folder visibility helpers.
 */

export type NsfwFeed = {
  id: string;
  isNsfw?: boolean;
  folderId?: string | null;
};

export type NsfwFolder = {
  id: string;
  isNsfw?: boolean;
};

/** When nsfwMode is false (office), hide isNsfw folders from the sidebar. */
export function filterFoldersForSidebar<T extends NsfwFolder>(
  folders: readonly T[],
  nsfwMode: boolean,
): T[] {
  if (nsfwMode) return folders.slice();
  return folders.filter((f) => !f.isNsfw);
}

/**
 * When nsfwMode is false (office), hide feeds that are isNsfw
 * or that live in an isNsfw folder.
 */
export function filterFeedsForSidebar<T extends NsfwFeed>(
  feeds: readonly T[],
  nsfwMode: boolean,
  nsfwFolderIds?: ReadonlySet<string> | readonly NsfwFolder[],
): T[] {
  if (nsfwMode) return feeds.slice();
  const folderNsfw = toNsfwFolderSet(nsfwFolderIds);
  return feeds.filter((f) => {
    if (f.isNsfw) return false;
    const fid = f.folderId?.trim();
    if (fid && folderNsfw.has(fid)) return false;
    return true;
  });
}

/** True when article lists / counts should exclude NSFW-feed articles. */
export function shouldExcludeNsfwArticles(nsfwMode: boolean): boolean {
  return !nsfwMode;
}

/** Feed IDs hidden in office mode (feed flag or parent folder). */
export function hiddenNsfwFeedIds<F extends NsfwFeed>(
  feeds: readonly F[],
  nsfwFolderIds?: ReadonlySet<string> | readonly NsfwFolder[],
): Set<string> {
  const folderNsfw = toNsfwFolderSet(nsfwFolderIds);
  return new Set(
    feeds
      .filter((f) => {
        if (f.isNsfw) return true;
        const fid = f.folderId?.trim();
        return !!(fid && folderNsfw.has(fid));
      })
      .map((f) => f.id),
  );
}

/** Filter articles by feed NSFW flag or parent folder NSFW (client double-check). */
export function filterArticlesByNsfwMode<
  T extends { feedId: string },
  F extends NsfwFeed,
>(
  articles: readonly T[],
  feeds: readonly F[],
  nsfwMode: boolean,
  nsfwFolderIds?: ReadonlySet<string> | readonly NsfwFolder[],
): T[] {
  if (nsfwMode) return articles.slice();
  const hiddenFeedIds = hiddenNsfwFeedIds(feeds, nsfwFolderIds);
  if (hiddenFeedIds.size === 0) return articles.slice();
  return articles.filter((a) => !hiddenFeedIds.has(a.feedId));
}

function toNsfwFolderSet(
  input?: ReadonlySet<string> | readonly NsfwFolder[],
): Set<string> {
  if (!input) return new Set();
  if (input instanceof Set) return new Set(input);
  return new Set(
    (input as readonly NsfwFolder[]).filter((f) => f.isNsfw).map((f) => f.id),
  );
}
