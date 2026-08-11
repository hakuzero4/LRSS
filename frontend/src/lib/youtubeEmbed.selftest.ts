/**
 * Run: npx tsx frontend/src/lib/youtubeEmbed.selftest.ts
 */
import {
  articleDisplayHTML,
  buildYouTubeEmbedHTML,
  youtubeVideoIdFromURL,
} from "./youtubeEmbed.ts";

function assert(cond: unknown, msg: string) {
  if (!cond) throw new Error(msg);
}

assert(youtubeVideoIdFromURL("https://www.youtube.com/watch?v=o8-EgQhqdU0") === "o8-EgQhqdU0", "watch");
assert(youtubeVideoIdFromURL("https://youtu.be/o8-EgQhqdU0") === "o8-EgQhqdU0", "short");
assert(youtubeVideoIdFromURL("https://example.com") === null, "non-yt");

const html = buildYouTubeEmbedHTML("o8-EgQhqdU0", "Hello <b>x</b>");
assert(html.includes("youtube-nocookie.com/embed/o8-EgQhqdU0"), "embed");
assert(html.includes("Hello &lt;b&gt;x&lt;/b&gt;"), "escaped desc");
assert(!html.includes("<b>x</b>"), "no raw html in desc");

const fallback = articleDisplayHTML({
  contentHtml: "",
  url: "https://www.youtube.com/watch?v=o8-EgQhqdU0",
  summary: "desc",
});
assert(fallback.includes("embed/o8-EgQhqdU0"), "fallback embed");

const preferStored = articleDisplayHTML({
  contentHtml: "<p>stored</p>",
  url: "https://www.youtube.com/watch?v=o8-EgQhqdU0",
});
assert(preferStored === "<p>stored</p>", "prefer stored body");

console.log("youtubeEmbed.selftest: ok");
