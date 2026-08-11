<script setup lang="ts">
import { Check, ClipboardCopy, FileCode2, X } from "@lucide/vue";
import { computed, ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { copyTextToClipboard } from "@/lib/htmlToMarkdown";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const props = defineProps<{
  content: string;
  articleTitle?: string;
}>();

const emit = defineEmits<{
  close: [];
}>();

const { t } = useI18n();
const copying = ref(false);
const justCopied = ref(false);
let copiedTimer: ReturnType<typeof setTimeout> | null = null;

const hasContent = computed(() => props.content.trim().length > 0);

const headerLabel = computed(() => {
  const title = (props.articleTitle ?? "").trim();
  if (!title) return t("article.markdownPanelTitle");
  return title;
});

async function onCopy() {
  if (!hasContent.value || copying.value) return;
  copying.value = true;
  try {
    const ok = await copyTextToClipboard(props.content);
    if (ok) {
      toast.success(t("article.copyMarkdownDone"));
      justCopied.value = true;
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => {
        justCopied.value = false;
      }, 1600);
    } else {
      toast.error(t("article.copyMarkdownFailed"));
    }
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("article.copyMarkdownFailed"), { description: msg });
  } finally {
    copying.value = false;
  }
}
</script>

<template>
  <section
    class="flex h-full min-h-0 min-w-0 w-full flex-col overflow-hidden border-l border-border/80 bg-background"
    role="complementary"
    :aria-label="t('article.markdownPanelTitle')"
  >
    <header class="pane-chrome flex h-12 shrink-0 items-center justify-between gap-2 px-3">
      <div class="flex min-w-0 flex-1 items-center gap-2">
        <FileCode2 class="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        <div class="min-w-0">
          <p class="truncate text-[12px] font-medium text-foreground">
            {{ t("article.markdownPanelTitle") }}
          </p>
          <p
            v-if="articleTitle"
            class="truncate text-[11px] text-muted-foreground"
            :title="headerLabel"
          >
            {{ headerLabel }}
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
                :disabled="!hasContent || copying"
                :aria-label="t('article.copyMarkdown')"
                @click="onCopy"
              >
                <Check v-if="justCopied" class="size-4 text-emerald-500" />
                <ClipboardCopy v-else class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ t("article.copyMarkdown") }}</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-muted-foreground"
                :aria-label="t('article.closeMarkdownPanel')"
                @click="emit('close')"
              >
                <X class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ t("article.closeMarkdownPanel") }}</TooltipContent>
          </Tooltip>
        </div>
      </TooltipProvider>
    </header>

    <div class="scroll-pane flex-1 bg-muted/20">
      <pre
        v-if="hasContent"
        class="markdown-source m-0 min-h-full whitespace-pre-wrap break-words px-4 py-4 font-mono text-[12.5px] leading-[1.55] text-foreground/90"
      >{{ content }}</pre>
      <div
        v-else
        class="flex h-full items-center justify-center px-6 text-center text-[13px] text-muted-foreground"
      >
        {{ t("article.markdownEmpty") }}
      </div>
    </div>
  </section>
</template>
