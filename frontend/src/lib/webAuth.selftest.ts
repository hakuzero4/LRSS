/**
 * Structural checks for web token gate.
 * Run: npx tsx src/lib/webAuth.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const app = readFileSync(join(root, "App.vue"), "utf8");
const http = readFileSync(join(root, "lib/httpAppsvc.ts"), "utf8");
const web = readFileSync(join(root, "lib/webMode.ts"), "utf8");
const blocked = readFileSync(join(root, "views/WebAuthBlocked.vue"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");

assert(web.includes("webAuthState"), "webAuthState exported");
assert(web.includes("unauthorized"), "unauthorized state");
assert(http.includes("HttpApiError"), "HttpApiError");
assert(http.includes("setWebAuthState"), "http sets auth state");
assert(http.includes("401"), "detects 401");
assert(app.includes("WebAuthBlocked"), "App gates on WebAuthBlocked");
assert(app.includes("webAuthState"), "App reads webAuthState");
assert(blocked.includes("webAuth.title") || blocked.includes("t(\"webAuth"), "blocked uses i18n");
assert(zh.includes("webAuth:"), "zh-CN webAuth strings");
assert(en.includes("webAuth:"), "en-US webAuth strings");

console.log("webAuth.selftest: OK");
