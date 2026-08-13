<script setup lang="ts">
import { AlertCircle, LoaderCircle, RefreshCw, Star, Trash2 } from "@lucide/vue";
import { computed, nextTick, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute, relativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const { t } = useI18n();

const {
  briefings,
  briefingsLoading,
  selectedBriefingId,
  collectionTitle,
  refreshing,
  webMode,
  selectBriefing,
  setBriefingStarred,
  deleteBriefing,
  refreshFeeds,
} = useRssStore();

const deleteOpen = ref(false);
const deleteTargetId = ref<string | null>(null);
const deleteBusy = ref(false);

function openDelete(id: string) {
  deleteTargetId.value = id;
  deleteOpen.value = true;
}

async function confirmDelete() {
  const id = deleteTargetId.value;
  if (!id || deleteBusy.value) return;
  deleteBusy.value = true;
  try {
    await deleteBriefing(id);
    deleteOpen.value = false;
    deleteTargetId.value = null;
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("briefing.deleteFailed"), { description: msg });
  } finally {
    deleteBusy.value = false;
  }
}

const listEl = ref<HTMLElement | null>(null);
const empty = computed(() => briefings.value.length === 0);

const countLabel = computed(() => {
  if (briefingsLoading.value && empty.value) return t("common.loading");
  const n = briefings.value.length;
  return n === 1 ? t("briefing.countOne") : t("briefing.count", { n });
});

function statusLabel(status: string): string {
  if (status === "pending") return t("briefing.statusPending");
  if (status === "error") return t("briefing.statusError");
  return t("briefing.statusReady");
}

function teaser(overview: string, status: string, error?: string): string {
  if (status === "pending") return t("briefing.pending");
  if (status === "error") return error || t("briefing.error");
  return overview;
}

async function onToggleStar(id: string, starred: boolean) {
  try {
    await setBriefingStarred(id, starred);
  } catch {
    /* store logs */
  }
}

watch(selectedBriefingId, async (id) => {
  if (!id || !listEl.value) return;
  await nextTick();
  const row = listEl.value.querySelector(`[data-briefing-id="${CSS.escape(id)}"]`);
  row?.scrollIntoView({ block: "nearest" });
});
</script>

<template>
  <section class="flex h-full min-h-0 w-full min-w-0 flex-col overflow-hidden bg-background">
    <header class="pane-chrome flex h-12 items-center gap-2 px-3">
      <div class="min-w-0 flex-1">
        <h2 class="truncate text-[13px] font-semibold tracking-tight">
          {{ collectionTitle }}
        </h2>
        <p class="text-[11px] text-muted-foreground tabular-nums">
          {{ countLabel }}
        </p>
      </div>

      <TooltipProvider :delay-duration="300">
        <Tooltip v-if="!webMode">
          <TooltipTrigger as-child>
            <Button
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground"
              :disabled="refreshing"
              :aria-label="t('article.refreshFeeds')"
              @click="refreshFeeds"
            >
              <RefreshCw class="size-4" :class="refreshing && 'animate-spin'" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{{ t("article.refreshFeeds") }}</TooltipContent>
        </Tooltip>
      </TooltipProvider>
    </header>

    <div ref="listEl" class="scroll-pane flex-1">
      <div v-if="empty" class="flex h-48 flex-col items-center justify-center px-6 text-center">
        <p class="text-[13px] font-medium text-foreground/80">{{ t("briefing.empty") }}</p>
        <p class="mt-1 text-[12px] leading-relaxed text-muted-foreground">
          {{ briefingsLoading ? t("common.loading") : t("briefing.emptyHint") }}
        </p>
      </div>

      <ContextMenu v-for="item in briefings" :key="item.id">
      <ContextMenuTrigger as-child>
      <button
        type="button"
        :data-briefing-id="item.id"
        :class="
          cn(
            'article-row group w-full border-b border-border/50 px-3 py-3 text-left transition-colors duration-150',
            'hover:bg-black/[0.03] dark:hover:bg-white/[0.04]',
            'active:scale-[0.995] active:bg-black/[0.05] dark:active:bg-white/[0.06]',
            item.id === selectedBriefingId && 'bg-primary/10 dark:bg-primary/15',
            !item.isRead && 'is-unread',
          )
        "
        @click="selectBriefing(item.id)"
      >
        <div class="flex items-start gap-2.5">
          <span
            class="mt-1.5 size-1.5 shrink-0 rounded-full"
            :class="item.isRead ? 'bg-transparent' : 'unread-dot'"
            aria-hidden="true"
          />
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <p class="min-w-0 flex-1 truncate text-[11px] text-muted-foreground">
                {{ relativeTime(item.createdAt) }}
                <span class="mx-1">·</span>
                {{ statusLabel(item.status) }}
                <span v-if="item.articleCount > 0">
                  ·
                  {{
                    item.articleCount === 1
                      ? t("briefing.articleCountOne")
                      : t("briefing.articleCount", { n: item.articleCount })
                  }}
                </span>
              </p>
              <LoaderCircle
                v-if="item.status === 'pending'"
                class="size-3 shrink-0 animate-spin text-muted-foreground"
              />
              <AlertCircle
                v-else-if="item.status === 'error'"
                class="size-3 shrink-0 text-destructive"
              />
              <button
                type="button"
                class="rounded p-0.5 text-muted-foreground hover:text-amber-500"
                :aria-label="item.isStarred ? t('briefing.unstar') : t('briefing.star')"
                @click.stop="onToggleStar(item.id, !item.isStarred)"
              >
                <Star
                  class="size-3.5"
                  :class="item.isStarred && 'fill-amber-500 text-amber-500'"
                />
              </button>
            </div>
            <h3
              class="mt-1 line-clamp-2 text-[13.5px] leading-snug tracking-[-0.01em]"
              :class="
                item.isRead ? 'font-medium text-foreground/80' : 'font-semibold text-foreground'
              "
            >
              {{ formatAbsolute(item.createdAt) }}
            </h3>
            <p
              v-if="teaser(item.overview, item.status, item.error)"
              class="article-teaser"
              :class="item.status === 'error' && 'text-destructive'"
            >
              {{ teaser(item.overview, item.status, item.error) }}
            </p>
          </div>
        </div>
      </button>
      </ContextMenuTrigger>
      <ContextMenuContent class="w-44">
        <ContextMenuItem variant="destructive" @select="openDelete(item.id)">
          <Trash2 class="size-3.5" />
          {{ t("briefing.delete") }}
        </ContextMenuItem>
      </ContextMenuContent>
      </ContextMenu>
    </div>

    <AlertDialog v-model:open="deleteOpen">
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{{ t("briefing.deleteTitle") }}</AlertDialogTitle>
          <AlertDialogDescription>
            {{ t("briefing.deleteDesc") }}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel size="sm" :disabled="deleteBusy">
            {{ t("common.cancel") }}
          </AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            size="sm"
            :disabled="deleteBusy"
            @click="confirmDelete"
          >
            {{ deleteBusy ? t("common.loading") : t("briefing.deleteConfirm") }}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  </section>
</template>
