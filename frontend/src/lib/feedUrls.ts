/**
 * Parse one-or-many feed URLs from free text (settings multi-line add, paste).
 * Rules: one URL per line preferred; also splits on commas/semicolons.
 * Skips blanks and lines starting with #. Dedupes by href. Only http(s).
 */

export function parseFeedUrlsFromText(text: string): string[] {
  const raw = String(text ?? "");
  if (!raw.trim()) return [];

  const out: string[] = [];
  const seen = new Set<string>();

  for (const line of raw.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;

    for (const part of trimmed.split(/[\s,;]+/)) {
      const token = part.trim();
      if (!token) continue;
      let href: string;
      try {
        const u = new URL(token);
        if (u.protocol !== "http:" && u.protocol !== "https:") continue;
        href = u.href;
      } catch {
        continue;
      }
      if (seen.has(href)) continue;
      seen.add(href);
      out.push(href);
    }
  }
  return out;
}
