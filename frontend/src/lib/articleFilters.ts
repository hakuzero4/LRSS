/**
 * Client-side article list filters driven by Settings → Filters (UIPrefs).
 * Pure helpers — used by useRssStore.filteredArticles and unit-tested.
 */

export type FilterableArticle = {
  id: string;
  title: string;
  summary?: string | null;
  publishedAt?: string;
  feedId?: string;
};

/** Normalize title for duplicate comparison. */
export function normalizeTitle(title: string): string {
  return title
    .normalize("NFKC")
    .toLowerCase()
    .replace(/[\s\u3000]+/g, " ")
    .replace(/[“”"']/g, "")
    .trim();
}

/**
 * Parse comma / Chinese comma / semicolon separated block keywords.
 * Empty tokens discarded; case-insensitive matching is done at match time.
 */
export function parseBlockKeywords(raw: string | null | undefined): string[] {
  if (!raw || typeof raw !== "string") return [];
  return raw
    .split(/[,，;；|/]+/)
    .map((s) => s.trim().toLowerCase())
    .filter((s) => s.length > 0);
}

/** First matching keyword in title/summary, or "". */
export function firstBlockKeyword(
  article: FilterableArticle,
  keywords: readonly string[],
): string {
  if (!keywords.length) return "";
  const title = (article.title ?? "").toLowerCase();
  const summary = (article.summary ?? "").toLowerCase();
  const hay = title + "\n" + summary;
  for (const kw of keywords) {
    if (kw && hay.includes(kw)) return kw;
  }
  return "";
}

/** True if title or summary contains any keyword (case-insensitive substring). */
export function matchesBlockKeywords(
  article: FilterableArticle,
  keywords: readonly string[],
): boolean {
  return firstBlockKeyword(article, keywords) !== "";
}

export type ListHideCause = "keyword" | "duplicate" | "nsfw";

export type HiddenByFilter<T extends FilterableArticle = FilterableArticle> = {
  article: T;
  cause: ListHideCause;
  keyword?: string;
};

/**
 * Apply list hides and also return why each dropped article disappeared.
 * Order: NSFW feed → block keywords → duplicate titles (newest first).
 */
export function applyArticleView<T extends FilterableArticle>(
  articles: readonly T[],
  opts: {
    hideDuplicateTitles: boolean;
    blockKeywords: string;
    isHiddenFeed?: (article: T) => boolean;
  },
): { visible: T[]; hidden: HiddenByFilter<T>[] } {
  const hidden: HiddenByFilter<T>[] = [];
  let list = articles.slice();

  if (opts.isHiddenFeed) {
    const next: T[] = [];
    for (const a of list) {
      if (opts.isHiddenFeed(a)) hidden.push({ article: a, cause: "nsfw" });
      else next.push(a);
    }
    list = next;
  }

  const keywords = parseBlockKeywords(opts.blockKeywords);
  if (keywords.length) {
    const next: T[] = [];
    for (const a of list) {
      const keyword = firstBlockKeyword(a, keywords);
      if (keyword) hidden.push({ article: a, cause: "keyword", keyword });
      else next.push(a);
    }
    list = next;
  }

  if (opts.hideDuplicateTitles) {
    const seen = new Set<string>();
    const next: T[] = [];
    for (const a of list) {
      const key = normalizeTitle(a.title ?? "");
      if (!key) {
        next.push(a);
        continue;
      }
      if (seen.has(key)) {
        hidden.push({ article: a, cause: "duplicate" });
        continue;
      }
      seen.add(key);
      next.push(a);
    }
    list = next;
  }

  return { visible: list, hidden };
}

/**
 * Drop articles whose title/summary hit any block keyword.
 * Order preserved.
 */
export function applyBlockKeywords<T extends FilterableArticle>(
  articles: readonly T[],
  blockKeywordsRaw: string,
): T[] {
  const keywords = parseBlockKeywords(blockKeywordsRaw);
  if (!keywords.length) return articles.slice();
  return articles.filter((a) => !matchesBlockKeywords(a, keywords));
}

/**
 * Keep the first occurrence of each normalized title (caller should sort
 * newest-first so the newest duplicate is kept).
 * Empty titles never collapse with each other.
 */
export function applyHideDuplicateTitles<T extends FilterableArticle>(
  articles: readonly T[],
  enabled: boolean,
): T[] {
  if (!enabled) return articles.slice();
  const seen = new Set<string>();
  const out: T[] = [];
  for (const a of articles) {
    const key = normalizeTitle(a.title ?? "");
    if (!key) {
      out.push(a);
      continue;
    }
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(a);
  }
  return out;
}

/**
 * Full filter pipeline used by the article list.
 * Expects input already collection-scoped; does not sort.
 */
export function applyArticleFilters<T extends FilterableArticle>(
  articles: readonly T[],
  opts: { hideDuplicateTitles: boolean; blockKeywords: string },
): T[] {
  return applyArticleView(articles, opts).visible;
}
