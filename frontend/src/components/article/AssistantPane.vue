<script setup lang="ts">
import { BookOpenText, Eraser, Library, Sparkles, X } from "@lucide/vue";
import { computed, provide, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { loadAppsvc } from "@/lib/backend";
import {
  Conversation,
  ConversationContent,
  ConversationEmptyState,
  ConversationScrollButton,
} from "@/components/ai-elements/conversation";
import { Loader } from "@/components/ai-elements/loader";
import { Message, MessageContent, MessageResponse } from "@/components/ai-elements/message";
import {
  PromptInput,
  PromptInputActionMenu,
  PromptInputActionMenuContent,
  PromptInputActionMenuTrigger,
  PromptInputCommand,
  PromptInputCommandEmpty,
  PromptInputCommandGroup,
  PromptInputCommandInput,
  PromptInputCommandItem,
  PromptInputCommandList,
  PromptInputBody,
  PromptInputButton,
  PromptInputFooter,
  PromptInputHeader,
  PromptInputSubmit,
  PromptInputTextarea,
  PromptInputTools,
  type PromptInputMessage,
} from "@/components/ai-elements/prompt-input";
import { Suggestion, Suggestions } from "@/components/ai-elements/suggestion";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useRssStore, type AssistantCitation } from "@/composables/useRssStore";
import AssistantCiteLink from "@/components/article/AssistantCiteLink.vue";
import { linkifyCiteMarkers } from "@/lib/chatMarkdown";

const { t } = useI18n();
const {
  assistant,
  selectedArticle,
  selectedFeed,
  filteredArticles,
  articles,
  feeds,
  closeAssistant,
  assistantSend,
  assistantClear,
  assistantCancel,
  assistantAttach,
  assistantDetach,
  setAssistantUseLibrary,
  selectArticle,
} = useRssStore();

const articleChips = computed(() => [
  { id: "about", label: t("ai.chipAbout") },
  { id: "facts", label: t("ai.chipFacts") },
  { id: "gaps", label: t("ai.chipGaps") },
  { id: "suggest", label: t("ai.chipSuggest") },
  { id: "classify", label: t("ai.chipClassify") },
]);

const libraryChips = computed(() => {
  const out = [{ id: "readFirst", label: t("ai.chipReadFirst"), library: true }];
  if (selectedArticle.value) {
    out.push({ id: "related", label: t("ai.chipRelated"), library: true });
  }
  return out;
});

const empty = computed(() => assistant.messages.length === 0);
const composerDisabled = computed(() => !assistant.llmConfigured);
const status = computed(() => {
  if (assistant.busy) return "streaming" as const;
  if (assistant.error) return "error" as const;
  return "ready" as const;
});

const promptKey = computed(
  () => `${assistant.draft}\0${assistant.selection}`,
);

function feedTitleOf(feedId: string | undefined) {
  if (!feedId) return "";
  return feeds.value.find((f) => f.id === feedId)?.title ?? "";
}

type AttachPick = { id: string; title: string; feedTitle: string };

const attachFilter = ref("");
const attachHits = ref<AttachPick[]>([]);
const attachSearching = ref(false);
let attachSearchSeq = 0;
let attachSearchTimer: ReturnType<typeof setTimeout> | null = null;

function takenAttachIds() {
  const taken = new Set(assistant.attaches.map((a) => a.id));
  if (selectedArticle.value) taken.add(selectedArticle.value.id);
  return taken;
}

const attachCandidates = computed(() => {
  const taken = takenAttachIds();
  const q = attachFilter.value.trim();
  const seen = new Set<string>();
  const out: AttachPick[] = [];
  const add = (id: string, title: string, feedTitle: string) => {
    if (!id || taken.has(id) || seen.has(id)) return;
    seen.add(id);
    out.push({ id, title, feedTitle });
  };
  for (const a of filteredArticles.value) {
    add(a.id, a.title, feedTitleOf(a.feedId));
  }
  if (q) {
    for (const a of articles.value) {
      add(a.id, a.title, feedTitleOf(a.feedId));
    }
  }
  return out.slice(0, q ? 40 : 20);
});

const attachLibraryHits = computed(() => {
  const taken = takenAttachIds();
  for (const c of attachCandidates.value) taken.add(c.id);
  return attachHits.value.filter((h) => h.id && !taken.has(h.id));
});

function resetAttachSearch() {
  if (attachSearchTimer) {
    clearTimeout(attachSearchTimer);
    attachSearchTimer = null;
  }
  attachSearchSeq += 1;
  attachFilter.value = "";
  attachHits.value = [];
  attachSearching.value = false;
}

