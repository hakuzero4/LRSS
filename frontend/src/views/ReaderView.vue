<script setup lang="ts">
import ArticleList from "@/components/article/ArticleList.vue";
import ArticleReader from "@/components/article/ArticleReader.vue";
</script>

<template>
  <div class="flex h-full min-h-0 flex-1 overflow-hidden">
    <ArticleList />
    <ArticleReader />
  </div>
</template>
