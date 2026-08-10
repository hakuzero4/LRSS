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
    <SettingsGroup title="主题" description="界面配色与侧栏密度。">
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">外观</p>
        <Tabs v-model="settings.theme" class="w-full">
          <TabsList class="grid w-full grid-cols-3">
            <TabsTrigger value="system">跟随系统</TabsTrigger>
            <TabsTrigger value="light">浅色</TabsTrigger>
            <TabsTrigger value="dark">深色</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <div class="space-y-3 py-3">
        <p class="text-[13px] font-medium">强调色</p>
        <Tabs v-model="settings.accent" class="w-full">
          <TabsList class="grid w-full grid-cols-4">
            <TabsTrigger value="purple">紫</TabsTrigger>
            <TabsTrigger value="blue">蓝</TabsTrigger>
            <TabsTrigger value="teal">青</TabsTrigger>
            <TabsTrigger value="orange">橙</TabsTrigger>
          </TabsList>
        </Tabs>
      </div>
      <div class="py-2.5">
        <SettingsRow title="紧凑侧栏" description="缩小侧栏行高，显示更多订阅源。">
          <Switch
            :checked="settings.compactSidebar"
            @update:checked="(v: boolean) => (settings.compactSidebar = v)"
          />
        </SettingsRow>
      </div>
    </SettingsGroup>
  </div>
</template>
