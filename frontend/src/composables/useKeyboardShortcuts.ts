import { onMounted, onUnmounted } from "vue";
import { useRssStore } from "@/composables/useRssStore";
import { openExternalLink } from "@/lib/openLink";

/** Documented shortcut map (keys → action id). */
export const KEYBOARD_SHORTCUT_MAP = {
  j: "nextArticle",
  k: "previousArticle",
  s: "toggleStar",
  m: "toggleRead",
  r: "refreshFeeds",
  o: "openArticleUrl",
  Enter: "openArticleUrl",
  "/": "focusSearch",
  Escape: "closeOrClearSearch",
  "Ctrl+,": "openSettings",
  "Meta+,": "openSettings",
  n: "openAddFeed",
  z: "toggleZenMode",
} as const;

export type ShortcutAction = (typeof KEYBOARD_SHORTCUT_MAP)[keyof typeof KEYBOARD_SHORTCUT_MAP];

function isEditableTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const el =
    target.closest("input, textarea, select, [contenteditable=''], [contenteditable='true']") ??
    target;
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return true;
  if (el.isContentEditable) return true;
  return false;
}

function focusSearchInput(): boolean {
  const byType = document.querySelector<HTMLInputElement>('input[type="search"]');
  if (byType) {
    byType.focus();
    byType.select();
    return true;
  }
  const byAria = document.querySelector<HTMLInputElement>(
    'input[aria-label], input[type="text"][placeholder]',
  );
  if (byAria) {
    byAria.focus();
    byAria.select();
    return true;
  }
  return false;
}

/**
 * Global reader keyboard shortcuts when `settings.enableKeyboardShortcuts` is true.
 * Mount once from AppLayout (window keydown).
 */
export function useKeyboardShortcuts() {
  const {
    settings,
    filteredArticles,
    selectedArticleId,
    selectedArticle,
    searchQuery,
    settingsOpen,
    addFeedOpen,
    zenMode,
    selectArticle,
    toggleStar,
    toggleRead,
    refreshFeeds,
    openSettings,
    closeSettings,
    openAddFeed,
    closeAddFeed,
    toggleZenMode,
    setZenMode,
  } = useRssStore();

  function moveSelection(delta: 1 | -1) {
    const list = filteredArticles.value;
    if (list.length === 0) return;

    const currentId = selectedArticleId.value;
    let idx = currentId ? list.findIndex((a) => a.id === currentId) : -1;

    if (idx < 0) {
      idx = delta > 0 ? 0 : list.length - 1;
    } else {
      idx = Math.max(0, Math.min(list.length - 1, idx + delta));
    }

    void selectArticle(list[idx]!.id);
  }

  function openSelectedUrl() {
    const article = selectedArticle.value;
    if (!article?.url) return;
    void openExternalLink(article.url, {
      forceBrowser: settings.openLinksInBrowser,
    });
  }

  function onKeydown(e: KeyboardEvent) {
    if (!settings.enableKeyboardShortcuts) return;
    // Ignore pure modifier presses
    if (e.key === "Control" || e.key === "Meta" || e.key === "Alt" || e.key === "Shift") return;

    const mod = e.ctrlKey || e.metaKey;
    const editable = isEditableTarget(e.target);

    // Ctrl/Meta+, — open settings (works while typing)
    if (mod && e.key === ",") {
      e.preventDefault();
      openSettings();
      return;
    }

    // Escape — close dialogs / clear search / exit zen (works while typing)
    if (e.key === "Escape") {
      if (settingsOpen.value) {
        e.preventDefault();
        closeSettings();
        return;
      }
      if (addFeedOpen.value) {
        e.preventDefault();
        closeAddFeed();
        return;
      }
      if (searchQuery.value) {
        e.preventDefault();
        searchQuery.value = "";
        return;
      }
      if (zenMode.value) {
        e.preventDefault();
        setZenMode(false);
        return;
      }
      if (editable) {
        e.preventDefault();
        (e.target as HTMLElement).blur();
      }
      return;
    }

    // All other shortcuts: ignore while typing in inputs / contenteditable
    if (editable) return;
    // Don't steal browser/system chords (except Ctrl+, handled above)
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    const key = e.key.length === 1 ? e.key.toLowerCase() : e.key;

    switch (key) {
      case "j":
        e.preventDefault();
        moveSelection(1);
        break;
      case "k":
        e.preventDefault();
        moveSelection(-1);
        break;
      case "s": {
        const id = selectedArticleId.value;
        if (!id) return;
        e.preventDefault();
        void toggleStar(id);
        break;
      }
      case "m": {
        const id = selectedArticleId.value;
        if (!id) return;
        e.preventDefault();
        void toggleRead(id);
        break;
      }
      case "r":
        e.preventDefault();
        void refreshFeeds();
        break;
      case "o":
      case "Enter":
        if (!selectedArticle.value?.url) return;
        e.preventDefault();
        openSelectedUrl();
        break;
      case "/":
        e.preventDefault();
        focusSearchInput();
        break;
      case "n":
        e.preventDefault();
        openAddFeed();
        break;
      case "z":
        e.preventDefault();
        toggleZenMode();
        break;
      default:
        break;
    }
  }

  onMounted(() => {
    window.addEventListener("keydown", onKeydown);
  });

  onUnmounted(() => {
    window.removeEventListener("keydown", onKeydown);
  });

  return {
    shortcutMap: KEYBOARD_SHORTCUT_MAP,
  };
}
