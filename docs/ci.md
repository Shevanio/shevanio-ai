# CI Operations

Shevanio AI separates pull-request gates, full Windows regression coverage, releases, and optional notifications so each workflow has one clear responsibility.

## Quick path

Before opening a pull request, run the checks that match your change:

```bash
go run ./internal/gofmtcheck
go test ./...
node --test .github/scripts/*.test.cjs
```

Run `RUN_FULL_E2E=1 RUN_BACKUP_TESTS=1 ./e2e/docker-test.sh` when installation behavior changes. Review-lifecycle and benchmark changes have additional commands in [CONTRIBUTING.md](../CONTRIBUTING.md).

## Workflow map

| Workflow | Triggers | Responsibility |
|---|---|---|
| `CI` | Pull requests, pushes to `main`, daily at 03:00 UTC, manual dispatch | Format, unit, benchmark, platform-runtime, deterministic real-agent, and Linux-distribution E2E checks. |
| `PR Validation` | Pull request open, edit, synchronization, label, or unlabel events | Review budget, issue linkage and approval, exactly one `type:*` label, and workflow-script tests. |
| `Windows Full Suite` | Pushes to `main`, daily at 03:20 UTC, manual dispatch | All Go packages on native Windows, split into ten cost-balanced shards. |
| `Release` | Stable `v*` tags without a hyphen; reusable workflow call | Exact-commit CI gate, signing preflight, tests, and protected publication. |
| `Release candidate` | `v*-rc.*` tags | Calls `Release` with the RC channel and skips Homebrew publication. |
| `Promote stable RC` | Manual dispatch | Re-verifies an immutable RC and promotes it through the protected release environment. |
| `Discord Notifications` | Published releases; issue open, close, and reopen events | Sends GitHub event payloads only when the corresponding optional webhook is configured. |

## Required pull-request checks

The protected `main` branch currently requires these exact check names:

- `Go Format`
- `Unit Tests`
- `Claude Network-None Runtime (Required)`
- `Darwin Runtime`
- `Windows Runtime`
- `Organic Runtime E2E (ubuntu-latest)`
- `Organic Runtime E2E (windows-latest)`
- `E2E Tests (ubuntu)`
- `E2E Tests (arch)`
- `E2E Tests (fedora)`
- `Check PR Cognitive Load`
- `Check Workflow Scripts`
- `Check Issue Reference`
- `Check Issue Has status:approved`
- `Check PR Has type:* Label`

`Windows Full Suite` is not a pull-request check. The required `Windows Runtime` job runs the curated release-blocker set on every pull request; the full suite supplies post-merge, scheduled, and manually dispatched regression evidence.

## Release gate

The stable and RC paths share `.github/workflows/release.yml`. Before publication, `scripts/require-ci-success.sh` requires the newest `CI` workflow run for the exact release commit to be complete and successful. Release jobs then verify the tag, trust anchors, provider-contract bundle, repository state, and signing policy before the protected publication job receives write permission.

Use [Release signing and key rotation](release-signing.md) for the release procedure. Do not move a tag past a failing commit or treat a different commit's green run as evidence.

## Optional Discord webhooks

The notification workflow recognizes three repository secrets:

| Secret | Event |
|---|---|
| `DISCORD_SHEVANIO_AI_RELEASES_WEBHOOK` | Stable release publication |
| `DISCORD_SHEVANIO_AI_BETA_WEBHOOK` | Prerelease publication |
| `DISCORD_SHEVANIO_AI_ISSUES_WEBHOOK` | Issue opened, closed, or reopened |

These secrets are optional. An empty value emits a GitHub Actions notice and succeeds without sending a request. A non-empty value must match a Discord HTTPS webhook endpoint; malformed values fail the job. Valid endpoints are masked before `curl` sends the event payload.

## Investigate a failure

1. Open the failing job and identify the first failed command, not only the final job conclusion.
2. Reproduce that command locally where the required platform is available.
3. Distinguish deterministic product failures from runner or provider incidents using the logs and a rerun on the same SHA.
4. Fix the cause on a new commit. Never make a required check optional to hide a deterministic failure.

Use `workflow_dispatch` when a workflow needs a fresh run on unchanged `main`. A release must still observe a successful `CI` run for its exact commit.
