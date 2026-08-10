<script setup lang="ts">
import { computed } from "vue";
import { useI18n } from "vue-i18n";
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

const shortcuts = computed(() => [
  { keys: "j / k", action: t("settings.shortcuts.nextPrev") },
  { keys: "o / Enter", action: t("settings.shortcuts.openFocus") },
  { keys: "r", action: t("settings.shortcuts.refresh") },
  { keys: "s", action: t("settings.shortcuts.toggleStar") },
  { keys: "m", action: t("settings.shortcuts.toggleRead") },
  { keys: "⌘ ,", action: t("settings.shortcuts.openSettings") },
]);

function onEnableChange(v: boolean) {
  settings.enableKeyboardShortcuts = v;
  persistUIPrefs();
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
      <div class="py-2">
        <ul class="divide-y divide-border/60">
          <li
            v-for="item in shortcuts"
            :key="item.keys"
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