function onAttachMenuOpen(open: boolean) {
  if (!open) resetAttachSearch();
}

function onAttachFilter(value: unknown) {
  const q = String(value ?? "");
  attachFilter.value = q;
  if (attachSearchTimer) clearTimeout(attachSearchTimer);
  const trimmed = q.trim();
  if (trimmed.length < 2) {
    attachHits.value = [];
    attachSearching.value = false;
    attachSearchSeq += 1;
    return;
  }
  attachSearching.value = true;
  const seq = ++attachSearchSeq;
  attachSearchTimer = setTimeout(() => {
    void searchAttachLibrary(trimmed, seq);
  }, 220);
}

async function searchAttachLibrary(query: string, seq: number) {
  try {
    const api = await loadAppsvc();
    const fn = api?.SearchService?.Search;
    if (typeof fn !== "function") {
      if (seq === attachSearchSeq) attachSearching.value = false;
      return;
    }
    const raw = await fn(query, "auto", 24);
    if (seq !== attachSearchSeq) return;
    const hits = (raw?.hits ?? raw?.Hits ?? []) as Array<{
      articleId?: string;
      ArticleId?: string;
      title?: string;
      Title?: string;
      snippet?: string;
      Snippet?: string;
    }>;
    attachHits.value = hits
      .map((h) => ({
        id: String(h.articleId ?? h.ArticleId ?? "").trim(),
        title: String(h.title ?? h.Title ?? "").trim() || String(h.snippet ?? h.Snippet ?? "").trim(),
        feedTitle: "",
      }))
      .filter((h) => h.id);
  } catch {
    if (seq === attachSearchSeq) attachHits.value = [];
  } finally {
    if (seq === attachSearchSeq) attachSearching.value = false;
  }
}

function onAttachCommandKeydown(e: KeyboardEvent) {
  if (e.key !== "Escape") e.stopPropagation();
}

const canAttachCurrent = computed(() => {
  const cur = selectedArticle.value;
  if (!cur) return false;
  return !assistant.attaches.some((a) => a.id === cur.id);
});

function attachCurrent() {
  const cur = selectedArticle.value;
  if (!cur) return;
  assistantAttach({
    id: cur.id,
    title: cur.title,
    feedTitle: selectedFeed.value?.title,
  });
}

async function sendText(text: string, useLibrary?: boolean) {
  try {
    await assistantSend(text, useLibrary ? { useLibrary: true } : undefined);
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("ai.failed"), { description: msg });
  }
}

async function onPromptSubmit(msg: PromptInputMessage) {
  const text = String(msg.text ?? "").trim();
  if (assistant.busy) {
    void assistantCancel();
    return;
  }
  if (composerDisabled.value || !text) return;
  await sendText(text);
}

async function onChip(label: string, library = false) {
  if (composerDisabled.value || assistant.busy) return;
  if (library) setAssistantUseLibrary(true);
  else attachCurrent();
  await sendText(label, library);
}

function clearSelectionChip() {
  assistant.selection = "";
}

function citationFor(n: number, local?: AssistantCitation[]): AssistantCitation | undefined {
  if (local?.length) {
    const hit = local.find((c) => c.n === n);
    if (hit) return hit;
  }
  for (let i = assistant.messages.length - 1; i >= 0; i--) {
    const hit = assistant.messages[i].citations?.find((c) => c.n === n);
    if (hit) return hit;
  }
  return undefined;
}

function openCite(n: number, local?: AssistantCitation[]) {
  const hit = citationFor(n, local);
  if (hit?.articleId) void selectArticle(hit.articleId, { keepAssistant: true });
}

const citeRenderers = { link: AssistantCiteLink };
provide("lrssOpenCite", (n: number) => openCite(n));
</script>

