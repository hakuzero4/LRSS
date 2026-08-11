<script setup lang="ts">
import {
  BookOpenText,
  Check,
  ClipboardCopy,
  ExternalLink,
  FileCode2,
  Focus,
  Languages,
  LoaderCircle,
  MessageCircleQuestion,
  Newspaper,
  ShieldAlert,
  Sparkles,
  Star,
  Tags,
  TextQuote,
  X,
} from "@lucide/vue";
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute, plainText } from "@/lib/format";
import { openExternalLink } from "@/lib/openLink";
import {
  readerShellClasses,
  shouldMarkReadOnScrollEnd,
} from "@/lib/readingSettings";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const { t } = useI18n();

/** Right-side Markdown preview panel (owned by ReaderView). */
const markdownOpen = defineModel<boolean>("markdownOpen", { default: false });

const {
  selectedArticle,
  selectedFeed,
  settings,
  toggleStar,
  toggleRead,
  fetchFullContent,
  backendReady,
  zenMode,
  toggleZenMode,
  aiPanel,
  summaryStream,
  translateView,
  aiSummarize,
  aiTranslate,
  aiTranslateSelection,
  aiAsk,
  aiSuggest,
  aiClassify,
} = useRssStore();

const scrollPaneRef = ref<HTMLElement | null>(null);
/** Content root used to scope selection translate (body / bilingual). */
const readerContentRef = ref<HTMLElement | null>(null);
const selectPopupEl = ref<HTMLElement | null>(null);
/** Avoid repeated mark-read while sitting at the bottom. */
const scrollEndMarkedForId = ref<string | null>(null);
const fetchingFull = ref(false);

const SELECT_MAX_CHARS = 2000;
const SELECT_MIN_CHARS = 2;

type SelectPopupState = {
  open: boolean;
  x: number;
  y: number;
  source: string;
  result: string;
  busy: boolean;
  error: string;
  seq: number;
};

const selectPopup = reactive<SelectPopupState>({
  open: false,
  x: 0,
  y: 0,
  source: "",
  result: "",
  busy: false,
  error: "",
  seq: 0,
});
const aiBusy = computed(() => aiPanel.busy);
const summarizeBusy = computed(
  () =>
    summaryStream.busy &&
    summaryStream.articleId === selectedArticle.value?.id,
);
const translateBusy = computed(
  () =>
    translateView.busy &&
    translateView.articleId === selectedArticle.value?.id,
);
const showBilingual = computed(
  () =>
    !!selectedArticle.value &&
    translateView.active &&
    translateView.articleId === selectedArticle.value.id &&
    (translateView.pairs.length > 0 || translateView.busy),
);

/** Live AI summary stream for the selected article. */
const isStreamingSummary = computed(() => {
  const a = selectedArticle.value;
  return (
    !!a &&
    summaryStream.articleId === a.id &&
    (summaryStream.busy || !!summaryStream.text)
  );
});

/**
 * Deck / standfirst under the title.
 * Prefer streaming AI text; else stored summary (possibly AI-replaced).
 * Hidden only when it is essentially the same as the full body text.
 */
const readerSummary = computed(() => {
  const a = selectedArticle.value;
  if (!a) return "";

  // Streaming / just-finished AI deck (show full text, no aggressive clamp).
  if (summaryStream.articleId === a.id && summaryStream.text.trim()) {
    return summaryStream.text.trim();
  }

  if (!a.summary) return "";
  // Keep newlines for AI deck (• bullets); only strip HTML tags if present.
  const raw = String(a.summary);
  const s = /[<>]/.test(raw)
    ? plainText(raw, 4000)
    : raw.replace(/\r\n/g, "\n").trim();
  if (!s || s.length < 12) return "";
  const body = plainText(a.contentHtml, 800);
  if (!body) return s;
  const sOneLine = s.replace(/\s+/g, " ").trim();
  // Exact or near-full duplicate of the article body → no deck.
  if (sOneLine === body) return "";
  if (body.startsWith(sOneLine) && sOneLine.length > body.length * 0.72) return "";
  if (sOneLine.startsWith(body) && body.length > sOneLine.length * 0.72) return "";
  return s;
});

