<script setup lang="ts">
import { Search, X } from "@lucide/vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";

const { t } = useI18n();
const { searchQuery, setSearchQuery } = useRssStore();

const searchModel = computed({
  get: () => searchQuery.value,
  set: (v: string) => setSearchQuery(v),
});
</script>

<template>
  <div class="article-search" role="search">
    <div class="article-search-field">
      <Search class="article-search-icon" aria-hidden="true" />
      <input
        v-model="searchModel"
        type="text"
        enterkeyhint="search"
        autocomplete="off"
        spellcheck="false"
        :placeholder="t('article.searchPlaceholder')"
        class="article-search-input"
        :aria-label="t('article.searchAria')"
      />
      <button
        v-if="searchQuery"
        type="button"
        class="article-search-clear"
        :aria-label="t('article.clearSearch')"
        @click="setSearchQuery('')"
      >
        <X class="size-3.5" stroke-width="2" />
      </button>
    </div>
  </div>
</template>
