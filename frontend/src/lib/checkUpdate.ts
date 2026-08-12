/**
 * Check GitHub Releases for a newer app version.
 */

import {
  APP_VERSION,
  GITHUB_LATEST_API,
  GITHUB_RELEASES_URL,
  GITHUB_REPO_URL,
} from "./appMeta";

export type UpdateCheckResult =
  | { status: "upToDate"; current: string; latest: string; htmlUrl: string }
  | { status: "updateAvailable"; current: string; latest: string; htmlUrl: string; name?: string }
  | { status: "error"; current: string; message: string };

/** Strip leading v/V and pre-release/build metadata for numeric compare. */
export function normalizeVersion(raw: string): string {
  return String(raw ?? "")
    .trim()
    .replace(/^v/i, "")
    .split(/[-+]/)[0]
    .trim();
}

/** Compare two semver-like strings (major.minor.patch). Returns -1 / 0 / 1. */
export function compareVersions(a: string, b: string): number {
  const pa = normalizeVersion(a)
    .split(".")
    .map((x) => {
      const n = parseInt(x, 10);
      return Number.isFinite(n) ? n : 0;
    });
  const pb = normalizeVersion(b)
    .split(".")
    .map((x) => {
      const n = parseInt(x, 10);
      return Number.isFinite(n) ? n : 0;
    });
  const len = Math.max(pa.length, pb.length, 3);
  for (let i = 0; i < len; i++) {
    const da = pa[i] ?? 0;
    const db = pb[i] ?? 0;
    if (da < db) return -1;
    if (da > db) return 1;
  }
  return 0;
}

type GhRelease = {
  tag_name?: string;
  name?: string;
  html_url?: string;
  draft?: boolean;
  prerelease?: boolean;
};

export type CheckUpdateDeps = {
  fetchImpl?: typeof fetch;
  currentVersion?: string;
  apiUrl?: string;
};

/**
 * Fetch latest GitHub release and compare to the running app version.
 * Uses public API (no auth); subject to unauthenticated rate limits.
 */
export async function checkForUpdate(deps?: CheckUpdateDeps): Promise<UpdateCheckResult> {
  const current = normalizeVersion(deps?.currentVersion ?? APP_VERSION) || APP_VERSION;
  const apiUrl = deps?.apiUrl ?? GITHUB_LATEST_API;
  const fetchImpl = deps?.fetchImpl ?? fetch;

  try {
    const res = await fetchImpl(apiUrl, {
      headers: {
        Accept: "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
      },
    });
    if (res.status === 404) {
      return {
        status: "error",
        current,
        message: "no_releases",
      };
    }
    if (!res.ok) {
      return {
        status: "error",
        current,
        message: `http_${res.status}`,
      };
    }
    const data = (await res.json()) as GhRelease;
    const tag = String(data.tag_name ?? "").trim();
    if (!tag) {
      return { status: "error", current, message: "invalid_release" };
    }
    const latest = normalizeVersion(tag);
    const htmlUrl =
      (typeof data.html_url === "string" && data.html_url) ||
      GITHUB_RELEASES_URL ||
      GITHUB_REPO_URL;
    const cmp = compareVersions(current, latest);
    if (cmp < 0) {
      return {
        status: "updateAvailable",
        current,
        latest,
        htmlUrl,
        name: data.name || tag,
      };
    }
    return {
      status: "upToDate",
      current,
      latest,
      htmlUrl,
    };
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    return { status: "error", current, message: msg || "network_error" };
  }
}
