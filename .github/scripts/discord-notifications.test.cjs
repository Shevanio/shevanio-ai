const assert = require("node:assert/strict");
const { readFileSync } = require("node:fs");
const { resolve } = require("node:path");
const { spawnSync } = require("node:child_process");
const test = require("node:test");

const workflowPath = resolve(__dirname, "../workflows/discord-notifications.yml");
const workflow = readFileSync(workflowPath, "utf8");
const scripts = [...workflow.matchAll(/^ {8}run: \|\n((?:^ {10}.*(?:\n|$))*)/gm)].map(
  (match) => match[1].replace(/^ {10}/gm, ""),
);

function run(script, webhook) {
  return spawnSync("bash", ["-c", script], {
    encoding: "utf8",
    env: {
      ...process.env,
      DISCORD_WEBHOOK_URL: webhook,
      GITHUB_EVENT_PATH: "/dev/null",
    },
  });
}

test("every Discord notification script skips an unconfigured webhook", () => {
  assert.equal(scripts.length, 3, "expected stable, prerelease, and issue scripts");

  for (const script of scripts) {
    const result = run(script, "");
    assert.equal(result.status, 0, result.stderr);
    assert.match(result.stdout, /::notice::Discord .* is not configured; skipping notification\./);
  }
});

test("every Discord notification script rejects a malformed non-empty webhook", () => {
  assert.equal(scripts.length, 3, "expected stable, prerelease, and issue scripts");

  for (const script of scripts) {
    const result = run(script, "not-a-discord-webhook");
    assert.equal(result.status, 1, result.stdout);
    assert.match(result.stderr, /Discord webhook endpoint is invalid/);
  }
});
