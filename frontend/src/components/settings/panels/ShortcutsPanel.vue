<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";

const { t } = useI18n();
const { settings, persistUIPrefs, zenMode, toggleZenMode } = useRssStore();

const shortcuts = computed(() => [
  { keys: "j / k", action: t("settings.shortcuts.nextPrev") },
  { keys: "o / Enter", action: t("settings.shortcuts.openFocus") },
  { keys: "r", action: t("settings.shortcuts.refresh") },
  { keys: "s", action: t("settings.shortcuts.toggleStar") },
  { keys: "m", action: t("settings.shortcuts.toggleRead") },
  { keys: "z", action: t("settings.shortcuts.zenMode"), interactive: true as const },
  { keys: "Esc", action: t("settings.shortcuts.escapeZen"), zenOnly: true as const },
  { keys: "⌘ ,", action: t("settings.shortcuts.openSettings") },
]);

function onEnableChange(v: boolean) {
  settings.enableKeyboardShortcuts = v;
  persistUIPrefs();
}

function onZenClick() {
  toggleZenMode();
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.shortcuts.group')"
      :description="t('settings.shortcuts.groupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.shortcuts.enable')"
          :description="t('settings.shortcuts.enableDesc')"
        >
          <Switch
            :checked="settings.enableKeyboardShortcuts"
            @update:checked="onEnableChange"
          />
        </SettingsRow>
      </div>

      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.shortcuts.zenMode')"
          :description="t('settings.shortcuts.zenModeDesc')"
        >
          <div class="flex items-center gap-2">
            <kbd
              class="rounded-md border border-border bg-muted/50 px-2 py-0.5 font-mono text-[11.5px] text-foreground/80"
            >
              z
            </kbd>
            <Button
              type="button"
              size="sm"
              variant="outline"
              class="h-8"
              @click="onZenClick"
            >
              {{
                zenMode
                  ? t("settings.shortcuts.zenModeExit")
                  : t("settings.shortcuts.zenModeEnter")
              }}
            </Button>
          </div>
        </SettingsRow>
      </div>

      <div class="py-2">
        <ul class="divide-y divide-border/60">
          <li
            v-for="item in shortcuts"
            :key="item.keys + item.action"
            class="flex items-center justify-between gap-3 py-2.5 text-[13px]"
          >
            <span class="text-muted-foreground">{{ item.action }}</span>
            <kbd
              class="rounded-md border border-border bg-muted/50 px-2 py-0.5 font-mono text-[11.5px] text-foreground/80"
            >
              {{ item.keys }}
            </kbd>
          </li>
        </ul>
      </div>
    </SettingsGroup>
  </div>
</template>
