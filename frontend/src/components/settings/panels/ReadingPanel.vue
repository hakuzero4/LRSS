<script setup lang="ts">
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";

const { settings } = useRssStore();
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup title="阅读体验" description="正文排版与列表过滤。">
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">正文字号</p>
        <Tabs v-model="settings.fontSize" class="w-full">
          <TabsList class="grid w-full grid-cols-3">
            <TabsTrigger value="sm">小</TabsTrigger>
            <TabsTrigger value="md">中</TabsTrigger>
            <TabsTrigger value="lg">大</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">阅读宽度</p>
        <Tabs v-model="settings.readerWidth" class="w-full">
          <TabsList class="grid w-full grid-cols-3">
            <TabsTrigger value="narrow">窄</TabsTrigger>
            <TabsTrigger value="medium">适中</TabsTrigger>
            <TabsTrigger value="wide">宽</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <div class="py-2.5">
        <SettingsRow title="仅显示未读" description="列表中隐藏已读文章（收藏列表除外）。">
          <Switch
            :checked="settings.showUnreadOnly"
            @update:checked="(v: boolean) => (settings.showUnreadOnly = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow title="在浏览器中打开链接" description="原文与正文外链使用系统浏览器。">
          <Switch
            :checked="settings.openLinksInBrowser"
            @update:checked="(v: boolean) => (settings.openLinksInBrowser = v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
