const relativeTimeFormatters = new Map<string, Intl.RelativeTimeFormat>();

function relativeTimeFormatter(locale?: string): Intl.RelativeTimeFormat {
  const key = locale || "default";
  let fmt = relativeTimeFormatters.get(key);
  if (!fmt) {
    fmt = new Intl.RelativeTimeFormat(locale || undefined, { numeric: "auto" });
    relativeTimeFormatters.set(key, fmt);
  }
  return fmt;
}

export function relativeTime(iso: string, now = Date.now()): string {
  const then = new Date(iso).getTime();
  const diffSec = Math.round((then - now) / 1000);
  const abs = Math.abs(diffSec);
  const locale =
    typeof document !== "undefined" ? document.documentElement.lang || undefined : undefined;
  const rtf = relativeTimeFormatter(locale);

  if (abs < 60) return rtf.format(diffSec, "second");
  const diffMin = Math.round(diffSec / 60);
  if (Math.abs(diffMin) < 60) return rtf.format(diffMin, "minute");
  const diffHour = Math.round(diffMin / 60);
  if (Math.abs(diffHour) < 24) return rtf.format(diffHour, "hour");
  const diffDay = Math.round(diffHour / 24);
  if (Math.abs(diffDay) < 30) return rtf.format(diffDay, "day");
  const diffMonth = Math.round(diffDay / 30);
  if (Math.abs(diffMonth) < 12) return rtf.format(diffMonth, "month");
  return rtf.format(Math.round(diffMonth / 12), "year");
}

export function formatAbsolute(iso: string): string {
  return new Intl.DateTimeFormat(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(iso));
}

export function feedInitials(title: string): string {
  const parts = title.trim().split(/\s+/).slice(0, 2);
  return parts.map((p) => p[0]?.toUpperCase() ?? "").join("") || "F";
}

/**
 * Normalize RSS author for display. Google Blog etc. store person markup like
 * `<name>…</name><title>…</title>` which must not render as raw tags.
 */
export function formatAuthor(raw: string | null | undefined): string {
  if (!raw) return "";
  const s = String(raw).trim();
  if (!s) return "";
  if (!/[<>]/.test(s)) return s;
  const pick = (tag: string): string => {
    const re = new RegExp(`<${tag}(?:\\s[^>]*)?>([\\s\\S]*?)</${tag}>`, "i");
    const m = s.match(re);
    return m ? plainText(m[1]).trim() : "";
  };
  const parts = ["name", "title", "department", "company", "email"]
    .map(pick)
    .filter(Boolean);
  if (parts.length > 0) return parts.join(" · ");
  return plainText(s);
}

/** Strip tags / decode entities for safe plain-text display (list/reader summary). */
export function plainText(htmlOrText: string | null | undefined, maxLen = 0): string {
  if (!htmlOrText) return "";
  let s = String(htmlOrText);
  // If it looks like HTML, strip tags first.
  if (/[<>]/.test(s)) {
    s = s
      .replace(/<script[\s\S]*?<\/script>/gi, " ")
      .replace(/<style[\s\S]*?<\/style>/gi, " ")
      .replace(/<br\s*\/?>/gi, " ")
      .replace(/<\/(p|div|li|h[1-6]|tr)>/gi, " ")
      .replace(/<[^>]+>/g, " ");
  }
  s = s
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/gi, "'")
    .replace(/&#(\d+);/g, (_, n) => String.fromCharCode(Number(n)))
    .replace(/\s+/g, " ")
    .trim();
  if (maxLen > 0 && s.length > maxLen) {
    return s.slice(0, maxLen).replace(/\s+\S*$/, "") + "…";
  }
  return s;
}
