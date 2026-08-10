/**
 * Self-test for article filter helpers (Settings → Filters).
 * Run: npx tsx src/lib/articleFilters.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import {
  applyArticleFilters,
  applyBlockKeywords,
  applyHideDuplicateTitles,
  matchesBlockKeywords,
  normalizeTitle,
  parseBlockKeywords,
} from "./articleFilters";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// parse keywords
assert(
  JSON.stringify(parseBlockKeywords("广告, 促销, sponsored")) ===
    JSON.stringify(["广告", "促销", "sponsored"]),
  "parse comma",
);
assert(
  JSON.stringify(parseBlockKeywords("a，b；c|d")) ===
    JSON.stringify(["a", "b", "c", "d"]),
  "parse zh separators",
);
assert(parseBlockKeywords("  ,  ").length === 0, "empty tokens");
assert(parseBlockKeywords("").length === 0, "empty string");

// match
const sample = { id: "1", title: "Weekly Sponsored Deal", summary: "hello" };
assert(matchesBlockKeywords(sample, ["sponsored"]), "title hit");
assert(matchesBlockKeywords({ id: "2", title: "News", summary: "广告来了" }, ["广告"]), "summary hit");
assert(!matchesBlockKeywords(sample, ["xyz"]), "no hit");

// block filter
const blocked = applyBlockKeywords(
  [
    { id: "a", title: "OK post", summary: "" },
    { id: "b", title: "广告推广", summary: "" },
    { id: "c", title: "Tech", summary: "contains promo word" },
  ],
  "广告, promo",
);
assert(
  blocked.map((x) => x.id).join(",") === "a",
  `block filter got ${blocked.map((x) => x.id)}`,
);

// normalize + duplicates (keep first = newest when pre-sorted)
assert(normalizeTitle("  Hello   World ") === "hello world", "normalize space");
const dups = applyHideDuplicateTitles(
  [
    { id: "new", title: "Same Title", publishedAt: "2026-01-02" },
    { id: "old", title: "same title", publishedAt: "2026-01-01" },
    { id: "other", title: "Different", publishedAt: "2026-01-03" },
  ],
  true,
);
assert(
  dups.map((x) => x.id).join(",") === "new,other",
  `dups got ${dups.map((x) => x.id)}`,
);
assert(
  applyHideDuplicateTitles([{ id: "1", title: "A" }], false).length === 1,
  "disabled dups",
);

// pipeline
const pipe = applyArticleFilters(
  [
    { id: "1", title: "Dup", summary: "" },
    { id: "2", title: "dup", summary: "" },
    { id: "3", title: "Keep me", summary: "广告" },
    { id: "4", title: "Also keep", summary: "fine" },
  ],
  { hideDuplicateTitles: true, blockKeywords: "广告" },
);
assert(
  pipe.map((x) => x.id).join(",") === "1,4",
  `pipeline got ${pipe.map((x) => x.id)}`,
);

// wiring: store applies filters
const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
assert(store.includes("applyArticleFilters"), "store must call applyArticleFilters");
assert(store.includes("hideDuplicateTitles"), "store reads hideDuplicateTitles");
assert(store.includes("blockKeywords"), "store reads blockKeywords");

console.log("articleFilters.selftest: OK");
