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

/** True if title or summary contains any keyword (case-insensitive substring). */
export function matchesBlockKeywords(
  article: FilterableArticle,
  keywords: readonly string[],
): boolean {
  if (!keywords.length) return false;
  const title = (article.title ?? "").toLowerCase();
  const summary = (article.summary ?? "").toLowerCase();
  const hay = title + "\n" + summary;
  for (const kw of keywords) {
    if (kw && hay.includes(kw)) return true;
  }
  return false;
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
  let list = applyBlockKeywords(articles, opts.blockKeywords);
  list = applyHideDuplicateTitles(list, opts.hideDuplicateTitles);
  return list;
}
