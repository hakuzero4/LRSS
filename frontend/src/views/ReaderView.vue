<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import ArticleList from "@/components/article/ArticleList.vue";
import ArticleReader from "@/components/article/ArticleReader.vue";
import AIResultPanel from "@/components/article/AIResultPanel.vue";
import MarkdownPanel from "@/components/article/MarkdownPanel.vue";
import { useRssStore } from "@/composables/useRssStore";
import { articleToMarkdown } from "@/lib/htmlToMarkdown";
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
  zenMode,
  aiPanel,
  closeAIPanel,
  aiApplyFolder,
} = useRssStore();

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

const sidePanelOpen = computed(() => markdownOpen.value || aiPanel.open);

function closeMarkdownPanel() {
  markdownOpen.value = false;
}

async function onApplyFolder(folderId: string) {
  const id = selectedArticleId.value;
  if (!id || !folderId) return;
  try {
    await aiApplyFolder(id, folderId);
    toast.success(t("ai.folderApplied"));
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("ai.folderApplyFailed"), { description: msg });
  }
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
    <div class="min-h-0 min-w-0 flex-1">
      <ArticleReader v-model:markdown-open="markdownOpen" />
    </div>
    <div
      v-if="aiPanel.open"
      class="min-h-0 w-[min(42%,30rem)] min-w-[16rem] max-w-[50%] shrink-0"
    >
      <AIResultPanel
        :title="aiPanel.title"
        :content="aiPanel.markdown"
        :model="aiPanel.model"
        :cached="aiPanel.cached"
        :busy="aiPanel.busy"
        :folder-id="aiPanel.folderId"
        :folder-name="aiPanel.folderName"
        :verdict="aiPanel.verdict"
        @close="closeAIPanel"
        @apply-folder="onApplyFolder"
      />
    </div>
    <div
      v-else-if="markdownOpen"
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
      :default-size="sidePanelOpen ? 24 : 34"
      :min-size="18"
      :max-size="50"
      class="min-w-0"
    >
      <ArticleList />
    </ResizablePanel>

    <ResizableHandle with-handle />

    <ResizablePanel
      id="article-reader"
      :default-size="sidePanelOpen ? 42 : 66"
      :min-size="28"
      class="min-w-0"
    >
      <ArticleReader v-model:markdown-open="markdownOpen" />
    </ResizablePanel>

    <template v-if="aiPanel.open">
      <ResizableHandle with-handle />
      <ResizablePanel
        id="article-ai"
        :default-size="34"
        :min-size="18"
        :max-size="48"
        class="min-w-0"
      >
        <AIResultPanel
          :title="aiPanel.title"
          :content="aiPanel.markdown"
          :model="aiPanel.model"
          :cached="aiPanel.cached"
          :busy="aiPanel.busy"
          :folder-id="aiPanel.folderId"
          :folder-name="aiPanel.folderName"
          :verdict="aiPanel.verdict"
          @close="closeAIPanel"
          @apply-folder="onApplyFolder"
        />
      </ResizablePanel>
    </template>
    <template v-else-if="markdownOpen">
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
