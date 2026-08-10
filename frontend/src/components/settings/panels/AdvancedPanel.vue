<script setup lang="ts">
import { useRssStore } from "@/composables/useRssStore";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";

const { settings } = useRssStore();
</script>

<template>
  <div class="space-y-7">
    <SettingsGroup title="高级" description="性能与诊断选项。请谨慎修改。">
      <div class="py-2.5">
        <SettingsRow title="硬件加速" description="使用 GPU 渲染界面（推荐开启）。">
          <Switch
            :checked="settings.hardwareAcceleration"
            @update:checked="(v: boolean) => (settings.hardwareAcceleration = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow title="退出时清理缓存" description="关闭应用时清除临时网页缓存。">
          <Switch
            :checked="settings.clearCacheOnQuit"
            @update:checked="(v: boolean) => (settings.clearCacheOnQuit = v)"
          />
        </SettingsRow>
      </div>
      <div class="py-2.5">
        <SettingsRow title="开发者模式" description="显示额外日志与调试入口。">
          <Switch
            :checked="settings.developerMode"
            @update:checked="(v: boolean) => (settings.developerMode = v)"
          />
        </SettingsRow>
      </div>
      <div class="flex flex-wrap gap-2 py-3">
        <Button variant="outline" size="sm" type="button">导出日志</Button>
        <Button variant="outline" size="sm" type="button">重置全部设置</Button>
      </div>
    </SettingsGroup>
  </div>
</template>
