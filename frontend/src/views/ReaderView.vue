<script setup lang="ts">
import { ArrowLeft } from "@lucide/vue";
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import ArticleList from "@/components/article/ArticleList.vue";
import ArticleReader from "@/components/article/ArticleReader.vue";
import MarkdownPanel from "@/components/article/MarkdownPanel.vue";
import BriefingList from "@/components/briefing/BriefingList.vue";
import BriefingReader from "@/components/briefing/BriefingReader.vue";
import { useRssStore } from "@/composables/useRssStore";
import { articleToMarkdown } from "@/lib/htmlToMarkdown";
import { Button } from "@/components/ui/button";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";

const { t } = useI18n();
const {
  selectedArticle,
  selectedFeed,
  selectedArticleId,
  selectedBriefingId,
  collectionId,
  collectionDisplayMode,
  zenMode,
  selectArticle,
} = useRssStore();

const briefingActive = computed(
  () =>
    !!selectedBriefingId.value &&
    (collectionId.value === "briefing" || collectionId.value === "starred"),
);
const briefingShowsArticle = computed(
  () => briefingActive.value && !!selectedArticleId.value,
);

function backToBriefing() {
  selectArticle(null);
}

const markdownOpen = ref(false);

const markdownContent = computed(() => {
  const a = selectedArticle.value;
  if (!a || !markdownOpen.value) return "";
  return articleToMarkdown({
    title: a.title,
    author: a.author,
    feedTitle: selectedFeed.value?.title,
    publishedAt: a.publishedAt,
    url: a.url,
    summary: a.summary,
    contentHtml: a.contentHtml,
  });
});

const sidePanelOpen = computed(() => markdownOpen.value);

function closeMarkdownPanel() {
  markdownOpen.value = false;
}

// Close AI/md panels when selection is cleared.
watch(selectedArticleId, (id) => {
  if (!id) {
    markdownOpen.value = false;
  }
});
</script>

<template>
  <!-- Zen mode: reader (+ optional side panels) only -->
  <div
    v-if="zenMode"
    class="flex h-full min-h-0 w-full flex-1 overflow-hidden"
  >
    <div class="flex min-h-0 min-w-0 flex-1 flex-col">
      <div
        v-if="briefingShowsArticle"
        class="pane-chrome flex h-10 shrink-0 items-center px-3"
      >
        <Button
          type="button"
          variant="ghost"
          size="sm"
          class="h-8 gap-1.5 px-2 text-[12.5px] text-muted-foreground"
          @click="backToBriefing"
        >
          <ArrowLeft class="size-3.5" />
          {{ t("briefing.backToBriefing") }}
        </Button>
      </div>
      <div class="min-h-0 min-w-0 flex-1">
        <ArticleReader v-model:markdown-open="markdownOpen" />
      </div>
    </div>
    <div
      v-if="markdownOpen"
      class="min-h-0 w-[min(40%,28rem)] min-w-[16rem] max-w-[48%] shrink-0"
    >
      <MarkdownPanel
        :content="markdownContent"
        :article-title="selectedArticle?.title"
        @close="closeMarkdownPanel"
      />
    </div>
  </div>

  <ResizablePanelGroup
    v-else
    id="lrss-reader"
    direction="horizontal"
    auto-save-id="lrss-reader"
    class="h-full min-h-0 w-full flex-1 overflow-hidden"
  >
    <ResizablePanel
      id="article-list"
      :default-size="collectionDisplayMode === 'cards' ? (sidePanelOpen ? 36 : 50) : sidePanelOpen ? 24 : 34"
      :min-size="collectionDisplayMode === 'cards' ? 28 : 18"
      :max-size="collectionDisplayMode === 'cards' ? 70 : 50"
      class="h-full min-h-0 min-w-0 overflow-hidden"
    >
      <BriefingList v-if="collectionId === 'briefing'" />
      <ArticleList v-else />
    </ResizablePanel>

    <ResizableHandle with-handle />

    <ResizablePanel
      id="article-reader"
      :default-size="sidePanelOpen ? 42 : 66"
      :min-size="28"
      class="min-w-0"
    >
      <div
        v-if="briefingShowsArticle"
        class="flex h-full min-h-0 flex-col"
      >
        <div class="pane-chrome flex h-10 shrink-0 items-center px-3">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            class="h-8 gap-1.5 px-2 text-[12.5px] text-muted-foreground"
            @click="backToBriefing"
          >
            <ArrowLeft class="size-3.5" />
            {{ t("briefing.backToBriefing") }}
          </Button>
        </div>
        <div class="min-h-0 flex-1">
          <ArticleReader v-model:markdown-open="markdownOpen" />
        </div>
      </div>
      <BriefingReader v-else-if="briefingActive" />
      <ArticleReader v-else v-model:markdown-open="markdownOpen" />
    </ResizablePanel>

    <template v-if="markdownOpen">
      <ResizableHandle with-handle />
      <ResizablePanel
        id="article-markdown"
        :default-size="30"
        :min-size="18"
        :max-size="48"
        class="min-w-0"
      >
        <MarkdownPanel
          :content="markdownContent"
          :article-title="selectedArticle?.title"
          @close="closeMarkdownPanel"
        />
      </ResizablePanel>
    </template>
  </ResizablePanelGroup>
</template>
