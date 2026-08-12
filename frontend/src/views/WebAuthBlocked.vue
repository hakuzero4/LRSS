<script setup lang="ts">
import { useI18n } from "vue-i18n";
import { clearWebToken } from "@/lib/webMode";

const { t } = useI18n();

function onClearAndReload() {
  clearWebToken();
  // Hard reload so loadAppsvc re-probes with an empty token (still unauthorized
  // unless the user pastes a fresh ?token= URL).
  window.location.href = window.location.pathname || "/";
}
</script>

<template>
  <div
    class="web-auth-blocked flex h-full min-h-screen w-full items-center justify-center bg-background px-6 py-10 text-foreground"
    role="alert"
    aria-live="polite"
  >
    <div
      class="web-auth-card w-full max-w-md rounded-2xl border border-border bg-card px-7 py-8 shadow-sm"
    >
      <div class="flex items-start gap-4">
        <div
          class="flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-xl bg-muted"
          aria-hidden="true"
        >
          <img
            src="/appicon.png"
            alt=""
            width="48"
            height="48"
            class="size-full object-contain"
            draggable="false"
          />
        </div>
        <div class="min-w-0 pt-0.5">
          <p class="text-[12px] font-medium tracking-wide text-muted-foreground uppercase">
            LRSS
          </p>
          <h1 class="mt-1 text-[18px] font-semibold tracking-tight">
            {{ t("webAuth.title") }}
          </h1>
        </div>
      </div>

      <p class="mt-5 text-[13.5px] leading-relaxed text-muted-foreground">
        {{ t("webAuth.body") }}
      </p>
      <p class="mt-3 text-[12.5px] leading-relaxed text-muted-foreground">
        {{ t("webAuth.hint") }}
      </p>

      <div class="mt-7 flex flex-wrap gap-2">
        <button
          type="button"
          class="inline-flex h-9 items-center justify-center rounded-lg bg-primary px-4 text-[13px] font-medium text-primary-foreground transition-opacity hover:opacity-90"
          @click="onClearAndReload"
        >
          {{ t("webAuth.reload") }}
        </button>
      </div>
    </div>
  </div>
</template>
