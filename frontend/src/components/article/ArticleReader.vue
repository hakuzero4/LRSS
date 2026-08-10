<script setup lang="ts">
import {
  BookOpenText,
  Check,
  ExternalLink,
  Star,
} from "@lucide/vue";
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute, plainText } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const { t } = useI18n();

const {
  selectedArticle,
  selectedFeed,
  settings,
  toggleStar,
  toggleRead,
} = useRssStore();

const fontClass = computed(() => {
  if (settings.fontSize === "sm") return "text-[15px] leading-[1.65]";
  if (settings.fontSize === "lg") return "text-[18px] leading-[1.7]";
  return "text-[16.5px] leading-[1.7]";
});

/**
 * Deck / standfirst under the title.
 * Hidden only when it is essentially the same as the full body text.
 */
const readerSummary = computed(() => {
  const a = selectedArticle.value;
  if (!a?.summary) return "";
  const s = plainText(a.summary, 480);
  if (!s || s.length < 12) return "";
  const body = plainText(a.contentHtml, 800);
  if (!body) return s;
  // Exact or near-full duplicate of the article body → no deck.
  if (s === body) return "";
  if (body.startsWith(s) && s.length > body.length * 0.72) return "";
  if (s.startsWith(body) && body.length > s.length * 0.72) return "";
  return s;
});

const hasBody = computed(() => {
  const html = selectedArticle.value?.contentHtml?.trim() ?? "";
  return html.length > 0 && plainText(html).length > 0;
});

const readerWidthClass = computed(() => {
  if (settings.readerWidth === "narrow") return "max-w-[34rem]";
  if (settings.readerWidth === "wide") return "max-w-[52rem]";
  return "max-w-[42rem]";
});

function openOriginal() {
  if (!selectedArticle.value) return;
  window.open(selectedArticle.value.url, "_blank", "noopener,noreferrer");
}
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
            <Tooltip>
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

            <Tooltip>
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

            <Tooltip>
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

      <div class="scroll-pane flex-1">
        <article :class="cn('mx-auto px-6 py-8 sm:px-10 sm:py-10', readerWidthClass)">
          <header class="reader-header-block">
            <p class="text-[12px] font-medium tracking-[0.01em] text-muted-foreground">
              {{ formatAbsolute(selectedArticle.publishedAt) }}
            </p>
            <h1
              class="mt-2 text-[1.75rem] font-semibold tracking-[-0.025em] text-balance leading-[1.2] sm:text-[2rem]"
            >
              {{ selectedArticle.title }}
            </h1>

            <!-- Summary deck: visually separate from body -->
            <aside
              v-if="readerSummary"
              class="reader-summary"
              :aria-label="t('article.summaryLabel')"
            >
              <div class="reader-summary-inner">
                <p class="reader-summary-label">
                  <span class="reader-summary-label-dot" aria-hidden="true" />
                  {{ t("article.summaryLabel") }}
                </p>
                <p class="reader-summary-text">
                  {{ readerSummary }}
                </p>
              </div>
            </aside>
          </header>

          <template v-if="hasBody">
            <div
              v-if="readerSummary"
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
                  fontClass,
                  readerSummary ? 'mt-4' : 'mt-8',
                )
              "
              v-html="selectedArticle.contentHtml"
            />
          </template>
        </article>
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
  </section>
</template>
