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
 * Build a safe CSS `font-family` value for the article reader.
 * Empty / system / default → "" (use app stack via CSS inherit).
 * Rejects characters that could break style injection.
 */
export function readerFontFamilyCSS(
  family: string | null | undefined,
): string {
  const raw = String(family ?? "").trim();
  if (!raw) return "";
  if (/^(system|default|system-ui)$/i.test(raw)) {
    // system-ui is a valid CSS generic — allow as a real stack starter.
    if (raw.toLowerCase() === "system-ui") {
      return `system-ui, ui-sans-serif, sans-serif`;
    }
    return "";
  }
  if (/[/\\<>|{};\n\r]/.test(raw) || raw.length > 80) return "";
  // Escape double quotes in family name for CSS string.
  const safe = raw.replace(/\\/g, "").replace(/"/g, "");
  if (!safe) return "";
  return `"${safe}", ui-sans-serif, system-ui, sans-serif`;
}

/**
 * Hide read articles when showUnreadOnly is on.
 * Starred and recently-read collections are exempt (product rule).
 */
export function applyShowUnreadOnly<T extends { read?: boolean }>(
  articles: readonly T[],
  showUnreadOnly: boolean,
  collectionId: string,
): T[] {
  if (!showUnreadOnly) return articles.slice();
  if (collectionId === "starred" || collectionId === "recent") return articles.slice();
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
