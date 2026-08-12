/**
 * Run: npx tsx src/lib/checkUpdate.selftest.ts
 */
import { compareVersions, normalizeVersion, checkForUpdate } from "./checkUpdate";

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(`ASSERT: ${msg}`);
}

assert(normalizeVersion("v0.1.1") === "0.1.1", "strip v");
assert(normalizeVersion("0.1.1-beta") === "0.1.1", "strip pre");
assert(compareVersions("0.1.0", "0.1.1") < 0, "older");
assert(compareVersions("0.1.1", "0.1.0") > 0, "newer");
assert(compareVersions("v0.1.1", "0.1.1") === 0, "equal");
assert(compareVersions("0.2.0", "0.10.0") < 0, "numeric not lexical");

async function mockOk() {
  const r = await checkForUpdate({
    currentVersion: "0.1.0",
    fetchImpl: async () =>
      new Response(
        JSON.stringify({
          tag_name: "v0.1.1",
          name: "LRSS v0.1.1",
          html_url: "https://github.com/hakuzero4/LRSS/releases/tag/v0.1.1",
        }),
        { status: 200 },
      ),
  });
  assert(r.status === "updateAvailable", "update available");
  if (r.status === "updateAvailable") {
    assert(r.latest === "0.1.1", "latest");
  }
}

async function mockUpToDate() {
  const r = await checkForUpdate({
    currentVersion: "0.1.1",
    fetchImpl: async () =>
      new Response(JSON.stringify({ tag_name: "v0.1.1", html_url: "https://x" }), {
        status: 200,
      }),
  });
  assert(r.status === "upToDate", "up to date");
}

await mockOk();
await mockUpToDate();
console.log("checkUpdate.selftest: OK");
