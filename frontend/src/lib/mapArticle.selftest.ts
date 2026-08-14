/**
 * Run: npx tsx src/lib/mapArticle.selftest.ts
 *
 * Proves mapArticle keeps full AI summaries (not hard-clamped to ~320)
 * for the reader deck field after list/get remapping.
 */
import { mapArticle } from "./backend.ts";
import { plainText } from "./format.ts";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// Long AI-style deck summary (well over 320 chars).
const longSummary = Array.from({ length: 12 }, (_, i) =>
  `• Point ${i + 1}: detailed takeaway about the article topic with enough words to exceed the old clamp.`,
).join("\n");

assert(longSummary.length > 320, "fixture longer than 320");

const mapped = mapArticle({
  id: "a1",
  feedId: "f1",
  title: "Sample",
  summary: longSummary,
  contentHtml: "<p>Distinct article body that is not a duplicate of the summary deck text.</p>",
  url: "https://example.com/x",
  publishedAt: new Date().toISOString(),
  isRead: false,
  isStarred: false,
  fullContentFetched: false,
});

assert(typeof mapped.summary === "string", "summary string");
assert(
  mapped.summary.length > 320,
  `summary must retain full AI deck (got len=${mapped.summary.length})`,
);
const pascal = mapArticle({
  ID: "a2",
  FeedID: "f2",
  Title: "Pascal",
  Summary: "s",
  URL: "https://example.com/p",
  PublishedAt: new Date().toISOString(),
});
assert(pascal.id === "a2" && pascal.feedId === "f2", "mapArticle accepts PascalCase id/feedId");

assert(
  mapped.summary.includes("Point 12"),
  "last bullet preserved after mapArticle",
);
assert(
  mapped.summary.includes("Point 1"),
  "first bullet preserved",
);

// List teaser path still clamps at display (not mapArticle).
const teaser = plainText(mapped.summary, 160);
assert(teaser.length <= 165, "list teaser clamp at display");
assert(teaser.length < mapped.summary.length, "teaser shorter than deck field");

// HTML in summary is stripped but length not hard-clamped to 320.
const htmlSum = mapArticle({
  id: "a2",
  summary: `<p>${"word ".repeat(100)}</p>`,
  contentHtml: "<p>Other body content for uniqueness here.</p>",
  title: "T",
  url: "https://example.com/y",
  publishedAt: new Date().toISOString(),
});
assert(!htmlSum.summary.includes("<p>"), "tags stripped");
assert(htmlSum.summary.length > 200, "html summary not crushed to empty/tiny");

console.log("mapArticle.selftest: OK", {
  deckLen: mapped.summary.length,
  teaserLen: teaser.length,
});