const showSummaryDeck = computed(
  () =>
    !!readerSummary.value ||
    (isStreamingSummary.value && summaryStream.busy),
);

const hasBody = computed(() => {
  const html = selectedArticle.value?.contentHtml?.trim() ?? "";
  return html.length > 0 && plainText(html).length > 0;
});

/** Root classes driven by Settings → Reading (font size + column width). */
const readerShellClass = computed(() => {
  const { className } = readerShellClasses(settings.fontSize, settings.readerWidth);
  return className;
});

async function openOriginal() {
  if (!selectedArticle.value?.url) return;
  await openExternalLink(selectedArticle.value.url, {
    forceBrowser: settings.openLinksInBrowser,
  });
}

/** Toggle right-side Markdown panel (does not copy). */
function toggleMarkdownPanel() {
  if (!selectedArticle.value) return;
  markdownOpen.value = !markdownOpen.value;
}

/** Download original page and replace partial feed body. */
async function onFetchFullContent() {
  const article = selectedArticle.value;
  if (!article?.url || fetchingFull.value) return;
  if (!backendReady.value) {
    toast.error(t("article.fetchFullUnavailable"));
    return;
  }
  fetchingFull.value = true;
  try {
    await fetchFullContent(article.id);
    toast.success(t("article.fetchFullDone"));
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("article.fetchFullFailed"), { description: msg });
  } finally {
    fetchingFull.value = false;
  }
}

async function runAI(action: () => Promise<void>) {
  try {
    await action();
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("ai.failed"), { description: msg });
  }
}

/** Manual deck summary (also available when auto-summarize is off). */
async function onSummarize() {
  const id = selectedArticle.value?.id;
  if (!id) {
    toast.message(t("ai.failed"), { description: t("article.selectTitle") });
    return;
  }
  if (!backendReady.value) {
    toast.error(t("ai.backendUnavailable"));
    return;
  }
  if (summarizeBusy.value) return;
  toast.message(t("ai.summarizeStarting"));
  try {
    await aiSummarize(id);
    toast.success(t("ai.summarizeDone"));
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("ai.failed"), { description: msg });
  }
}

async function onTranslate(lang?: string) {
  const id = selectedArticle.value?.id;
  if (!id) {
    toast.message(t("ai.failed"), { description: t("article.selectTitle") });
    return;
  }
  if (!backendReady.value) {
    toast.error(t("ai.backendUnavailable"));
    return;
  }
  // Toggling off an existing bilingual view — no success toast.
  const togglingOff =
    translateView.active &&
    translateView.articleId === id &&
    !translateView.busy &&
    translateView.pairs.length > 0;
  if (!togglingOff) {
    toast.message(t("ai.translateStarting"));
  }
  try {
    await aiTranslate(id, lang);
    if (togglingOff) return;
    // Always keep original + translation; never toast "replaced original".
    toast.success(t("ai.translateDone"));
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("ai.failed"), { description: msg });
  }
}

/** One-click translate (target follows UI language). */
function onTranslateClick() {
  void onTranslate();
}

function onAsk() {
  const id = selectedArticle.value?.id;
  if (!id) return;
  const q = window.prompt(t("ai.askPrompt"), t("ai.askDefault"));
  if (q === null) return;
  void runAI(() => aiAsk(id, q));
}

function onSuggest() {
  const id = selectedArticle.value?.id;
  if (!id) return;
  void runAI(() => aiSuggest(id));
}

function onClassify() {
  const id = selectedArticle.value?.id;
  if (!id) return;
  void runAI(() => aiClassify(id));
}

function closeSelectPopup() {
  selectPopup.open = false;
  selectPopup.source = "";
  selectPopup.result = "";
  selectPopup.busy = false;
  selectPopup.error = "";
  selectPopup.seq += 1;
}

