<script setup lang="ts">
import { ref } from "vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { loadAppsvc } from "@/lib/backend";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

const { t } = useI18n();
const { settings, persistUIPrefs } = useRssStore();

const testing = ref(false);
const status = ref("");

async function patchBool(key: "notifyOnNewArticles" | "notifySound", v: boolean) {
  settings[key] = v;
  persistUIPrefs();
  if (key === "notifyOnNewArticles" && v) {
    await ensurePermission();
  }
}

async function ensurePermission(): Promise<boolean> {
  const api = await loadAppsvc();
  const fn = api?.SettingsService?.EnsureNotificationPermission;
  if (typeof fn !== "function") return true;
  try {
    const ok = await fn();
    if (!ok) {
      status.value = t("settings.notifications.permissionDenied");
      toast.error(t("settings.notifications.permissionDenied"));
      return false;
    }
    return true;
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    console.warn("[lrss] EnsureNotificationPermission", e);
    status.value = msg;
    return false;
  }
}

async function onTestNotification() {
  if (testing.value) return;
  testing.value = true;
  status.value = "";
  try {
    const ok = await ensurePermission();
    if (!ok) return;
    const api = await loadAppsvc();
    const testFn = api?.SettingsService?.TestNotification;
    if (typeof testFn !== "function") {
      status.value = t("settings.notifications.unavailable");
      toast.error(t("settings.notifications.unavailable"));
      return;
    }
    await testFn();
    status.value = t("settings.notifications.testSent");
    toast.success(t("settings.notifications.testSent"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    status.value = msg;
    toast.error(msg);
    console.warn("[lrss] TestNotification", e);
  } finally {
    testing.value = false;
  }
}
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup
      :title="t('settings.notifications.group')"
      :description="t('settings.notifications.groupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.notifications.newArticles')"
          :description="t('settings.notifications.newArticlesDesc')"
        >
          <Switch
            :checked="settings.notifyOnNewArticles"
            @update:checked="(v: boolean) => void patchBool('notifyOnNewArticles', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5" :class="!settings.notifyOnNewArticles && 'opacity-50'">
        <SettingsRow
          :title="t('settings.notifications.sound')"
          :description="t('settings.notifications.soundDesc')"
        >
          <Switch
            :checked="settings.notifySound"
            :disabled="!settings.notifyOnNewArticles"
            @update:checked="(v: boolean) => void patchBool('notifySound', v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.notifications.test')"
          :description="t('settings.notifications.testDesc')"
        >
          <Button
            type="button"
            variant="outline"
            size="sm"
            :disabled="testing"
            @click="onTestNotification"
          >
            {{ testing ? t("settings.notifications.testing") : t("settings.notifications.test") }}
          </Button>
        </SettingsRow>
      </div>
      <p
        v-if="status"
        class="px-0.5 pb-2.5 text-[12px] leading-relaxed text-muted-foreground"
        role="status"
      >
        {{ status }}
      </p>
    </SettingsGroup>
  </div>
</template>
