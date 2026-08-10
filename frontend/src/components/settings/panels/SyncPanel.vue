<script setup lang="ts">
import { useI18n } from "vue-i18n";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const { t } = useI18n();
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.sync.group')"
      :description="t('settings.sync.groupDesc')"
    >
      <p
        class="mb-2 rounded-md border border-border/70 bg-muted/40 px-3 py-2 text-[12px] leading-relaxed text-muted-foreground"
        role="status"
      >
        {{ t("settings.unavailable.previewBanner") }}
      </p>
      <div class="py-2.5 opacity-60">
        <SettingsRow
          :title="t('settings.sync.enable')"
          :description="t('settings.sync.enableDesc')"
        >
          <Switch :checked="false" disabled :aria-disabled="true" />
        </SettingsRow>
      </div>
      <div class="py-2.5 opacity-60">
        <SettingsRow
          :title="t('settings.sync.provider')"
          :description="t('settings.sync.providerDesc')"
        >
          <Select model-value="none" disabled>
            <SelectTrigger class="h-8 w-[150px] text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">{{ t("settings.sync.providerNone") }}</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>
      <p class="text-[11.5px] text-muted-foreground">
        {{ t("settings.unavailable.syncNote") }}
      </p>
    </SettingsGroup>
  </div>
</template>
