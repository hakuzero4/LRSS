<script setup lang="ts">
import { computed, ref } from "vue";
import { Trash2 } from "@lucide/vue";
import { useI18n } from "vue-i18n";
import { toast } from "vue-sonner";
import { useRssStore } from "@/composables/useRssStore";
import { parseBlockKeywords } from "@/lib/articleFilters";
import {
  formatKeepConfidence,
  keepConfidenceThreshold,
  parseKeepLog,
  profileIsEmpty,
  profilePreview,
  smartKeepState,
  type KeepDecision,
} from "@/lib/filterStatus";
import { firstLevelKeepFolders, keepFolderOptions } from "@/lib/keepFolders";
import SettingsGroup from "@/components/settings/SettingsGroup.vue";
import SettingsRow from "@/components/settings/SettingsRow.vue";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import type { AppSettings } from "@/types/rss";

const { t } = useI18n();
const {
  settings,
  persistUIPrefs,
  assistant,
  jobActivity,
  smartCounts,
  scanUnreadForKeep,
  keepFolders,
  createKeepFolder,
  deleteKeepFolder,
  webMode,
  closeSettings,
  selectArticle,
} = useRssStore();

const keywordCount = computed(() => parseBlockKeywords(settings.blockKeywords).length);
const modelReady = computed(() => assistant.llmConfigured);
const scanning = ref(false);

const keepErrorText = computed(() => {
  const raw = jobActivity.keepError.trim();
  if (!raw) return "";
  const low = raw.toLowerCase();
  if (low.includes("timeout") || low.includes("deadline") || low.includes("awaiting response")) {
    return t("settings.filters.errTimeout");
  }
  return t("settings.filters.errGeneric", { err: raw });
});

const statusLine = computed(() => {
  const parts: string[] = [];
  if (jobActivity.keepState === "judging") {
    parts.push(t("settings.filters.statusJudging"));
  } else if (jobActivity.keepPending > 0) {
    parts.push(t("settings.filters.statusQueued", { pending: jobActivity.keepPending }));
  } else {
    parts.push(t("settings.filters.statusIdle"));
  }
  parts.push(t("settings.filters.statusKept", { n: smartCounts.kept }));
  return parts.join(" · ");
});

type ActiveRule = { id: string; on: boolean; title: string; detail: string };

const activeRules = computed((): ActiveRule[] => {
  const kws = parseBlockKeywords(settings.blockKeywords);
  const smart = smartKeepState(settings.smartFilterEnabled, modelReady.value);
  const thr = keepConfidenceThreshold(settings.smartFilterStrictness);
  const strictLabel =
    settings.smartFilterStrictness === "loose"
      ? t("settings.filters.loose")
      : settings.smartFilterStrictness === "strict"
        ? t("settings.filters.strict")
        : t("settings.filters.standard");
  const folderNames = keepFolders.value.map((f) => f.name).filter(Boolean);
  const smartOn = smart === "on";
  const smartTitle =
    smart === "on"
      ? t("settings.filters.activeSmartOn", { strict: strictLabel, threshold: thr.toFixed(2) })
      : smart === "need-model"
        ? t("settings.filters.activeSmartNeedModel")
        : t("settings.filters.activeSmartOff");
  return [
    {
      id: "duplicates",
      on: settings.hideDuplicateTitles,
      title: t("settings.filters.activeDuplicates"),
      detail: settings.hideDuplicateTitles
        ? t("settings.filters.activeDuplicatesOn")
        : t("settings.filters.activeDuplicatesOff"),
    },
    {
      id: "keywords",
      on: kws.length > 0,
      title: t("settings.filters.activeKeywordsTitle"),
      detail: kws.length
        ? t("settings.filters.activeKeywords", { words: kws.join(t("activity.listSep")) })
        : t("settings.filters.activeKeywordsOff"),
    },
    {
      id: "unread",
      on: settings.showUnreadOnly,
      title: t("settings.filters.activeUnreadOnly"),
      detail: settings.showUnreadOnly
        ? t("settings.filters.activeUnreadOnlyOn")
        : t("settings.filters.activeUnreadOnlyOff"),
    },
    {
      id: "office",
      on: !settings.nsfwMode,
      title: t("settings.filters.layerOffice"),
      detail: settings.nsfwMode
        ? t("settings.filters.activeOfficeOff")
        : t("settings.filters.activeOffice"),
    },
    {
      id: "smart",
      on: smartOn,
      title: t("settings.filters.layerSmart"),
      detail: smartTitle,
    },
    {
      id: "profile",
      on: smartOn && !profileIsEmpty(settings.smartFilterProfile),
      title: t("settings.filters.profile"),
      detail: !smartOn
        ? t("settings.filters.activeProfileIdle")
        : profileIsEmpty(settings.smartFilterProfile)
          ? t("settings.filters.activeProfileEmpty")
          : t("settings.filters.activeProfileSet", {
              preview: profilePreview(settings.smartFilterProfile),
            }),
    },
    {
      id: "folders",
      on: smartOn && folderNames.length > 0,
      title: t("settings.filters.keepFolders"),
      detail: folderNames.length
        ? t("settings.filters.activeFolders", { names: folderNames.join(t("activity.listSep")) })
        : t("settings.filters.activeFoldersNone"),
    },
  ];
});

