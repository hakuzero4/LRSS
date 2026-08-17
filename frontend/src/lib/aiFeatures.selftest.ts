/**
 * Structural wiring for P0/P1 AI features.
 * Run: npx tsx src/lib/aiFeatures.selftest.ts
 */
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const reader = readFileSync(join(root, "components/article/ArticleReader.vue"), "utf8");
const panel = readFileSync(join(root, "components/article/AssistantPane.vue"), "utf8");
const view = readFileSync(join(root, "views/ReaderView.vue"), "utf8");
const layout = readFileSync(join(root, "layouts/AppLayout.vue"), "utf8");
const sidebar = readFileSync(join(root, "components/layout/AppSidebar.vue"), "utf8");
const store = readFileSync(join(root, "composables/useRssStore.ts"), "utf8");
const zh = readFileSync(join(root, "i18n/locales/zh-CN.ts"), "utf8");
const en = readFileSync(join(root, "i18n/locales/en-US.ts"), "utf8");
const aisvc = readFileSync(
  join(root, "../bindings/lrss/internal/appsvc/aiservice.ts"),
  "utf8",
);
const http = readFileSync(join(root, "lib/httpAppsvc.ts"), "utf8");

for (const key of [
  "aiSummarize",
  "aiTranslate",
  "aiAsk",
  "aiSuggest",
  "aiClassify",
  "assistant",
  "assistantSend",
  "openAssistant",
]) {
  assert(store.includes(key), `store exports/uses ${key}`);
}
assert(!store.includes("aiDailyDigest"), "store must not export daily digest");

assert(reader.includes("onSummarize") || reader.includes("aiSummarize"), "reader summarize");
assert(reader.includes("onTranslate") || reader.includes("aiTranslate"), "reader translate");
assert(!reader.includes("onToggleAssistant"), "reader toolbar is not the assistant entry");
assert(!reader.includes("Sparkles"), "sparkles is not in the article toolbar");
assert(reader.includes("openAssistant"), "reader can still attach-and-ask from selection");
assert(!reader.includes("onSuggest"), "suggest is not a toolbar menu");
assert(!reader.includes("onClassify"), "classify is not a toolbar menu");
assert(!reader.includes("window.prompt"), "ask no longer uses window.prompt");
assert(sidebar.includes("toggleAssistant"), "sidebar owns the global assistant toggle");
assert(layout.includes("AssistantPane"), "shell hosts the global assistant pane");
assert(!view.includes("AssistantPane"), "reader view does not host assistant");
assert(panel.includes("chipSuggest") || panel.includes("Suggestion"), "suggest lives in assistant chips");

assert(panel.includes("assistantTitle") || panel.includes("PromptInput"), "assistant pane");
assert(panel.includes("ConversationEmptyState"), "official conversation empty state");
assert(panel.includes("vue-stream-markdown") || panel.includes("MessageResponse"), "official message response");
assert(panel.includes("PromptInputActionMenu"), "prompt input can attach articles");
assert(panel.includes("PromptInputCommandInput"), "attach menu has a filter input");
assert(panel.includes("attachFilter") || panel.includes("onAttachFilter"), "attach menu filters");
assert(zh.includes("attachFilter:"), "zh i18n attach filter");
assert(en.includes("attachFilter:"), "en i18n attach filter");
assert(panel.includes("searchLibrary") || panel.includes("useLibrary"), "library search toggle");
assert(panel.includes("linkifyCiteMarkers"), "replies linkify [n] citations");
assert(panel.includes("AssistantCiteLink") || panel.includes("citeRenderers"), "cites use in-app buttons not hash links");
const citeLink = readFileSync(join(root, "components/article/AssistantCiteLink.vue"), "utf8");
assert(citeLink.includes("chat-cite"), "cite link is a button");
assert(!citeLink.includes("<a "), "cite renderer must not emit <a href>");
assert(store.includes("attachIds") || store.includes("attaches"), "store tracks attached articles");
assert(store.includes("useLibrary"), "store sends useLibrary");
assert(store.includes("keepAssistant"), "citation jump can keep the chat");
assert(panel.includes("keepAssistant: true") || panel.includes("keepAssistant:true"), "cite click keeps assistant");
assert(store.includes("toggleAssistant"), "store toggleAssistant");

assert(!sidebar.includes("aiDailyDigest") && !sidebar.includes("onDailyDigest"), "no sidebar digest");
assert(!sidebar.includes("dailyDigest"), "no digest label in sidebar");

for (const [name, src] of [
  ["zh", zh],
  ["en", en],
] as const) {
  for (const k of ["summarize:", "translate:", "ask:", "suggest:", "classify:", "assistantTitle:"]) {
    assert(src.includes(k), `${name} i18n has ${k}`);
  }
  assert(!src.includes("dailyDigest:"), `${name} i18n must drop dailyDigest`);
}

for (const fn of [
  "Summarize",
  "Translate",
  "Ask",
  "SuggestFolders",
  "ClassifyPromo",
  "ApplySuggestedFolder",
  "ChatSend",
  "ChatHistory",
  "ChatClear",
]) {
  assert(aisvc.includes(`function ${fn}`), `binding ${fn}`);
}
assert(!aisvc.includes("function DailyDigest"), "binding must drop DailyDigest");

assert(http.includes('postJSON("/api/ai/chat"'), "web ChatSend hits /api/ai/chat");
assert(http.includes("/api/ai/chat/clear"), "web ChatClear");
assert(http.includes("/api/ai/chat/cancel"), "web ChatCancel");
assert(http.includes("/api/ai/stream") || store.includes("/api/ai/stream"), "web llm stream");
assert(!http.includes("desktop-only"), "reading assistant is not desktop-only");
assert(!panel.includes("webMode.value || !assistant.llmConfigured"), "web can compose");
assert(sidebar.includes("toggleAssistant"), "sidebar assistant toggle stays visible");
assert(sidebar.includes('v-if="!webMode"') && sidebar.includes("openSettings"), "settings stay desktop-only");

console.log("aiFeatures.selftest: OK");
