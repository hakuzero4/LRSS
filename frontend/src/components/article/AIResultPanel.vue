<script setup lang="ts">
import { Check, ClipboardCopy, Sparkles, X } from "@lucide/vue";
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
  title: string;
  content: string;
  model?: string;
  cached?: boolean;
  busy?: boolean;
  folderId?: string;
  folderName?: string;
  verdict?: string;
}>();

const emit = defineEmits<{
  close: [];
  applyFolder: [folderId: string];
}>();

const { t } = useI18n();
const copying = ref(false);
const justCopied = ref(false);
let copiedTimer: ReturnType<typeof setTimeout> | null = null;

const hasContent = computed(() => props.content.trim().length > 0);
const canApplyFolder = computed(
  () => !!(props.folderId && props.folderId.trim()),
);

async function onCopy() {
  if (!hasContent.value || copying.value) return;
  copying.value = true;
  try {
    const ok = await copyTextToClipboard(props.content);
    if (ok) {
      toast.success(t("ai.copied"));
      justCopied.value = true;
      if (copiedTimer) clearTimeout(copiedTimer);
      copiedTimer = setTimeout(() => {
        justCopied.value = false;
      }, 1600);
    } else {
      toast.error(t("ai.copyFailed"));
    }
  } finally {
    copying.value = false;
  }
}
</script>

<template>
  <section
    class="flex h-full min-h-0 min-w-0 w-full flex-col overflow-hidden border-l border-border/80 bg-background"
    role="complementary"
    :aria-label="title || t('ai.panelTitle')"
  >
    <header class="pane-chrome flex h-12 shrink-0 items-center justify-between gap-2 px-3">
      <div class="flex min-w-0 flex-1 items-center gap-2">
        <Sparkles class="size-4 shrink-0 text-primary" aria-hidden="true" />
        <div class="min-w-0">
          <p class="truncate text-[12px] font-medium text-foreground">
            {{ title || t("ai.panelTitle") }}
          </p>
          <p class="truncate text-[11px] text-muted-foreground">
            <span v-if="busy">{{ t("ai.working") }}</span>
            <template v-else>
              <span v-if="model">{{ model }}</span>
              <span v-if="cached" class="ml-1 opacity-80">· {{ t("ai.cached") }}</span>
              <span v-if="verdict" class="ml-1">· {{ verdict }}</span>
            </template>
          </p>
        </div>
      </div>

      <TooltipProvider :delay-duration="300">
        <div class="flex shrink-0 items-center gap-0.5">
          <Button
            v-if="canApplyFolder"
            type="button"
            size="sm"
            variant="outline"
            class="h-7 text-[11px]"
            @click="emit('applyFolder', folderId!)"
          >
            {{ t("ai.applyFolder", { name: folderName || folderId }) }}
          </Button>
          <Tooltip>
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-muted-foreground"
                :disabled="!hasContent || copying || busy"
                :aria-label="t('ai.copy')"
                @click="onCopy"
              >
                <Check v-if="justCopied" class="size-4 text-emerald-500" />
                <ClipboardCopy v-else class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ t("ai.copy") }}</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                class="text-muted-foreground"
                :aria-label="t('ai.closePanel')"
                @click="emit('close')"
              >
                <X class="size-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>{{ t("ai.closePanel") }}</TooltipContent>
          </Tooltip>
        </div>
      </TooltipProvider>
    </header>

    <div class="scroll-pane flex-1 bg-muted/15">
      <div
        v-if="busy && !hasContent"
        class="flex h-full flex-col items-center justify-center gap-2 px-6 text-[13px] text-muted-foreground"
      >
        <Sparkles class="size-5 animate-pulse text-primary" />
        {{ t("ai.working") }}
      </div>
      <pre
        v-else-if="hasContent"
        class="m-0 min-h-full whitespace-pre-wrap break-words px-4 py-4 font-mono text-[12.5px] leading-[1.55] text-foreground/90"
      >{{ content }}</pre>
      <div
        v-else
        class="flex h-full items-center justify-center px-6 text-center text-[13px] text-muted-foreground"
      >
        {{ t("ai.empty") }}
      </div>
    </div>
  </section>
</template>