const keepLog = computed(() =>
  jobActivity.keepLog.length ? jobActivity.keepLog : parseKeepLog(jobActivity.keepLog),
);

function gateLabel(d: KeepDecision): string {
  switch (d.gate) {
    case "keyword":
      return t("settings.filters.gateKeyword", { keyword: d.reason || "—" });
    case "untitled":
      return t("settings.filters.gateUntitled");
    case "nsfw":
      return t("settings.filters.gateNsfw");
    case "confidence": {
      const pct = formatKeepConfidence(d.confidence);
      return pct
        ? t("settings.filters.gateConfidence", { confidence: pct })
        : t("settings.filters.gateAi");
    }
    case "unsure":
      return t("settings.filters.gateUnsure");
    default:
      return d.outcome === "kept"
        ? d.folder
          ? t("settings.filters.logKeptFolder", { folder: d.folder })
          : t("settings.filters.logKept")
        : d.reason || t("settings.filters.gateAi");
  }
}

function logLine(d: KeepDecision): string {
  const bits = [gateLabel(d)];
  if (d.outcome === "kept") {
    const pct = formatKeepConfidence(d.confidence);
    if (pct) bits.push(`${pct}%`);
    if (d.reason) bits.push(d.reason);
  } else if (d.gate !== "keyword" && d.reason && d.gate !== "untitled" && d.gate !== "nsfw") {
    bits.push(d.reason);
  }
  return bits.filter(Boolean).join(" · ");
}

function onOpenDecision(d: KeepDecision) {
  if (!d.articleId) return;
  closeSettings();
  void selectArticle(d.articleId);
}

function onHideDuplicates(v: boolean) {
  settings.hideDuplicateTitles = v;
  persistUIPrefs();
}

function onBlockKeywords(v: string | number) {
  settings.blockKeywords = String(v ?? "");
  persistUIPrefs();
}

function onSmartEnabled(v: boolean) {
  if (v && !modelReady.value) return;
  settings.smartFilterEnabled = v;
  persistUIPrefs();
}

function onProfile(v: string | number) {
  settings.smartFilterProfile = String(v ?? "");
  persistUIPrefs();
}

const strictnessModel = computed({
  get: () => settings.smartFilterStrictness,
  set: (v: string | number | boolean | Record<string, unknown> | null) => {
    const s = String(v ?? "");
    if (s === "loose" || s === "standard" || s === "strict") {
      settings.smartFilterStrictness = s as AppSettings["smartFilterStrictness"];
      persistUIPrefs();
    }
  },
});

const keepFolderRows = computed(() => keepFolderOptions(keepFolders.value));
const firstLevelParents = computed(() => firstLevelKeepFolders(keepFolders.value));
const newKeepName = ref("");
const newKeepParent = ref("none");
const keepBusy = ref(false);

async function onAddKeepFolder() {
  const name = newKeepName.value.trim();
  if (!name) {
    toast.error(t("keepFolder.emptyName"));
    return;
  }
  if (keepBusy.value) return;
  keepBusy.value = true;
  try {
    await createKeepFolder(name, newKeepParent.value === "none" ? undefined : newKeepParent.value);
    newKeepName.value = "";
    newKeepParent.value = "none";
    toast.success(t("keepFolder.created"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("keepFolder.createFailed"), { description: msg });
  } finally {
    keepBusy.value = false;
  }
}

async function onDeleteKeepFolder(id: string) {
  const folder = keepFolders.value.find((f) => f.id === id);
  if (!folder) return;
  if (!window.confirm(t("keepFolder.deleteConfirmBody", { name: folder.name }))) return;
  if (keepBusy.value) return;
  keepBusy.value = true;
  try {
    await deleteKeepFolder(id);
    if (newKeepParent.value === id) newKeepParent.value = "none";
    toast.success(t("keepFolder.deleteDone"));
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("keepFolder.deleteFailed"), { description: msg });
  } finally {
    keepBusy.value = false;
  }
}

async function onScanUnread() {
  if (!modelReady.value || scanning.value) return;
  scanning.value = true;
  try {
    const n = await scanUnreadForKeep();
    if (n <= 0) {
      toast.message(t("settings.filters.scanEmpty"));
    } else {
      toast.success(t("settings.filters.scanQueued", { n }));
    }
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    toast.error(t("settings.filters.scanFailed"), { description: msg });
  } finally {
    scanning.value = false;
  }
}
</script>

