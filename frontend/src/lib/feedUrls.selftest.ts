/**
 * Run: npx tsx src/lib/feedUrls.selftest.ts
 */
import { parseFeedUrlsFromText } from "./feedUrls";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

assert(parseFeedUrlsFromText("").length === 0, "empty");
assert(
  parseFeedUrlsFromText("https://a.com/f.xml\nhttps://b.com/atom\n# c\nftp://x.com").join(",") ===
    "https://a.com/f.xml,https://b.com/atom",
  "lines + skip comment + ftp",
);
assert(
  parseFeedUrlsFromText("https://a.com/1, https://a.com/1\nhttps://a.com/2").length === 2,
  "dedupe + comma",
);

console.log("feedUrls.selftest: OK");
