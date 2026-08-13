/**
 * App identity for About / update checks.
 * Keep APP_VERSION in sync with git release tags (vX.Y.Z) and build/config.yml when shipping.
 */

export const APP_NAME = "LRSS";

/** Semver without leading "v" (display + compare). */
export const APP_VERSION = "0.1.4";

/** GitHub owner/repo used for releases and docs. */
export const GITHUB_OWNER = "hakuzero4";
export const GITHUB_REPO = "LRSS";

export const GITHUB_REPO_URL = `https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}`;
export const GITHUB_RELEASES_URL = `${GITHUB_REPO_URL}/releases`;
export const GITHUB_LATEST_API = `https://api.github.com/repos/${GITHUB_OWNER}/${GITHUB_REPO}/releases/latest`;
