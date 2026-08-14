/**
 * Small, escaped Markdown renderer for assistant replies.
 * Handles paragraphs, emphasis, inline code, lists, and [n] citations.
 */

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function inlineFormat(s: string): string {
  let out = escapeHtml(s);
  out = out.replace(/`([^`]+)`/g, '<code class="chat-md-code">$1</code>');
  out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
  out = out.replace(/(^|[^\w*])\*([^*\n]+)\*(?!\*)/g, "$1<em>$2</em>");
  out = out.replace(
    /\[(\d+)\]/g,
    '<button type="button" class="chat-cite" data-cite="$1">[$1]</button>',
  );
  return out;
}

/** Convert assistant Markdown to safe HTML. */
export function renderChatMarkdown(raw: string): string {
  const text = String(raw ?? "").replace(/\r\n/g, "\n").trim();
  if (!text) return "";

  const lines = text.split("\n");
  const parts: string[] = [];
  let list: string[] = [];
  let para: string[] = [];

  const flushList = () => {
    if (!list.length) return;
    parts.push(`<ul class="chat-md-list">${list.join("")}</ul>`);
    list = [];
  };
  const flushPara = () => {
    if (!para.length) return;
    parts.push(`<p class="chat-md-p">${inlineFormat(para.join(" "))}</p>`);
    para = [];
  };

  for (const line of lines) {
    const bullet = line.match(/^\s*(?:[-*•]|\d+\.)\s+(.+)$/);
    if (bullet) {
      flushPara();
      list.push(`<li>${inlineFormat(bullet[1])}</li>`);
      continue;
    }
    if (line.trim() === "") {
      flushList();
      flushPara();
      continue;
    }
    flushList();
    para.push(line.trim());
  }
  flushList();
  flushPara();
  return parts.join("");
}

/** Turn [n] markers into markdown links the official renderer can click. */
export function linkifyCiteMarkers(md: string): string {
  return String(md ?? "").replace(/\[(\d+)\]/g, (_m, n: string) => `[[${n}]](#lrss-cite-${n})`);
}

export function citeNFromHref(href: string): number | null {
  const raw = String(href ?? "").trim();
  const hash = raw.includes("#") ? String(raw.split("#").pop() ?? "") : raw.replace(/^#/, "");
  const m = hash.match(/^\/?lrss-cite-(\d+)$/);
  if (!m) return null;
  const n = Number(m[1]);
  return Number.isFinite(n) && n >= 1 ? n : null;
}

export function extractCiteNs(raw: string): number[] {
  const seen = new Set<number>();
  const out: number[] = [];
  const re = /\[(\d+)\]/g;
  let m: RegExpExecArray | null;
  const s = String(raw ?? "");
  while ((m = re.exec(s))) {
    const n = Number(m[1]);
    if (!Number.isFinite(n) || n < 1 || seen.has(n)) continue;
    seen.add(n);
    out.push(n);
  }
  return out;
}
