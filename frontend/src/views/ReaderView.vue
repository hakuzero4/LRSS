<script setup lang="ts">
import { computed, ref, watch } from "vue";
import ArticleList from "@/components/article/ArticleList.vue";
import ArticleReader from "@/components/article/ArticleReader.vue";
import MarkdownPanel from "@/components/article/MarkdownPanel.vue";
import { useRssStore } from "@/composables/useRssStore";
import { articleToMarkdown } from "@/lib/htmlToMarkdown";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";

const { selectedArticle, selectedFeed, selectedArticleId, zenMode } = useRssStore();

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

function closeMarkdownPanel() {
  markdownOpen.value = false;
}

// Close panel when selection is cleared; refresh content when switching articles while open.
watch(selectedArticleId, (id) => {
  if (!id) markdownOpen.value = false;
});
</script>

<template>
  <!-- Zen mode: reader (+ optional markdown) only — no article list -->
  <div
    v-if="zenMode"
    class="flex h-full min-h-0 w-full flex-1 overflow-hidden"
  >
    <div class="min-h-0 min-w-0 flex-1">
      <ArticleReader v-model:markdown-open="markdownOpen" />
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
      :default-size="markdownOpen ? 26 : 34"
      :min-size="18"
      :max-size="50"
      class="min-w-0"
    >
      <ArticleList />
    </ResizablePanel>

    <ResizableHandle with-handle />

    <ResizablePanel
      id="article-reader"
      :default-size="markdownOpen ? 44 : 66"
      :min-size="28"
      class="min-w-0"
    >
      <ArticleReader v-model:markdown-open="markdownOpen" />
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
