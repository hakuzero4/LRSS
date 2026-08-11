/**
 * Pure helpers for Settings → Reading (and related General scroll-end).
 * Consumed by ArticleReader / filteredArticles; unit-tested without Vue/Wails.
 */

export type FontSizePref = "sm" | "md" | "lg";
export type ReaderWidthPref = "narrow" | "medium" | "wide" | "fill";

/** Map prefs → CSS class tokens applied on `.reader-shell`. */
export function readerShellClasses(
  fontSize: string | null | undefined,
  readerWidth: string | null | undefined,
): { font: string; width: string; className: string } {
  const font =
    fontSize === "sm"
      ? "reader-font-sm"
      : fontSize === "lg"
        ? "reader-font-lg"
        : "reader-font-md";
  const width =
    readerWidth === "narrow"
      ? "reader-width-narrow"
      : readerWidth === "wide"
        ? "reader-width-wide"
        : readerWidth === "fill"
          ? "reader-width-fill"
          : "reader-width-medium";
  return { font, width, className: `${font} ${width}` };
}

/**
 * Hide read articles when showUnreadOnly is on.
 * Starred collection is exempt (product rule).
 */
export function applyShowUnreadOnly<T extends { read?: boolean }>(
  articles: readonly T[],
  showUnreadOnly: boolean,
  collectionId: string,
): T[] {
  if (!showUnreadOnly) return articles.slice();
  if (collectionId === "starred") return articles.slice();
  return articles.filter((a) => !a.read);
}

export type ScrollMarkReadInput = {
  enabled: boolean;
  articleId: string | null | undefined;
  alreadyRead: boolean;
  /** article id already marked via scroll-end this visit */
  alreadyMarkedId: string | null | undefined;
  scrollHeight: number;
  scrollTop: number;
  clientHeight: number;
  /** px remaining considered "at bottom" */
  thresholdPx?: number;
};

/**
 * One-shot scroll-end mark-read decision.
 * Returns true only when we should mark this article read now.
 */
export function shouldMarkReadOnScrollEnd(input: ScrollMarkReadInput): boolean {
  if (!input.enabled) return false;
  if (!input.articleId) return false;
  if (input.alreadyRead) return false;
  if (input.alreadyMarkedId === input.articleId) return false;
  const threshold = input.thresholdPx ?? 48;
  const remaining =
    input.scrollHeight - input.scrollTop - input.clientHeight;
  return remaining <= threshold;
}

/** Normalize href for open-link: empty / javascript: rejected. */
export function normalizeOpenableUrl(url: string | null | undefined): string | null {
  const href = String(url ?? "").trim();
  if (!href) return null;
  if (/^\s*javascript:/i.test(href)) return null;
  return href;
}