<template>
  <div class="space-y-7">
    <section class="space-y-3">
      <header class="space-y-0.5">
        <h3 class="text-[13px] font-semibold tracking-tight">
          {{ t("settings.filters.activeTitle") }}
        </h3>
        <p class="text-[12px] leading-relaxed text-muted-foreground">
          {{ t("settings.filters.activeDesc") }}
        </p>
      </header>
      <div class="rounded-xl border border-border/70 bg-card/50 px-3.5 py-3 shadow-sm shadow-black/[0.02]">
        <ul class="space-y-2.5">
          <li
            v-for="rule in activeRules"
            :key="rule.id"
            class="flex items-start gap-2.5"
          >
            <span
              class="mt-1.5 size-1.5 shrink-0 rounded-full"
              :class="rule.on ? 'bg-primary' : 'bg-muted-foreground/35'"
              :aria-label="rule.on ? t('settings.filters.ruleOn') : t('settings.filters.ruleOff')"
            />
            <div class="min-w-0 flex-1">
              <p class="text-[13px] font-medium leading-snug">{{ rule.title }}</p>
              <p class="mt-0.5 text-[12px] leading-relaxed text-muted-foreground">
                {{ rule.detail }}
              </p>
            </div>
          </li>
        </ul>
        <p class="mt-3 border-t border-border/60 pt-3 text-[12px] leading-relaxed text-muted-foreground">
          {{ t("settings.filters.processBody") }}
        </p>
        <p class="mt-2 text-[11.5px] tabular-nums text-muted-foreground">
          {{ statusLine }}
        </p>
        <p
          v-if="keepErrorText"
          class="mt-1.5 text-[12px] leading-relaxed text-destructive"
        >
          {{ keepErrorText }}
        </p>
      </div>
    </section>

    <section class="space-y-3">
      <header class="space-y-0.5">
        <h3 class="text-[13px] font-semibold tracking-tight">
          {{ t("settings.filters.logTitle") }}
        </h3>
        <p class="text-[12px] leading-relaxed text-muted-foreground">
          {{ t("settings.filters.logDesc") }}
        </p>
      </header>
      <div class="overflow-hidden rounded-xl border border-border/70 bg-card/50 shadow-sm shadow-black/[0.02]">
        <p
          v-if="!keepLog.length"
          class="px-3.5 py-3 text-[12px] leading-relaxed text-muted-foreground"
        >
          {{ keepErrorText || t("settings.filters.logEmpty") }}
        </p>
        <ul v-else class="divide-y divide-border/60">
          <li
            v-for="d in keepLog"
            :key="d.articleId + d.at"
          >
            <button
              type="button"
              class="flex w-full items-start gap-2.5 px-3.5 py-2.5 text-left transition-colors hover:bg-muted/40 active:scale-[0.995]"
              @click="onOpenDecision(d)"
            >
              <span
                class="mt-1.5 size-1.5 shrink-0 rounded-full"
                :class="d.outcome === 'kept' ? 'bg-primary' : 'bg-muted-foreground/40'"
              />
              <span class="min-w-0 flex-1">
                <span class="block truncate text-[13px] font-medium">
                  {{ d.title || t("settings.filters.logNoTitle") }}
                </span>
                <span class="mt-0.5 block truncate text-[12px] text-muted-foreground">
                  {{
                    d.outcome === "kept"
                      ? t("settings.filters.logKeptMark")
                      : t("settings.filters.logSkipMark")
                  }}
                  · {{ logLine(d) }}
                </span>
              </span>
            </button>
          </li>
        </ul>
      </div>
    </section>

    <SettingsGroup
      :title="t('settings.filters.group')"
      :description="t('settings.filters.groupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.filters.hideDuplicates')"
          :description="t('settings.filters.hideDuplicatesDesc')"
        >
          <Switch
            :checked="settings.hideDuplicateTitles"
            @update:checked="onHideDuplicates"
          />
        </SettingsRow>
      </div>
      <div class="space-y-2 py-3">
        <div class="flex items-end justify-between gap-2">
          <div>
            <p class="text-[13px] font-medium">{{ t("settings.filters.blockKeywords") }}</p>
            <p class="mt-0.5 text-[12px] text-muted-foreground">
              {{ t("settings.filters.blockKeywordsDesc") }}
            </p>
          </div>
          <span
            v-if="keywordCount > 0"
            class="shrink-0 tabular-nums text-[11.5px] text-muted-foreground"
          >
            {{ t("settings.filters.keywordCount", { n: keywordCount }) }}
          </span>
        </div>
        <Input
          :model-value="settings.blockKeywords"
          :placeholder="t('settings.filters.blockKeywordsPlaceholder')"
          class="h-9 text-[13px]"
          @update:model-value="onBlockKeywords"
        />
        <p class="text-[11.5px] leading-relaxed text-muted-foreground">
          {{ t("settings.filters.applyHint") }}
        </p>
      </div>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.filters.smartGroup')"
      :description="t('settings.filters.smartGroupDesc')"
    >
      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.filters.smartEnabled')"
          :description="t('settings.filters.smartEnabledDesc')"
        >
          <Switch
            :checked="settings.smartFilterEnabled && modelReady"
            :disabled="!modelReady"
            @update:checked="onSmartEnabled"
          />
        </SettingsRow>
        <p
          v-if="!modelReady"
          class="mt-1.5 text-[11.5px] leading-relaxed text-muted-foreground"
        >
          {{ t("settings.filters.needModel") }}
        </p>
      </div>

      <div class="space-y-2 py-3">
        <p class="text-[13px] font-medium">{{ t("settings.filters.profile") }}</p>
        <p class="text-[12px] text-muted-foreground">
          {{ t("settings.filters.profileDesc") }}
        </p>
        <Textarea
          :model-value="settings.smartFilterProfile"
          :placeholder="t('settings.filters.profilePlaceholder')"
          :disabled="!modelReady"
          class="min-h-[96px] text-[13px] leading-relaxed"
          @update:model-value="onProfile"
        />
      </div>

      <div class="py-2.5">
        <SettingsRow
          :title="t('settings.filters.strictness')"
          :description="t('settings.filters.strictnessDesc')"
        >
          <Select v-model="strictnessModel" :disabled="!modelReady">
            <SelectTrigger class="h-8 w-[132px] text-[13px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="loose">{{ t("settings.filters.loose") }}</SelectItem>
              <SelectItem value="standard">{{ t("settings.filters.standard") }}</SelectItem>
              <SelectItem value="strict">{{ t("settings.filters.strict") }}</SelectItem>
            </SelectContent>
          </Select>
        </SettingsRow>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-3 py-3">
        <p class="text-[11.5px] leading-relaxed text-muted-foreground">
          {{ statusLine }}
        </p>
        <Button
          variant="outline"
          size="sm"
          class="h-8 text-[12.5px]"
          :disabled="!modelReady || scanning"
          @click="onScanUnread"
        >
          {{ scanning ? t("settings.filters.scanning") : t("settings.filters.scanUnread") }}
        </Button>
      </div>
    </SettingsGroup>

    <SettingsGroup
      :title="t('settings.filters.keepFolders')"
      :description="t('settings.filters.keepFoldersDesc')"
    >
      <div v-if="keepFolderRows.length" class="divide-y divide-border/60">
        <div
          v-for="row in keepFolderRows"
          :key="row.id"
          class="flex items-center gap-2 py-2"
        >
          <span
            class="min-w-0 flex-1 truncate text-[13px]"
            :class="row.depth ? 'pl-4 text-muted-foreground' : ''"
          >
            {{ row.name }}
          </span>
          <Button
            v-if="!webMode"
            type="button"
            variant="ghost"
            size="icon-xs"
            class="text-muted-foreground"
            :disabled="keepBusy"
            :aria-label="t('keepFolder.delete')"
            @click="onDeleteKeepFolder(row.id)"
          >
            <Trash2 class="size-3.5" />
          </Button>
        </div>
      </div>
      <p
        v-else
        class="py-3 text-[12px] text-muted-foreground"
      >
        {{ t("settings.filters.keepFolderEmpty") }}
      </p>
      <div v-if="!webMode" class="space-y-2 py-3">
        <div class="flex flex-wrap items-center gap-2">
          <Input
            v-model="newKeepName"
            :placeholder="t('settings.filters.keepFolderNamePlaceholder')"
            class="h-8 min-w-[140px] flex-1 text-[13px]"
            :disabled="keepBusy"
            @keydown.enter.prevent="onAddKeepFolder"
          />
          <Button
            type="button"
            size="sm"
            class="h-8 text-[12.5px]"
            :disabled="keepBusy || !newKeepName.trim()"
            @click="onAddKeepFolder"
          >
            {{ t("settings.filters.keepFolderAdd") }}
          </Button>
        </div>
        <div class="flex items-center gap-2">
          <span class="shrink-0 text-[12px] text-muted-foreground">
            {{ t("settings.filters.keepFolderParent") }}
          </span>
          <Select v-model="newKeepParent">
            <SelectTrigger class="h-8 w-[180px] text-[12.5px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="none">
                {{ t("settings.filters.keepFolderParentNone") }}
              </SelectItem>
              <SelectItem
                v-for="p in firstLevelParents"
                :key="p.id"
                :value="p.id"
              >
                {{ p.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
    </SettingsGroup>
  </div>
</template>
