/**
 * Run: npx tsx src/lib/folderCollapse.selftest.ts
 */
import {
  loadCollapsedFolders,
  pruneCollapsedFolders,
  saveCollapsedFolders,
  FOLDER_COLLAPSE_STORAGE_KEY,
} from "./folderCollapse";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

// Memory localStorage shim for Node
const mem = new Map<string, string>();
const g = globalThis as typeof globalThis & { localStorage?: Storage };
g.localStorage = {
  getItem: (k) => mem.get(k) ?? null,
  setItem: (k, v) => {
    mem.set(k, String(v));
  },
  removeItem: (k) => {
    mem.delete(k);
  },
  clear: () => mem.clear(),
  key: () => null,
  length: 0,
};

assert(Object.keys(loadCollapsedFolders()).length === 0, "empty default");

saveCollapsedFolders({ a: true, b: false, c: true });
const loaded = loadCollapsedFolders();
assert(loaded.a === true && loaded.c === true, "round-trip true");
assert(loaded.b === undefined, "false not stored");

const pruned = pruneCollapsedFolders({ a: true, gone: true }, ["a", "x"]);
assert(pruned.a === true && !pruned.gone, "prune removes missing");
assert(FOLDER_COLLAPSE_STORAGE_KEY === "lrss.folderCollapsed", "key");

console.log("folderCollapse.selftest: OK");
