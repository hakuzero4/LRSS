<script setup lang="ts">
import ArticleList from "@/components/article/ArticleList.vue";
import ArticleReader from "@/components/article/ArticleReader.vue";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";
</script>

<template>
  <ResizablePanelGroup
    id="lrss-reader"
    direction="horizontal"
    auto-save-id="lrss-reader"
    class="h-full min-h-0 w-full flex-1 overflow-hidden"
  >
    <ResizablePanel
      id="article-list"
      :default-size="34"
      :min-size="20"
      :max-size="55"
      class="min-w-0"
    >
      <ArticleList />
    </ResizablePanel>

    <ResizableHandle with-handle />

    <ResizablePanel id="article-reader" :default-size="66" :min-size="30" class="min-w-0">
      <ArticleReader />
    </ResizablePanel>
  </ResizablePanelGroup>
</template>