<template>
  <section
    class="side-pane flex h-full min-h-0 min-w-0 w-full flex-col overflow-hidden"
    role="complementary"
    :aria-label="t('ai.assistantTitle')"
  >
    <header class="pane-chrome flex h-12 shrink-0 items-center justify-between gap-2 px-3">
      <div class="flex min-w-0 flex-1 items-center gap-2">
        <Sparkles class="size-4 shrink-0 text-primary" aria-hidden="true" />
        <div class="min-w-0">
          <p class="truncate text-[12px] font-medium text-foreground">
            {{ t("ai.assistantTitle") }}
          </p>
          <p class="truncate text-[11px] text-muted-foreground">
            <span v-if="assistant.busy">{{ t("ai.working") }}</span>
            <span v-else-if="assistant.model">{{ assistant.model }}</span>
            <span v-else-if="!assistant.llmConfigured">{{ t("ai.needModel") }}</span>
            <span v-else>{{ t("ai.assistantHint") }}</span>
          </p>
        </div>
      </div>
      <TooltipProvider :delay-duration="300">
        <div class="flex shrink-0 items-center gap-0.5">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-muted-foreground"
                :disabled="empty && !assistant.selection && assistant.attaches.length === 0"
                :aria-label="t('ai.clearChat')"
                @click="assistantClear"
              >
                <Eraser class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ t("ai.clearChat") }}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-muted-foreground"
                :aria-label="t('ai.closePanel')"
                @click="closeAssistant"
              >
                <X class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ t("ai.closePanel") }}</TooltipContent>
          </Tooltip>
        </div>
      </TooltipProvider>
    </header>

    <Conversation class="min-h-0" :aria-label="t('ai.assistantTitle')">
      <ConversationContent class="gap-4 p-3">
        <ConversationEmptyState
          v-if="empty && !assistant.busy"
          :title="t('ai.assistantTitle')"
          :description="selectedArticle ? t('ai.emptyArticle') : t('ai.emptyNoArticle')"
        >
          <template #icon>
            <Sparkles class="size-5" />
          </template>
        </ConversationEmptyState>

        <Suggestions
          v-if="empty && !assistant.busy && assistant.llmConfigured"
          class="max-w-full flex-wrap"
        >
          <template v-if="selectedArticle">
            <Suggestion
              v-for="chip in articleChips"
              :key="chip.id"
              :suggestion="chip.label"
              @click="(label: string) => onChip(label, false)"
            />
          </template>
          <Suggestion
            v-for="chip in libraryChips"
            :key="chip.id"
            :suggestion="chip.label"
            @click="(label: string) => onChip(label, true)"
          />
        </Suggestions>

        <template v-else>
          <Message
            v-for="msg in assistant.messages"
            :key="msg.id"
            :from="msg.role"
          >
            <MessageContent>
              <div
                v-if="msg.role === 'assistant' && !msg.content && assistant.busy"
                class="flex items-center gap-2 text-muted-foreground"
              >
                <Loader :size="16" />
                {{ t("ai.working") }}
              </div>
              <div v-else class="min-w-0">
                <p
                  v-if="msg.role === 'user'"
                  class="whitespace-pre-wrap break-words"
                >
                  {{ msg.content }}
                </p>
                <MessageResponse
                  v-else
                  :content="linkifyCiteMarkers(msg.content)"
                  :node-renderers="citeRenderers"
                />
                <div
                  v-if="msg.role === 'assistant' && msg.citations?.length"
                  class="mt-2 flex flex-col gap-1"
                >
                  <p class="text-[11px] font-medium text-muted-foreground">
                    {{ t("ai.sources") }}
                  </p>
                  <button
                    v-for="c in msg.citations"
                    :key="`${msg.id}-${c.n}`"
                    type="button"
                    class="flex min-w-0 items-center gap-1.5 truncate text-left text-[12px] text-primary hover:underline"
                    @click.stop="openCite(c.n, msg.citations)"
                  >
                    <BookOpenText class="size-3 shrink-0" />
                    <span class="truncate">[{{ c.n }}] {{ c.title || c.articleId }}</span>
                  </button>
                </div>
              </div>
            </MessageContent>
          </Message>
        </template>
      </ConversationContent>
      <ConversationScrollButton />
    </Conversation>

    <p
      v-if="assistant.error"
      class="shrink-0 px-3 pb-1 text-[11.5px] text-destructive"
    >
      {{ assistant.error }}
    </p>

    <PromptInput
      :key="promptKey"
      class="shrink-0 p-3 pt-1"
      :initial-input="assistant.draft"
      @submit="onPromptSubmit"
    >
      <PromptInputHeader v-if="assistant.selection || assistant.attaches.length">
        <div class="flex w-full flex-col gap-1.5">
          <div
            v-if="assistant.selection"
            class="flex items-start gap-2 rounded-md bg-muted/50 px-2 py-1.5 text-[11.5px] text-muted-foreground"
          >
            <p class="min-w-0 flex-1 line-clamp-3">{{ assistant.selection }}</p>
            <button
              type="button"
              class="shrink-0 hover:text-foreground"
              @click="clearSelectionChip"
            >
              {{ t("common.close") }}
            </button>
          </div>
          <div v-if="assistant.attaches.length" class="flex flex-wrap gap-1">
            <span
              v-for="a in assistant.attaches"
              :key="a.id"
              class="inline-flex max-w-full items-center gap-1 rounded-full border border-border/80 bg-background px-2 py-0.5 text-[11px]"
            >
              <BookOpenText class="size-3 shrink-0 text-muted-foreground" />
              <span class="truncate">{{ a.title }}</span>
              <button
                type="button"
                class="text-muted-foreground hover:text-foreground"
                :aria-label="t('ai.detachArticle')"
                @click="assistantDetach(a.id)"
              >
                <X class="size-3" />
              </button>
            </span>
          </div>
        </div>
      </PromptInputHeader>
      <PromptInputBody>
        <PromptInputTextarea
          :placeholder="
            assistant.llmConfigured
              ? t('ai.inputPlaceholder')
              : t('ai.needModel')
          "
          :disabled="composerDisabled"
        />
      </PromptInputBody>
      <PromptInputFooter>
        <PromptInputTools>
          <PromptInputActionMenu @update:open="onAttachMenuOpen">
            <PromptInputActionMenuTrigger :aria-label="t('ai.attachArticle')" />
            <PromptInputActionMenuContent
              class="w-72 overflow-hidden p-0"
              @open-auto-focus.prevent
            >
              <PromptInputCommand
                class="bg-transparent"
                @keydown="onAttachCommandKeydown"
              >
                <PromptInputCommandInput
                  :placeholder="t('ai.attachFilter')"
                  @update:model-value="onAttachFilter"
                />
                <PromptInputCommandList class="max-h-64">
                  <PromptInputCommandEmpty>
                    {{
                      attachSearching ? t("ai.working") : t("ai.attachNone")
                    }}
                  </PromptInputCommandEmpty>
                  <PromptInputCommandGroup v-if="canAttachCurrent">
                    <PromptInputCommandItem
                      value="__current__"
                      @select="attachCurrent"
                    >
                      {{ t("ai.attachCurrent") }}
                      <span
                        v-if="selectedArticle"
                        class="ml-auto max-w-[9rem] truncate text-[11px] text-muted-foreground"
                      >
                        {{ selectedArticle.title }}
                      </span>
                    </PromptInputCommandItem>
                  </PromptInputCommandGroup>
                  <PromptInputCommandGroup
                    v-if="attachCandidates.length"
                    :heading="t('ai.attachFromList')"
                  >
                    <PromptInputCommandItem
                      v-for="item in attachCandidates"
                      :key="item.id"
                      :value="`${item.title} ${item.feedTitle}`"
                      @select="assistantAttach(item)"
                    >
                      <span class="flex min-w-0 flex-col">
                        <span class="truncate">{{ item.title }}</span>
                        <span
                          v-if="item.feedTitle"
                          class="truncate text-[11px] text-muted-foreground"
                        >
                          {{ item.feedTitle }}
                        </span>
                      </span>
                    </PromptInputCommandItem>
                  </PromptInputCommandGroup>
                  <PromptInputCommandGroup
                    v-if="attachLibraryHits.length || attachSearching"
                    :heading="t('ai.attachFromLibrary')"
                  >
                    <PromptInputCommandItem
                      v-for="item in attachLibraryHits"
                      :key="item.id"
                      :value="`${item.title} ${item.feedTitle}`"
                      @select="assistantAttach(item)"
                    >
                      <span class="min-w-0 truncate">{{ item.title }}</span>
                    </PromptInputCommandItem>
                  </PromptInputCommandGroup>
                </PromptInputCommandList>
              </PromptInputCommand>
            </PromptInputActionMenuContent>
          </PromptInputActionMenu>
          <TooltipProvider :delay-duration="300">
            <Tooltip>
              <TooltipTrigger as-child>
                <PromptInputButton
                  type="button"
                  :variant="assistant.useLibrary ? 'secondary' : 'ghost'"
                  :aria-pressed="assistant.useLibrary"
                  :aria-label="t('ai.searchLibrary')"
                  :class="
                    assistant.useLibrary
                      ? 'bg-primary/15 text-primary ring-1 ring-primary/45 hover:bg-primary/20'
                      : 'text-muted-foreground'
                  "
                  @click="setAssistantUseLibrary(!assistant.useLibrary)"
                >
                  <Library
                    class="size-4"
                    :class="assistant.useLibrary && 'fill-primary/25'"
                  />
                  <span
                    v-if="assistant.useLibrary"
                    class="pr-0.5 text-[11px] font-medium"
                  >
                    {{ t("ai.searchLibraryOn") }}
                  </span>
                </PromptInputButton>
              </TooltipTrigger>
              <TooltipContent class="max-w-[220px]">
                {{ t("ai.searchLibraryHint") }}
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </PromptInputTools>
        <PromptInputSubmit
          :status="status"
          :disabled="composerDisabled && !assistant.busy"
        />
      </PromptInputFooter>
    </PromptInput>
  </section>
</template>
