/** Bilingual <<o>> / <<t>> pair parsing for reader interlinear translate UI. */

export type BilingualPair = {
  original: string;
  translation: string;
};

/**
 * Parse model output into source/translation pairs.
 * Primary format:
 *   <<o>> original
 *   <<t>> translation
 */
export function parseBilingualPairs(raw: string | null | undefined): BilingualPair[] {
  if (!raw) return [];
  let s = String(raw).replace(/\r\n/g, "\n").trim();
  if (!s) return [];

  const re =
    /<<\s*o\s*>>\s*([\s\S]*?)<<\s*t\s*>>\s*([\s\S]*?)(?=<<\s*o\s*>>|$)/gi;
  const out: BilingualPair[] = [];
  let m: RegExpExecArray | null;
  while ((m = re.exec(s)) !== null) {
    const original = (m[1] ?? "").trim();
    let translation = (m[2] ?? "").trim();
    const cut = translation.indexOf("<<");
    if (cut >= 0) translation = translation.slice(0, cut).trim();
    if (!original && !translation) continue;
    out.push({ original, translation });
  }
  if (out.length > 0) return out;

  // Fallback: blank-line blocks with two lines.
  const blocks = s.split(/\n\s*\n+/);
  for (const b of blocks) {
    const lines = b
      .split("\n")
      .map((l) => l.trim().replace(/^>\s*/, "").replace(/^[-*]\s+/, ""))
      .filter(Boolean);
    if (lines.length >= 2) {
      out.push({ original: lines[0]!, translation: lines[1]! });
    }
  }
  if (out.length > 0) return out;

  return [{ original: "", translation: s }];
}
