/**
 * Structural checks for reading-settings wiring.
 * Run: npx tsx src/lib/openLink.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const css = readFileSync(join(root, "style.css"), "utf8");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const openLink = readFileSync(join(root, "lib/openLink.ts"), "utf8");

assert(reader.includes("readerShellClass") || reader.includes("reader-font-"), "reader applies font classes");
assert(reader.includes("reader-width-") || reader.includes("readerWidth"), "reader applies width");
assert(reader.includes("openExternalLink") || reader.includes("openLink"), "reader opens links via helper");
assert(reader.includes("onBodyClick"), "body link intercept");
assert(reader.includes("markAsReadOnScrollEnd"), "scroll-end mark read");
assert(reader.includes("onReaderScroll"), "scroll handler");

assert(css.includes("reader-font-sm"), "css sm font");
assert(css.includes("reader-font-lg"), "css lg font");
assert(css.includes("reader-width-narrow"), "css narrow width");
assert(css.includes("reader-width-wide"), "css wide width");
assert(css.includes("--reader-body-size"), "css body size var");

assert(store.includes("showUnreadOnly"), "unread-only setting");
assert(store.includes("openLinksInBrowser"), "open links setting");
assert(store.includes("fontSize"), "fontSize setting");
assert(store.includes("readerWidth"), "readerWidth setting");

assert(openLink.includes("Browser"), "uses Wails Browser API");
assert(openLink.includes("OpenURL"), "OpenURL call");

console.log("openLink.selftest (reading settings wiring): OK");
