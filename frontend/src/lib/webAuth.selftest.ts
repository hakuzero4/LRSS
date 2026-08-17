/**
 * Structural checks for web token gate.
 * Run: npx tsx src/lib/webAuth.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { isWailsHost } from "./webMode";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const app = readFileSync(join(root, "App.vue"), "utf8");
const http = readFileSync(join(root, "lib/httpAppsvc.ts"), "utf8");
const web = readFileSync(join(root, "lib/webMode.ts"), "utf8");
const backend = readFileSync(join(root, "lib/backend.ts"), "utf8");
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
assert(backend.includes("isWailsRuntime"), "desktop uses Wails bindings, not /api/meta");
assert(http.includes("isWailsHost"), "http adapter skips wails.localhost");
assert(http.includes("/api/ai/chat"), "http adapter exposes reading assistant");
assert(!http.includes("desktop-only"), "chat is not desktop-only");
assert(http.includes("content-type") || http.includes("contentType") || http.includes("includes(\"json\")"), "reject HTML as /api/meta");
const viteCfg = readFileSync(join(root, "..", "vite.config.ts"), "utf8");
assert(viteCfg.includes("lrss-api-not-web") || viteCfg.includes("/api/meta"), "vite does not SPA-fallback /api/meta");
assert(isWailsHost("wails.localhost") === true, "desktop host");
assert(isWailsHost("app.wails.localhost") === true, "desktop subdomain");
assert(isWailsHost("127.0.0.1") === false, "loopback is not wails");
assert(isWailsHost("10.1.1.10") === false, "lan web access is not wails");

console.log("webAuth.selftest: OK");
