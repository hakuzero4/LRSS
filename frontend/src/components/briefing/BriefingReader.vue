<script setup lang="ts">
import { AlertCircle, LoaderCircle, Newspaper, Star } from "@lucide/vue";
import { computed, ref } from "vue";
import { toast } from "vue-sonner";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute } from "@/lib/format";
import {
  readerFontFamilyCSS,
  readerShellClasses,
} from "@/lib/readingSettings";
import { cn } from "@/lib/utils";
import type { BriefingBullet, BriefingCite } from "@/types/rss";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const { t } = useI18n();

const {
  selectedBriefing,
  settings,
  selectArticle,
  setBriefingStarred,
  retryBriefing,
} = useRssStore();

const retrying = ref(false);

async function onRetry() {
  const id = selectedBriefing.value?.id;
  if (!id || retrying.value) return;
  retrying.value = true;
  try {
    await retryBriefing(id);
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("briefing.retryFailed"), { description: msg });
  } finally {
    retrying.value = false;
  }
}

const readerShellClass = computed(() => {
  const { className } = readerShellClasses(settings.fontSize, settings.readerWidth);
  return className;
});

const readerShellStyle = computed(() => {
  const fam = readerFontFamilyCSS(settings.readerFontFamily);
  if (!fam) return undefined;
  return { "--reader-font-family": fam } as Record<string, string>;
});

const overview = computed(() => {
  const b = selectedBriefing.value;
  if (!b) return "";
  return b.overview || b.payload.overview || "";
});

const themes = computed(() => selectedBriefing.value?.payload.themes ?? []);
const watchItems = computed(() => selectedBriefing.value?.payload.watch ?? []);

function statusLabel(status: string): string {
  if (status === "pending") return t("briefing.statusPending");
  if (status === "error") return t("briefing.statusError");
  return t("briefing.statusReady");
}

async function onToggleStar() {
  const b = selectedBriefing.value;
  if (!b) return;
  try {
    await setBriefingStarred(b.id, !b.isStarred);
  } catch {
    /* store logs */
  }
}

function citesOf(bullet: BriefingBullet): BriefingCite[] {
  if (bullet.cites && bullet.cites.length > 0) return bullet.cites;
  if (!bullet.articleId) return [];
  return [{ articleId: bullet.articleId, title: bullet.title, feedTitle: bullet.feedTitle }];
}

function openCite(cite: BriefingCite) {
  if (!cite.articleId) return;
  void selectArticle(cite.articleId);
}
</script>

