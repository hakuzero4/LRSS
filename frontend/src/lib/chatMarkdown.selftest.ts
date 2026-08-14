/**
 * Run: npx tsx src/lib/chatMarkdown.selftest.ts
 */
import { citeNFromHref, extractCiteNs, linkifyCiteMarkers, renderChatMarkdown } from "./chatMarkdown";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

assert(renderChatMarkdown("") === "", "empty");
assert(!renderChatMarkdown("<script>x</script>").includes("<script>"), "escapes html");
assert(renderChatMarkdown("see [1] now").includes('data-cite="1"'), "cite button");
assert(renderChatMarkdown("- a\n- b").includes("<li>"), "list");
assert(renderChatMarkdown("**bold**").includes("<strong>bold</strong>"), "bold");
assert(extractCiteNs("x [1] y [7] [1]").join(",") === "1,7", "cite ns");
assert(extractCiteNs("[0] nope").length === 0, "drop zero");
assert(linkifyCiteMarkers("see [1] and [2]").includes("(#lrss-cite-1)"), "linkify [1]");
assert(linkifyCiteMarkers("see [1] and [2]").includes("[[1]]"), "link text keeps [1]");
assert(citeNFromHref("#lrss-cite-3") === 3, "parse cite href");
assert(citeNFromHref("#/lrss-cite-6") === 6, "parse vue-router hash");
assert(citeNFromHref("https://wails.localhost:9245/#/lrss-cite-6") === 6, "parse full wails hash");
assert(citeNFromHref("https://example.com") === null, "ignore normal urls");

console.log("chatMarkdown.selftest: OK");
