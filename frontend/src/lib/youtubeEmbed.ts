/**
 * YouTube helpers for reader fallback when RSS body is empty
 * (YouTube Atom stores description under media:group only).
 */

const YT_ID_RE =
  /(?:youtube(?:-nocookie)?\.com\/(?:watch\?(?:[^#]*&)?v=|embed\/|shorts\/|v\/)|youtu\.be\/)([A-Za-z0-9_-]{6,20})/i;

/** Extract video id from a watch / embed / shorts / youtu.be URL. */
export function youtubeVideoIdFromURL(raw: string | null | undefined): string | null {
  const s = (raw ?? "").trim();
  if (!s) return null;
  const m = s.match(YT_ID_RE);
  if (m?.[1] && isYouTubeId(m[1])) return m[1];
  try {
    const u = new URL(s);
    if (/youtube\.com$/i.test(u.hostname) || /\.youtube\.com$/i.test(u.hostname)) {
      const v = u.searchParams.get("v")?.trim() ?? "";
      if (isYouTubeId(v)) return v;
    }
  } catch {
    /* ignore */
  }
  return null;
}

function isYouTubeId(id: string): boolean {
  return /^[A-Za-z0-9_-]{6,20}$/.test(id);
}

/** Privacy-friendly embed + escaped description (plain text). */
export function buildYouTubeEmbedHTML(
  videoId: string,
  description?: string | null,
): string {
  if (!isYouTubeId(videoId)) return "";
  const id = escapeAttr(videoId);
  let html =
    `<div class="yt-embed">` +
    `<iframe src="https://www.youtube-nocookie.com/embed/${id}" ` +
    `title="YouTube video" loading="lazy" allowfullscreen ` +
    `allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" ` +
    `referrerpolicy="strict-origin-when-cross-origin"></iframe>` +
    `</div>`;
  const desc = (description ?? "").trim();
  if (desc) {
    const parts = desc.split("\n");
    let body = "<p>";
    for (let i = 0; i < parts.length; i++) {
      const line = parts[i] ?? "";
      if (i > 0) {
        if (!line.trim()) {
          body += "</p><p>";
          continue;
        }
        body += "<br>";
      }
      body += escapeHtml(line);
    }
    body += "</p>";
    html += `<div class="yt-desc">${body}</div>`;
  }
  return html;
}

/**
 * Prefer stored contentHtml; if empty and URL is YouTube, synthesize embed
 * so already-imported channel items render without a re-fetch.
 */
export function articleDisplayHTML(opts: {
  contentHtml?: string | null;
  url?: string | null;
  summary?: string | null;
}): string {
  const stored = (opts.contentHtml ?? "").trim();
  if (stored) return stored;
  const id = youtubeVideoIdFromURL(opts.url);
  if (!id) return "";
  return buildYouTubeEmbedHTML(id, opts.summary);
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function escapeAttr(s: string): string {
  return escapeHtml(s).replace(/'/g, "&#39;");
}
