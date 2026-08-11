/**
 * Run: npx tsx src/lib/bilingualLayout.selftest.ts
 *
 * Static structural check: bilingual view must not exclusively replace
 * original body HTML (v-else-if). Original body stays mounted via v-if="hasBody".
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const here = dirname(fileURLToPath(import.meta.url));
const readerPath = join(here, "../components/article/ArticleReader.vue");
const src = readFileSync(readerPath, "utf8");

assert(src.includes('v-if="showBilingual"'), "bilingual section present");
assert(
  src.includes('data-bilingual-with-original="1"') ||
    src.includes("data-bilingual-with-original"),
  "marker that bilingual coexists with original",
);
assert(src.includes('data-reader-original-body="1"'), "original body marker");
assert(src.includes('v-if="hasBody"'), "original body gated with v-if hasBody");
// Must NOT use exclusive v-else-if that hides original when bilingual is on.
assert(
  !src.includes('v-else-if="hasBody"'),
  "must not hide original body behind v-else-if when bilingual active",
);
assert(src.includes("v-html=\"selectedArticle.contentHtml\""), "original HTML still rendered");
assert(
  src.includes('t("ai.translateShowOriginal")') ||
    src.includes("ai.translateShowOriginal"),
  "label path for original while bilingual is active",
);

console.log("bilingualLayout.selftest: OK");