function clampPopupPosition(left: number, top: number) {
  const pad = 8;
  const w = typeof window !== "undefined" ? window.innerWidth : 800;
  const h = typeof window !== "undefined" ? window.innerHeight : 600;
  const boxW = 320;
  const boxH = 180;
  return {
    x: Math.min(Math.max(pad, left), Math.max(pad, w - boxW - pad)),
    y: Math.min(Math.max(pad, top), Math.max(pad, h - boxH - pad)),
  };
}

function selectionInsideReader(sel: Selection): boolean {
  const root = readerContentRef.value;
  if (!root) return false;
  const a = sel.anchorNode;
  const f = sel.focusNode;
  if (!a || !f) return false;
  return root.contains(a) && root.contains(f);
}

async function runSelectTranslate(text: string) {
  const seq = ++selectPopup.seq;
  selectPopup.busy = true;
  selectPopup.error = "";
  selectPopup.result = "";
  try {
    const out = await aiTranslateSelection(text);
    if (seq !== selectPopup.seq) return;
    selectPopup.result = out;
  } catch (e) {
    if (seq !== selectPopup.seq) return;
    selectPopup.error = e instanceof Error ? e.message : String(e);
  } finally {
    if (seq === selectPopup.seq) selectPopup.busy = false;
  }
}

/** 划词翻译: show floating card and auto-translate selected text. */
function onReaderMouseUp(_ev: MouseEvent) {
  if (!settings.selectTranslate || !backendReady.value) return;
  // Let the browser finalize the selection first.
  window.setTimeout(() => {
    if (!settings.selectTranslate) return;
    const sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) return;
    if (!selectionInsideReader(sel)) return;
    const text = sel.toString().replace(/\s+/g, " ").trim();
    if (text.length < SELECT_MIN_CHARS) return;
    if (text.length > SELECT_MAX_CHARS) {
      toast.message(t("ai.selectTooLong"));
      return;
    }
    // Same selection still open → keep popup (avoid re-fire while interacting).
    if (
      selectPopup.open &&
      selectPopup.source === text &&
      (selectPopup.busy || selectPopup.result || selectPopup.error)
    ) {
      return;
    }
    let rect: DOMRect;
    try {
      rect = sel.getRangeAt(0).getBoundingClientRect();
    } catch {
      return;
    }
    if (!rect || (rect.width === 0 && rect.height === 0)) return;
    const pos = clampPopupPosition(rect.left, rect.bottom + 8);
    selectPopup.open = true;
    selectPopup.x = pos.x;
    selectPopup.y = pos.y;
    selectPopup.source = text;
    selectPopup.result = "";
    selectPopup.error = "";
    void runSelectTranslate(text);
  }, 10);
}

function onDocPointerDown(ev: PointerEvent) {
  if (!selectPopup.open) return;
  const t = ev.target as Node | null;
  if (t && selectPopupEl.value?.contains(t)) return;
  // Clicking elsewhere dismisses; keep selection if user re-selects.
  closeSelectPopup();
}

function onDocKeydown(ev: KeyboardEvent) {
  if (ev.key === "Escape" && selectPopup.open) {
    closeSelectPopup();
  }
}

async function copySelectResult() {
  const text = selectPopup.result.trim();
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    toast.success(t("ai.selectCopied"));
  } catch {
    toast.error(t("ai.copyFailed"));
  }
}

onMounted(() => {
  document.addEventListener("pointerdown", onDocPointerDown, true);
  document.addEventListener("keydown", onDocKeydown, true);
});

onBeforeUnmount(() => {
  document.removeEventListener("pointerdown", onDocPointerDown, true);
  document.removeEventListener("keydown", onDocKeydown, true);
});

watch(
  () => selectedArticle.value?.id,
  () => {
    closeSelectPopup();
  },
);

