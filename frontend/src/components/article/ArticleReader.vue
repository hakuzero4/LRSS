<script setup lang="ts">
import {
  BookOpenText,
  Check,
  ExternalLink,
  Star,
} from "@lucide/vue";
import { computed } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import { formatAbsolute } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";

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

function openOriginal() {
  if (!selectedArticle.value) return;
  // Wails will wire this later; design uses a normal link affordance.
  window.open(selectedArticle.value.url, "_blank", "noopener,noreferrer");
}
</script>

<template>
  <section class="flex min-w-0 flex-1 flex-col bg-background">
    <template v-if="selectedArticle">
      <header class="pane-chrome flex h-12 items-center justify-between gap-2 px-4">
        <div class="min-w-0">
          <p class="truncate text-[12px] text-muted-foreground">
            {{ selectedFeed?.title ?? "Feed" }}
            <span v-if="selectedArticle.author"> · {{ selectedArticle.author }}</span>
          </p>
        </div>

        <TooltipProvider :delay-duration="300">
          <div class="flex items-center gap-0.5">
            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  :class="selectedArticle.starred ? 'text-amber-500' : 'text-muted-foreground'"
                  :aria-label="selectedArticle.starred ? 'Unstar' : 'Star'"
                  @click="toggleStar(selectedArticle.id)"
                >
                  <Star
                    class="size-4"
                    :class="selectedArticle.starred && 'fill-current'"
                  />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ selectedArticle.starred ? "Unstar" : "Star" }}
              </TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="text-muted-foreground"
                  :aria-label="selectedArticle.read ? 'Mark unread' : 'Mark read'"
                  @click="toggleRead(selectedArticle.id)"
                >
                  <Check class="size-4" :class="!selectedArticle.read && 'opacity-40'" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {{ selectedArticle.read ? "Mark unread" : "Mark read" }}
              </TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger as-child>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  class="text-muted-foreground"
                  aria-label="Open original"
                  @click="openOriginal"
                >
                  <ExternalLink class="size-4" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>Open original</TooltipContent>
            </Tooltip>
          </div>
        </TooltipProvider>
      </header>

      <ScrollArea class="flex-1">
        <article class="mx-auto max-w-[42rem] px-6 py-8 sm:px-10 sm:py-10">
          <p class="text-[12px] font-medium tracking-[0.01em] text-muted-foreground">
            {{ formatAbsolute(selectedArticle.publishedAt) }}
          </p>
          <h1
            class="mt-2 text-[1.75rem] font-semibold tracking-[-0.025em] text-balance leading-[1.15] sm:text-[2rem]"
          >
            {{ selectedArticle.title }}
          </h1>
          <p
            v-if="selectedArticle.summary"
            class="mt-3 text-[15px] leading-relaxed text-muted-foreground"
          >
            {{ selectedArticle.summary }}
          </p>

          <div
            :class="cn('reader-body mt-8 text-foreground/90', fontClass)"
            v-html="selectedArticle.contentHtml"
          />
        </article>
      </ScrollArea>
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
      <h2 class="text-[15px] font-semibold tracking-tight">Select an article</h2>
      <p class="mt-1.5 max-w-xs text-[13px] leading-relaxed text-muted-foreground">
        Choose something from the list to read. Your place and star state stay local to this
        library.
      </p>
    </div>
  </section>
</template>
