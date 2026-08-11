/**
 * Lightweight HTML → Markdown for RSS article bodies.
 * Uses DOMParser in the browser; no external dependency.
 */

export type ArticleMarkdownInput = {
  title: string;
  author?: string | null;
  feedTitle?: string | null;
  publishedAt?: string | null;
  url?: string | null;
  summary?: string | null;
  contentHtml?: string | null;
};

/** Escape text for use inside Markdown (not code). */
export function escapeMdText(text: string): string {
  return text.replace(/([\\`*_{}[\]()#+\-.!|>])/g, "\\$1");
}

function decodeEntities(s: string): string {
  return s
    .replace(/&nbsp;/gi, " ")
    .replace(/&amp;/gi, "&")
    .replace(/&lt;/gi, "<")
    .replace(/&gt;/gi, ">")
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/gi, "'")
    .replace(/&#(\d+);/g, (_, n) => String.fromCharCode(Number(n)))
    .replace(/&#x([0-9a-f]+);/gi, (_, h) => String.fromCharCode(parseInt(h, 16)));
}

function collapseInline(s: string): string {
  return s.replace(/[ \t]+\n/g, "\n").replace(/\n[ \t]+/g, "\n").replace(/[ \t]{2,}/g, " ");
}

function trimBlankLines(s: string): string {
  return s
    .replace(/[ \t]+\n/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .replace(/^\n+/, "")
    .replace(/\n+$/, "");
}

function attr(el: Element, name: string): string {
  return (el.getAttribute(name) ?? "").trim();
}

type WalkCtx = {
  listStack: Array<{ ordered: boolean; index: number }>;
  inPre: boolean;
};

function walk(node: Node, ctx: WalkCtx): string {
  if (node.nodeType === Node.TEXT_NODE) {
    const raw = node.textContent ?? "";
    if (ctx.inPre) return raw;
    // Collapse whitespace outside pre/code.
    return raw.replace(/\s+/g, " ");
  }
  if (node.nodeType !== Node.ELEMENT_NODE) return "";

  const el = node as Element;
  const tag = el.tagName.toLowerCase();

  if (tag === "script" || tag === "style" || tag === "noscript" || tag === "svg") {
    return "";
  }

  if (tag === "br") return "\n";
  if (tag === "hr") return "\n\n---\n\n";

  if (tag === "img") {
    const src = attr(el, "src");
    if (!src) return "";
    const alt = attr(el, "alt") || "image";
    const title = attr(el, "title");
    return title
      ? `![${alt}](${src} "${title.replace(/"/g, '\\"')}")`
      : `![${alt}](${src})`;
  }

  if (tag === "a") {
    const href = attr(el, "href");
    const inner = childrenMd(el, ctx).trim();
    if (!href || href.startsWith("javascript:")) return inner;
    if (!inner) return href;
    // Avoid nested link noise for pure URL text.
    if (inner === href) return `<${href}>`;
    return `[${inner}](${href})`;
  }

  if (tag === "strong" || tag === "b") {
    const inner = childrenMd(el, ctx).trim();
    return inner ? `**${inner}**` : "";
  }
  if (tag === "em" || tag === "i") {
    const inner = childrenMd(el, ctx).trim();
    return inner ? `*${inner}*` : "";
  }
  if (tag === "s" || tag === "del" || tag === "strike") {
    const inner = childrenMd(el, ctx).trim();
    return inner ? `~~${inner}~~` : "";
  }
  if (tag === "code" && el.parentElement?.tagName.toLowerCase() !== "pre") {
    const inner = (el.textContent ?? "").replace(/\n/g, " ").trim();
    if (!inner) return "";
    const fence = inner.includes("`") ? "``" : "`";
    return `${fence}${inner}${fence}`;
  }

  if (tag === "pre") {
    const codeEl = el.querySelector("code");
    const text = (codeEl?.textContent ?? el.textContent ?? "").replace(/\n$/, "");
    const lang =
      (codeEl && Array.from(codeEl.classList).find((c) => c.startsWith("language-"))?.slice(9)) ||
      "";
    return `\n\n\`\`\`${lang}\n${text}\n\`\`\`\n\n`;
  }

  if (/^h[1-6]$/.test(tag)) {
    const level = Number(tag[1]);
    const inner = childrenMd(el, ctx).trim();
    if (!inner) return "";
    return `\n\n${"#".repeat(level)} ${inner}\n\n`;
  }

  if (tag === "blockquote") {
    const inner = childrenMd(el, ctx).trim();
    if (!inner) return "";
    const quoted = inner
      .split("\n")
      .map((line) => (line.trim() ? `> ${line}` : ">"))
      .join("\n");
    return `\n\n${quoted}\n\n`;
  }

  if (tag === "ul" || tag === "ol") {
    ctx.listStack.push({ ordered: tag === "ol", index: 0 });
    const inner = childrenMd(el, ctx);
    ctx.listStack.pop();
    return `\n\n${inner.trim()}\n\n`;
  }

  if (tag === "li") {
    const depth = ctx.listStack.length;
    const frame = ctx.listStack[depth - 1];
    const indent = "  ".repeat(Math.max(0, depth - 1));
    let bullet = "- ";
    if (frame?.ordered) {
      frame.index += 1;
      bullet = `${frame.index}. `;
    }
    // li content may include nested lists; first line gets the bullet.
    const inner = childrenMd(el, ctx).trim();
    if (!inner) return `${indent}${bullet}\n`;
    const lines = inner.split("\n");
    const first = `${indent}${bullet}${lines[0]}`;
    const rest = lines
      .slice(1)
      .map((l) => (l.trim() ? `${indent}  ${l}` : ""))
      .join("\n");
    return rest ? `${first}\n${rest}\n` : `${first}\n`;
  }

  if (tag === "figure") {
    return `\n\n${childrenMd(el, ctx).trim()}\n\n`;
  }
  if (tag === "figcaption") {
    const inner = childrenMd(el, ctx).trim();
    return inner ? `\n*${inner}*\n` : "";
  }

  if (tag === "table") {
    return tableToMd(el) + "\n\n";
  }

  if (tag === "p" || tag === "div" || tag === "section" || tag === "article") {
    const inner = childrenMd(el, ctx).trim();
    if (!inner) return "";
    // Nested divs: keep spacing light.
    return `\n\n${inner}\n\n`;
  }

  // Default: unwrap.
  return childrenMd(el, ctx);
}

function childrenMd(el: Element, ctx: WalkCtx): string {
  let out = "";
  for (const child of Array.from(el.childNodes)) {
    out += walk(child, ctx);
  }
  return out;
}

function tableToMd(table: Element): string {
  const rows = Array.from(table.querySelectorAll("tr"));
  if (rows.length === 0) return "";

  const cellsOf = (tr: Element): string[] =>
    Array.from(tr.querySelectorAll("th,td")).map((c) =>
      (c.textContent ?? "").replace(/\s+/g, " ").trim().replace(/\|/g, "\\|"),
    );

  const matrix = rows.map(cellsOf).filter((r) => r.length > 0);
  if (matrix.length === 0) return "";

  const cols = Math.max(...matrix.map((r) => r.length));
  const pad = (row: string[]) => {
    const r = row.slice();
    while (r.length < cols) r.push("");
    return r;
  };

  const header = pad(matrix[0]!);
  const sep = header.map(() => "---");
  const body = matrix.slice(1).map(pad);

  const line = (cells: string[]) => `| ${cells.join(" | ")} |`;
  return ["", line(header), line(sep), ...body.map(line), ""].join("\n");
}

/**
 * Convert an HTML fragment to Markdown.
 * Safe on empty / plain-text input.
 */
export function htmlToMarkdown(html: string | null | undefined): string {
  if (!html) return "";
  const raw = String(html).trim();
  if (!raw) return "";

  // Plain text (no tags): return as-is with light normalize.
  if (!/<[a-z][\s\S]*>/i.test(raw)) {
    return trimBlankLines(decodeEntities(raw).replace(/\r\n/g, "\n"));
  }

  if (typeof DOMParser === "undefined") {
    // Non-browser fallback: strip tags crudely.
    return trimBlankLines(
      decodeEntities(
        raw
          .replace(/<br\s*\/?>/gi, "\n")
          .replace(/<\/(p|div|h[1-6]|li|tr)>/gi, "\n\n")
          .replace(/<[^>]+>/g, "")
          .replace(/\r\n/g, "\n"),
      ),
    );
  }

  const doc = new DOMParser().parseFromString(
    `<div id="__md_root">${raw}</div>`,
    "text/html",
  );
  const root = doc.getElementById("__md_root");
  if (!root) return "";

  const md = walk(root, { listStack: [], inPre: false });
  return trimBlankLines(collapseInline(md));
}

/**
 * Build a full Markdown document for an RSS article
 * (title, meta, optional summary, body).
 */
export function articleToMarkdown(article: ArticleMarkdownInput): string {
  const title = (article.title ?? "").trim() || "Untitled";
  const parts: string[] = [`# ${title}`, ""];

  const meta: string[] = [];
  const author = (article.author ?? "").trim();
  const feed = (article.feedTitle ?? "").trim();
  if (author && feed) meta.push(`${author} · ${feed}`);
  else if (author) meta.push(author);
  else if (feed) meta.push(feed);

  if (article.publishedAt) {
    const d = new Date(article.publishedAt);
    if (!Number.isNaN(d.getTime())) {
      meta.push(d.toISOString().slice(0, 10));
    }
  }

  if (meta.length) {
    parts.push(meta.join(" · "), "");
  }

  const url = (article.url ?? "").trim();
  if (url) {
    parts.push(`[原文](${url})`, "");
  }

  const summary = (article.summary ?? "").trim();
  if (summary) {
    // Summary may already be plain text from store normalization.
    const sumMd = /<[a-z][\s\S]*>/i.test(summary)
      ? htmlToMarkdown(summary)
      : summary;
    if (sumMd) {
      parts.push("> " + sumMd.replace(/\n+/g, "\n> "), "");
    }
  }

  const body = htmlToMarkdown(article.contentHtml);
  if (body) {
    if (summary) parts.push("---", "");
    parts.push(body, "");
  }

  return parts.join("\n").replace(/\n{3,}/g, "\n\n").trim() + "\n";
}

/**
 * Copy text to the system clipboard. Returns true on success.
 */
export async function copyTextToClipboard(text: string): Promise<boolean> {
  if (!text) return false;
  try {
    if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // fall through
  }
  try {
    if (typeof document === "undefined") return false;
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.left = "-9999px";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}