/** Intercept in-body links so they honor openLinksInBrowser (never leave the app shell). */
function onBodyClick(ev: MouseEvent) {
  const target = ev.target;
  if (!(target instanceof Element)) return;
  const anchor = target.closest("a");
  if (!anchor) return;
  const href = anchor.getAttribute("href");
  if (!href || href.startsWith("#") || href.startsWith("mailto:")) return;
  ev.preventDefault();
  ev.stopPropagation();
  void openExternalLink(href, { forceBrowser: settings.openLinksInBrowser });
}

function onReaderScroll() {
  const el = scrollPaneRef.value;
  const article = selectedArticle.value;
  if (!el || !article) return;
  const should = shouldMarkReadOnScrollEnd({
    enabled: settings.markAsReadOnScrollEnd,
    articleId: article.id,
    alreadyRead: article.read,
    alreadyMarkedId: scrollEndMarkedForId.value,
    scrollHeight: el.scrollHeight,
    scrollTop: el.scrollTop,
    clientHeight: el.clientHeight,
  });
  if (!should) return;
  scrollEndMarkedForId.value = article.id;
  // Only mark unread → read (toggleRead would flip read→unread if misused).
  if (!article.read) {
    void toggleRead(article.id);
  }
}

watch(
  () => selectedArticle.value?.id,
  async () => {
    scrollEndMarkedForId.value = null;
    await nextTick();
    if (scrollPaneRef.value) scrollPaneRef.value.scrollTop = 0;
  },
);
</script>

