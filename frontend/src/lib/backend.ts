/** Lazy load Wails-generated appsvc bindings. */

export async function loadAppsvc(): Promise<any | null> {
  try {
    const mod = await import("../../bindings/lrss/internal/appsvc/index.js");
    return mod;
  } catch {
    return null;
  }
}

export function mapArticle(a: any) {
  if (!a) return a;
  return {
    id: a.id,
    feedId: a.feedId,
    title: a.title ?? "",
    author: a.author ?? undefined,
    summary: a.summary ?? "",
    contentHtml: a.contentHtml ?? "",
    url: a.url ?? "",
    publishedAt: a.publishedAt ?? a.fetchedAt ?? new Date().toISOString(),
    read: !!(a.isRead ?? a.read),
    starred: !!(a.isStarred ?? a.starred),
    imageUrl: a.imageUrl ?? undefined,
  };
}

export function mapFeed(f: any) {
  if (!f) return f;
  return {
    id: f.id,
    title: f.title ?? "",
    siteUrl: f.siteUrl ?? f.feedUrl ?? "",
    feedUrl: f.feedUrl ?? "",
    folderId: f.folderId ?? undefined,
    unreadCount: f.unreadCount ?? 0,
    lastFetchedAt: f.lastFetchedAt ?? new Date().toISOString(),
  };
}

export function mapFolder(f: any, feeds: { id: string; folderId?: string }[]) {
  const feedIds = feeds.filter((x) => x.folderId === f.id).map((x) => x.id);
  return {
    id: f.id,
    name: f.name ?? "",
    feedIds,
  };
}
