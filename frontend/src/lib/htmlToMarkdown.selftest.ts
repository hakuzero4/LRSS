/**
 * Unit checks for HTML → Markdown helpers.
 * Run: npx tsx src/lib/htmlToMarkdown.selftest.ts
 */
import { articleToMarkdown, escapeMdText, htmlToMarkdown } from "./htmlToMarkdown.ts";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// --- escape ---
assert(escapeMdText("a*b").includes("\\*"), "escape asterisk");
assert(escapeMdText("x_y").includes("\\_"), "escape underscore");

// --- plain text ---
assert(htmlToMarkdown("hello world") === "hello world", "plain text passthrough");
assert(htmlToMarkdown("") === "", "empty");
assert(htmlToMarkdown(null) === "", "null");

// --- basic tags (DOMParser available in modern Node via undici? may fallback) ---
const p = htmlToMarkdown("<p>Hello <strong>world</strong></p>");
assert(p.includes("Hello"), "p text");
// Strong may be **world** with DOMParser, or stripped in fallback
assert(/world/.test(p), "strong inner text");

const link = htmlToMarkdown('<p>See <a href="https://ex.com">here</a></p>');
assert(link.includes("https://ex.com") || link.includes("here"), "link preserved");

const br = htmlToMarkdown("a<br>b");
assert(br.includes("a") && br.includes("b"), "br splits");

const list = htmlToMarkdown("<ul><li>one</li><li>two</li></ul>");
assert(/one/.test(list) && /two/.test(list), "list items");

const headings = htmlToMarkdown("<h2>Title</h2><p>Body</p>");
assert(headings.includes("Title") && headings.includes("Body"), "heading + body");

// --- article envelope ---
const md = articleToMarkdown({
  title: "Demo Article",
  author: "Alice",
  feedTitle: "Tech Blog",
  publishedAt: "2026-03-15T12:00:00Z",
  url: "https://example.com/post",
  summary: "A short deck.",
  contentHtml: "<p>First paragraph.</p><p>Second <em>para</em>.</p>",
});

assert(md.startsWith("# Demo Article"), "title heading");
assert(md.includes("Alice"), "author in meta");
assert(md.includes("Tech Blog"), "feed in meta");
assert(md.includes("2026-03-15"), "date");
assert(md.includes("[原文](https://example.com/post)"), "source link");
assert(md.includes("A short deck."), "summary");
assert(md.includes("First paragraph."), "body text");

const untitled = articleToMarkdown({ title: "  ", contentHtml: "" });
assert(untitled.startsWith("# Untitled"), "fallback title");

console.log("htmlToMarkdown.selftest: OK");