<template>
  <section class="flex h-full min-h-0 min-w-0 w-full flex-col overflow-hidden bg-background">
    <template v-if="selectedArticle">
      <header class="pane-chrome flex h-12 shrink-0 items-center justify-between gap-2 px-4">
        <div class="min-w-0 flex-1 pr-2">
          <p class="truncate text-[12px] text-muted-foreground">
            {{ selectedFeed?.title ?? t("article.feedFallback") }}
            <span v-if="selectedArticle.author"> · {{ selectedArticle.author }}</span>
          </p>
        </div>

        <TooltipProvider :delay-duration="300">
          <div class="flex shrink-0 items-center gap-0.5">
            <Tooltip v-if="settings.readerToolbar.zen">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :class="
                    zenMode
                      ? 'text-primary bg-primary/10'
                      : 'text-muted-foreground'
                  "
                  :aria-label="
                    zenMode ? t('article.exitZenMode') : t('article.zenMode')
                  "
                  :aria-pressed="zenMode"
                  @click="toggleZenMode"
                >
                  <Focus class="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  zenMode
                    ? t("article.exitZenMode")
                    : t("article.zenMode")
                }}
                <span class="ml-1.5 opacity-60">z</span>
              </TooltipContent>
            </Tooltip>

            <Tooltip v-if="settings.readerToolbar.star">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :class="selectedArticle.starred ? 'text-amber-500' : 'text-muted-foreground'"
                  :aria-label="selectedArticle.starred ? t('article.unstar') : t('article.star')"
                  @click="toggleStar(selectedArticle.id)"
                >
                  <Star
                    class="size-4"
                    :class="selectedArticle.starred && 'fill-current'"
                  />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ selectedArticle.starred ? t("article.unstar") : t("article.star") }}
              </TooltipContent>
            </Tooltip>

            <Tooltip v-if="settings.readerToolbar.read">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="text-muted-foreground"
                  :aria-label="selectedArticle.read ? t('article.markUnread') : t('article.markRead')"
                  @click="toggleRead(selectedArticle.id)"
                >
                  <Check class="size-4" :class="!selectedArticle.read && 'opacity-40'" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ selectedArticle.read ? t("article.markUnread") : t("article.markRead") }}
              </TooltipContent>
            </Tooltip>

            <!-- Summarize: one-click deck summary (works when auto-summarize is off) -->
            <Tooltip v-if="settings.readerToolbar.summarize">
              <TooltipTrigger as-child>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  :class="
                    summarizeBusy ||
                    (summaryStream.articleId === selectedArticle.id &&
                      !!summaryStream.text)
                      ? 'text-primary bg-primary/10'
                      : 'text-muted-foreground'
                  "
                  :disabled="summarizeBusy || !backendReady"
                  :aria-label="t('ai.summarize')"
                  @click="onSummarize"
                >
                  <LoaderCircle
                    v-if="summarizeBusy"
                    class="size-4 animate-spin"
                  />
                  <TextQuote v-else class="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  summarizeBusy
                    ? t("ai.summarizeStarting")
                    : t("ai.summarize")
                }}
              </TooltipContent>
            </Tooltip>

            <!-- Translate: direct click (nested Dropdown+Tooltip was swallowing clicks) -->
            <template v-if="settings.readerToolbar.translate">
              <Tooltip>
                <TooltipTrigger as-child>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    :class="
                      showBilingual || translateBusy
                        ? 'text-primary bg-primary/10'
                        : 'text-muted-foreground'
                    "
                    :disabled="translateBusy"
                    :aria-label="t('ai.translate')"
                    :aria-pressed="showBilingual"
                    @click="onTranslateClick"
                  >
                    <LoaderCircle
                      v-if="translateBusy"
                      class="size-4 animate-spin"
                    />
                    <Languages v-else class="size-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  {{
                    showBilingual
                      ? t("ai.translateShowOriginal")
                      : t("ai.translate")
                  }}
                </TooltipContent>
              </Tooltip>
              <DropdownMenu>
                <DropdownMenuTrigger as-child>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    class="-ml-1.5 min-w-5 px-0.5 text-muted-foreground"
                    :disabled="translateBusy"
                    :aria-label="t('ai.translateMenu')"
                  >
                    <span class="text-[10px] font-semibold leading-none">▾</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" class="w-48">
                  <DropdownMenuLabel>{{ t("ai.translate") }}</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem @select="() => void onTranslate()">
                    {{ t("ai.translateAuto") }}
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="() => void onTranslate('zh-CN')">
                    {{ t("ai.langZh") }}
                  </DropdownMenuItem>
                  <DropdownMenuItem @select="() => void onTranslate('en')">
                    {{ t("ai.langEn") }}
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </template>

            <DropdownMenu v-if="settings.readerToolbar.ai">
              <Tooltip>
                <TooltipTrigger as-child>
                  <DropdownMenuTrigger as-child>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      :class="
                        aiPanel.open || aiBusy
                          ? 'text-primary bg-primary/10'
                          : 'text-muted-foreground'
                      "
                      :disabled="!backendReady || aiBusy"
                      :aria-label="t('ai.menu')"
                    >
                      <LoaderCircle
                        v-if="aiBusy"
                        class="size-4 animate-spin"
                      />
                      <Sparkles v-else class="size-4" />
                    </Button>
                  </DropdownMenuTrigger>
                </TooltipTrigger>
                <TooltipContent>{{ t("ai.menu") }}</TooltipContent>
              </Tooltip>
              <DropdownMenuContent align="end" class="w-52">
                <DropdownMenuLabel>{{ t("ai.menu") }}</DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  :disabled="summarizeBusy"
                  @click="onSummarize"
                >
                  <TextQuote class="mr-2 size-3.5" />
                  {{ t("ai.summarize") }}
                </DropdownMenuItem>
                <DropdownMenuItem @click="onAsk">
                  <MessageCircleQuestion class="mr-2 size-3.5" />
                  {{ t("ai.ask") }}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem @click="onSuggest">
                  <Tags class="mr-2 size-3.5" />
                  {{ t("ai.suggest") }}
                </DropdownMenuItem>
                <DropdownMenuItem @click="onClassify">
                  <ShieldAlert class="mr-2 size-3.5" />
                  {{ t("ai.classify") }}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>

            <Tooltip v-if="settings.readerToolbar.fetchFull">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="text-muted-foreground"
                  :disabled="
                    fetchingFull ||
                    !selectedArticle.url ||
                    !backendReady
                  "
                  :aria-label="
                    fetchingFull
                      ? t('article.fetchFullBusy')
                      : t('article.fetchFull')
                  "
                  @click="onFetchFullContent"
                >
                  <LoaderCircle
                    v-if="fetchingFull"
                    class="size-4 animate-spin"
                  />
                  <Newspaper v-else class="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  fetchingFull
                    ? t("article.fetchFullBusy")
                    : t("article.fetchFull")
                }}
              </TooltipContent>
            </Tooltip>

            <Tooltip v-if="settings.readerToolbar.markdown">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :class="
                    markdownOpen
                      ? 'text-primary bg-primary/10'
                      : 'text-muted-foreground'
                  "
                  :aria-label="
                    markdownOpen
                      ? t('article.closeMarkdownPanel')
                      : t('article.showMarkdown')
                  "
                  :aria-pressed="markdownOpen"
                  @click="toggleMarkdownPanel"
                >
                  <FileCode2 class="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{
                  markdownOpen
                    ? t("article.closeMarkdownPanel")
                    : t("article.showMarkdown")
                }}
              </TooltipContent>
            </Tooltip>

            <Tooltip v-if="settings.readerToolbar.openOriginal">
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="text-muted-foreground"
                  :aria-label="t('article.openOriginal')"
                  @click="openOriginal"
                >
                  <ExternalLink class="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{{ t("article.openOriginal") }}</TooltipContent>
            </Tooltip>
          </div>
        </TooltipProvider>
      </header>

      <div
        ref="scrollPaneRef"
        class="scroll-pane flex-1"
        @scroll.passive="onReaderScroll"
      >
        <!-- Fade + soft drop-in when switching articles -->
        <Transition name="reader-enter" mode="out-in">
        <article
          :key="selectedArticle.id"
          ref="readerContentRef"
          :class="
            cn(
              'reader-shell mx-auto px-6 py-8 sm:px-10 sm:py-10',
              readerShellClass,
            )
          "
          :data-font-size="settings.fontSize"
          :data-reader-width="settings.readerWidth"
          @mouseup="onReaderMouseUp"
        >
          <header class="reader-header-block">
            <p class="text-[12px] font-medium tracking-[0.01em] text-muted-foreground">
              {{ formatAbsolute(selectedArticle.publishedAt) }}
            </p>
            <h1 class="reader-title mt-2 font-semibold tracking-[-0.025em] text-balance">
              {{ selectedArticle.title }}
            </h1>

            <!-- Summary deck: feed or AI (streams in place above body) -->
            <aside
              v-if="showSummaryDeck"
              class="reader-summary"
              :aria-label="t('article.summaryLabel')"
              :data-streaming="isStreamingSummary && summaryStream.busy ? '1' : undefined"
            >
              <div class="reader-summary-inner">
                <p class="reader-summary-label">
                  <span class="reader-summary-label-dot" aria-hidden="true" />
                  {{ t("article.summaryLabel") }}
                  <span
                    v-if="summaryStream.busy && summaryStream.articleId === selectedArticle.id"
                    class="ml-1.5 font-normal text-muted-foreground"
                  >
                    · {{ t("ai.streaming") }}
                  </span>
                </p>
                <p
                  class="reader-summary-text whitespace-pre-wrap"
                  :class="
                    summaryStream.busy &&
                    summaryStream.articleId === selectedArticle.id &&
                    'summary-stream-live'
                  "
                >
                  <template v-if="readerSummary">{{ readerSummary }}</template>
                  <span
                    v-else-if="summaryStream.busy"
                    class="text-muted-foreground"
                  >{{ t("ai.streaming") }}…</span>
                  <span
                    v-if="summaryStream.busy && summaryStream.articleId === selectedArticle.id"
                    class="summary-caret"
                    aria-hidden="true"
                  />
                </p>
                <p
                  v-if="summaryStream.error && summaryStream.articleId === selectedArticle.id"
                  class="mt-2 text-[12px] text-destructive"
                >
                  {{ summaryStream.error }}
                </p>
              </div>
            </aside>
          </header>

          <!-- Bilingual translation (interlinear pairs) -->
          <section
            v-if="showBilingual"
            class="bilingual-view mt-8"
            :aria-label="t('ai.translate')"
            :data-streaming="translateBusy ? '1' : undefined"
          >
            <div class="bilingual-view-head">
              <Languages class="size-3.5 opacity-80" />
              <span>{{ t("ai.bilingualTitle") }}</span>
              <span
                v-if="translateBusy"
                class="font-normal text-muted-foreground"
              >
                · {{ t("ai.streaming") }}
              </span>
            </div>
            <div
              v-if="translateView.pairs.length === 0 && translateBusy"
              class="bilingual-loading text-muted-foreground"
            >
              {{ t("ai.streaming") }}…
              <span class="summary-caret" aria-hidden="true" />
            </div>
            <div
              v-for="(pair, i) in translateView.pairs"
              :key="i"
              class="bilingual-pair"
            >
              <p v-if="pair.original" class="bilingual-original">
                {{ pair.original }}
              </p>
              <p
                v-if="pair.translation"
                class="bilingual-translation"
              >
                {{ pair.translation }}
              </p>
            </div>
            <p
              v-if="translateView.error"
              class="mt-3 text-[12px] text-destructive"
            >
              {{ translateView.error }}
            </p>
          </section>

          <template v-else-if="hasBody">
            <div
              v-if="showSummaryDeck"
              class="reader-body-rule"
              role="separator"
              :aria-label="t('article.bodyLabel')"
            >
              {{ t("article.bodyLabel") }}
            </div>
            <div
              :class="
                cn(
                  'reader-body text-foreground/90',
                  showSummaryDeck ? 'mt-4' : 'mt-8',
                )
              "
              @click="onBodyClick"
              v-html="selectedArticle.contentHtml"
            />
          </template>
        </article>
        </Transition>
      </div>
    </template>

    <div
      v-else
      class="flex flex-1 flex-col items-center justify-center px-8 text-center"
    >
      <div
        class="mb-4 flex size-14 items-center justify-center rounded-2xl border border-border/70 bg-muted/40 shadow-sm"
      >
        <BookOpenText class="size-6 text-muted-foreground" />
      </div>
      <h2 class="text-[15px] font-semibold tracking-tight">{{ t("article.selectTitle") }}</h2>
      <p class="mt-1.5 max-w-xs text-[13px] leading-relaxed text-muted-foreground">
        {{ t("article.selectHint") }}
      </p>
    </div>

    <!-- 划词翻译 floating card (fixed to viewport near selection) -->
    <div
      v-if="selectPopup.open && settings.selectTranslate"
      ref="selectPopupEl"
      class="select-translate-popup"
      :style="{ left: `${selectPopup.x}px`, top: `${selectPopup.y}px` }"
      role="dialog"
      :aria-label="t('ai.selectTranslate')"
      @mousedown.stop
      @mouseup.stop
    >
      <div class="select-translate-head">
        <Languages class="size-3.5 opacity-80" />
        <span>{{ t("ai.selectTranslate") }}</span>
        <button
          type="button"
          class="select-translate-close"
          :aria-label="t('ai.closePanel')"
          @click="closeSelectPopup"
        >
          <X class="size-3.5" />
        </button>
      </div>
      <p v-if="selectPopup.source" class="select-translate-source">
        {{ selectPopup.source }}
      </p>
      <div class="select-translate-body">
        <template v-if="selectPopup.busy">
          <LoaderCircle class="size-3.5 animate-spin opacity-70" />
          <span>{{ t("ai.working") }}</span>
        </template>
        <p v-else-if="selectPopup.error" class="select-translate-error">
          {{ selectPopup.error }}
        </p>
        <p v-else-if="selectPopup.result" class="select-translate-result">
          {{ selectPopup.result }}
        </p>
      </div>
      <div
        v-if="selectPopup.result && !selectPopup.busy"
        class="select-translate-actions"
      >
        <button type="button" class="select-translate-action" @click="copySelectResult">
          <ClipboardCopy class="size-3.5" />
          {{ t("ai.selectCopy") }}
        </button>
      </div>
    </div>
  </section>
</template>