<template>
  <section class="flex h-full min-h-0 min-w-0 w-full flex-col overflow-hidden bg-background">
    <template v-if="selectedBriefing">
      <header class="pane-chrome flex h-12 shrink-0 items-center justify-between gap-2 px-4">
        <div class="min-w-0 flex-1 pr-2">
          <p class="truncate text-[12px] text-muted-foreground">
            {{ statusLabel(selectedBriefing.status) }}
            <span v-if="selectedBriefing.model">
              · {{ t("briefing.model") }} {{ selectedBriefing.model }}
            </span>
          </p>
        </div>
        <TooltipProvider :delay-duration="300">
          <Tooltip>
            <TooltipTrigger as-child>
              <Button
                variant="ghost"
                size="icon-sm"
                :class="selectedBriefing.isStarred ? 'text-amber-500' : 'text-muted-foreground'"
                :aria-label="
                  selectedBriefing.isStarred ? t('briefing.unstar') : t('briefing.star')
                "
                @click="onToggleStar"
              >
                <Star
                  class="size-4"
                  :class="selectedBriefing.isStarred && 'fill-current'"
                />
              </Button>
            </TooltipTrigger>
            <TooltipContent>
              {{ selectedBriefing.isStarred ? t("briefing.unstar") : t("briefing.star") }}
            </TooltipContent>
          </Tooltip>
        </TooltipProvider>
      </header>

      <div class="scroll-pane flex-1">
        <article
          :class="cn('reader-shell mx-auto px-6 py-8 sm:px-10 sm:py-10', readerShellClass)"
          :style="readerShellStyle"
        >
          <header class="reader-header-block">
            <p class="text-[12px] font-medium tracking-[0.01em] text-muted-foreground">
              {{ formatAbsolute(selectedBriefing.createdAt) }}
            </p>
            <h1 class="reader-title mt-2 font-semibold tracking-tight">
              {{ t("nav.briefing") }}
            </h1>
            <p
              v-if="selectedBriefing.articleCount > 0"
              class="mt-2 text-[12.5px] text-muted-foreground"
            >
              {{
                selectedBriefing.articleCount === 1
                  ? t("briefing.articleCountOne")
                  : t("briefing.articleCount", { n: selectedBriefing.articleCount })
              }}
            </p>
          </header>

          <div
            v-if="selectedBriefing.status === 'pending'"
            class="mt-8 flex items-start gap-2.5 text-[14px] text-muted-foreground"
          >
            <LoaderCircle class="mt-0.5 size-4 shrink-0 animate-spin" />
            <div>
              <p class="font-medium text-foreground/80">{{ t("briefing.pending") }}</p>
              <p class="mt-1 text-[13px] leading-relaxed">{{ t("briefing.pendingHint") }}</p>
            </div>
          </div>

          <div
            v-else-if="selectedBriefing.status === 'error'"
            class="mt-8 flex items-start gap-2.5 text-[14px]"
          >
            <AlertCircle class="mt-0.5 size-4 shrink-0 text-destructive" />
            <div>
              <p class="font-medium text-destructive">{{ t("briefing.error") }}</p>
              <p
                v-if="selectedBriefing.error"
                class="mt-1 text-[13px] leading-relaxed text-muted-foreground"
              >
                {{ selectedBriefing.error }}
              </p>
              <Button
                type="button"
                variant="outline"
                size="sm"
                class="mt-3 h-8"
                :disabled="retrying"
                @click="onRetry"
              >
                {{ retrying ? t("briefing.retrying") : t("briefing.retry") }}
              </Button>
            </div>
          </div>

          <template v-else>
            <aside v-if="overview" class="reader-summary mt-6">
              <div class="reader-summary-inner">
                <p class="reader-summary-label">
                  <span class="reader-summary-label-dot" aria-hidden="true" />
                  {{ t("briefing.overview") }}
                </p>
                <p class="reader-summary-text whitespace-pre-wrap">{{ overview }}</p>
              </div>
            </aside>

            <section v-if="themes.length" class="mt-8" :aria-label="t('briefing.themes')">
              <div
                v-for="(theme, i) in themes"
                :key="`${theme.title}-${i}`"
                class="mt-6 first:mt-0"
              >
                <h2 class="text-[15px] font-semibold tracking-tight">
                  {{ theme.title || t("briefing.themes") }}
                </h2>
                <ul class="mt-2 space-y-3">
                  <li v-for="(bullet, bi) in theme.bullets" :key="bi + bullet.point">
                    <p class="text-[14px] leading-relaxed text-foreground/90">
                      {{ bullet.point || bullet.title }}
                    </p>
                    <div class="mt-1.5 flex flex-wrap gap-1.5">
                      <button
                        v-for="cite in citesOf(bullet)"
                        :key="cite.articleId"
                        type="button"
                        class="max-w-full truncate rounded-md bg-muted/70 px-2 py-0.5 text-left text-[11.5px] text-muted-foreground transition-colors hover:bg-primary/15 hover:text-foreground"
                        @click="openCite(cite)"
                      >
                        <span class="font-medium text-foreground/75">{{ cite.title }}</span>
                        <span v-if="cite.feedTitle"> · {{ cite.feedTitle }}</span>
                      </button>
                    </div>
                  </li>
                </ul>
              </div>
            </section>

            <section
              v-if="watchItems.length"
              class="mt-10 border-t border-border/60 pt-6"
              :aria-label="t('briefing.watch')"
            >
              <h2 class="text-[15px] font-semibold tracking-tight">
                {{ t("briefing.watch") }}
              </h2>
              <ul class="mt-2 space-y-3">
                <li v-for="(bullet, wi) in watchItems" :key="'w-' + wi + bullet.point">
                  <p class="text-[14px] leading-relaxed text-foreground/90">
                    {{ bullet.point || bullet.title }}
                  </p>
                  <div class="mt-1.5 flex flex-wrap gap-1.5">
                    <button
                      v-for="cite in citesOf(bullet)"
                      :key="'w-' + cite.articleId"
                      type="button"
                      class="max-w-full truncate rounded-md bg-muted/70 px-2 py-0.5 text-left text-[11.5px] text-muted-foreground transition-colors hover:bg-primary/15 hover:text-foreground"
                      @click="openCite(cite)"
                    >
                      <span class="font-medium text-foreground/75">{{ cite.title }}</span>
                      <span v-if="cite.feedTitle"> · {{ cite.feedTitle }}</span>
                    </button>
                  </div>
                </li>
              </ul>
            </section>

            <p
              v-if="selectedBriefing.omittedCount > 0"
              class="mt-8 text-[12px] text-muted-foreground"
            >
              {{
                selectedBriefing.omittedCount === 1
                  ? t("briefing.omittedOne")
                  : t("briefing.omitted", { n: selectedBriefing.omittedCount })
              }}
            </p>
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
        <Newspaper class="size-6 text-muted-foreground" />
      </div>
      <h2 class="text-[15px] font-semibold tracking-tight">{{ t("briefing.selectTitle") }}</h2>
      <p class="mt-1.5 max-w-xs text-[13px] leading-relaxed text-muted-foreground">
        {{ t("briefing.selectHint") }}
      </p>
    </div>
  </section>
</template>
