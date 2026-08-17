/**
 * Pure helpers for Settings → Filters "what's actually on" + keep log.
 * Run tests: npx tsx src/lib/filterStatus.selftest.ts
 */

export type SmartKeepState = "on" | "off" | "need-model";

export type KeepDecision = {
  articleId: string;
  title: string;
  outcome: "kept" | "skipped";
  gate: string;
  reason: string;
  confidence: number;
  folder: string;
  at: string;
};

export function smartKeepState(enabled: boolean, llmConfigured: boolean): SmartKeepState {
  if (!enabled) return "off";
  if (!llmConfigured) return "need-model";
  return "on";
}

/** Same bars as internal/llm.KeepConfidenceThreshold. */
export function keepConfidenceThreshold(strictness: string): number {
  switch (String(strictness ?? "").trim().toLowerCase()) {
    case "loose":
    case "宽松":
      return 0.55;
    case "strict":
    case "严格":
      return 0.85;
    default:
      return 0.7;
  }
}

export function profileIsEmpty(profile: string | null | undefined): boolean {
  return !String(profile ?? "").trim();
}

export function profilePreview(profile: string | null | undefined, max = 56): string {
  const raw = String(profile ?? "").replace(/\s+/g, " ").trim();
  if (!raw) return "";
  if (raw.length <= max) return raw;
  return `${raw.slice(0, Math.max(1, max - 1)).trimEnd()}…`;
}

export function formatKeepConfidence(n: number | null | undefined): string {
  if (typeof n !== "number" || !Number.isFinite(n) || n <= 0) return "";
  const pct = Math.round(n * 100);
  return String(Math.max(0, Math.min(100, pct)));
}

export function parseKeepLog(raw: unknown): KeepDecision[] {
  const list = Array.isArray(raw) ? raw : [];
  const out: KeepDecision[] = [];
  const seen = new Set<string>();
  for (const item of list) {
    if (!item || typeof item !== "object") continue;
    const o = item as Record<string, unknown>;
    const articleId = String(o.articleId ?? o.ArticleID ?? o.ArticleId ?? "").trim();
    if (!articleId || seen.has(articleId)) continue;
    seen.add(articleId);
    const outcomeRaw = String(o.outcome ?? o.Outcome ?? "").trim().toLowerCase();
    const outcome: KeepDecision["outcome"] = outcomeRaw === "kept" ? "kept" : "skipped";
    const confidenceRaw = o.confidence ?? o.Confidence;
    const confidence =
      typeof confidenceRaw === "number" && Number.isFinite(confidenceRaw) ? confidenceRaw : 0;
    out.push({
      articleId,
      title: String(o.title ?? o.Title ?? "").trim(),
      outcome,
      gate: String(o.gate ?? o.Gate ?? "").trim(),
      reason: String(o.reason ?? o.Reason ?? "").trim(),
      confidence,
      folder: String(o.folder ?? o.Folder ?? "").trim(),
      at: String(o.at ?? o.At ?? "").trim(),
    });
  }
  return out;
}
