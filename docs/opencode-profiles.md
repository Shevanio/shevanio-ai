# OpenCode SDD Profiles

← [Back to README](../README.md)

---

You configured your SDD models once, and now every task -- cheap or expensive, experimental or battle-tested -- runs through the same orchestrator. Profiles fix that: **create named model configurations and switch between them with Tab inside OpenCode.**

Shevanio AI supports **two ways** of working with OpenCode profiles. Profiles cover SDD phase agents; Judgment Day agents (`jd-judge-a`, `jd-judge-b`, `jd-fix-agent`) are workflow-level slots with independent model assignments.

1. **Generated multi-profile mode** -- the classic Shevanio AI flow. The base SDD conductor is `shevanio-orchestrator`. Each named profile generates its own `shevanio-orchestrator-{name}` plus 10 suffixed SDD phase sub-agents in `opencode.json`, and you switch between them with **Tab**.
2. **External single-active mode** -- for community tools that keep profile files outside `opencode.json` and activate one runtime profile at a time.

That means you can stay with the built-in multi-profile overlay, or plug Shevanio AI into an external profile manager without the two systems fighting each other.

---


## Native background subagents

OpenCode SDD uses native OpenCode subagents through the `task` permission. Shevanio AI no longer installs the legacy `background-agents.ts` plugin by default.

Shevanio AI controls the preference with `auto`, `on`, or `off`. The same control is available to both install and sync:

```bash
shevanio-ai install --agent opencode --component sdd --opencode-background-subagents=on
shevanio-ai sync --opencode-background-subagents=off
```

The environment equivalent is `SHEVANIO_AI_OPENCODE_BACKGROUND_SUBAGENTS=auto|on|off`. Resolution precedence is:

1. CLI flag.
2. Non-empty `SHEVANIO_AI_OPENCODE_BACKGROUND_SUBAGENTS`.
3. The prior managed choice in Shevanio AI state.
4. `auto`.

In the interactive installer, OpenCode + SDD with no prior, CLI, or environment decision shows a real choice between **Enable managed background subagents** and **Keep foreground**. Prior `on` or `off` choices skip that prompt. The choice is committed only after the install succeeds; going back or cancelling leaves state unchanged.

When managed activation is enabled, Shevanio AI owns launchers under `~/.shevanio-ai/bin/` (`opencode` on POSIX, `opencode.cmd` and `opencode.ps1` on Windows). The launcher sets `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=true` only when the variable is absent, so an explicit `OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS=false` remains foreground. Restart OpenCode, and restart the shell if PATH has not refreshed, after activation.

Sessions started through `opencode serve`, `opencode attach`, or OpenCode Desktop may not inherit the managed launcher environment. Those entry points deliberately fall back to foreground execution; Shevanio AI does not rewrite server, attach, or Desktop session configuration.

Background jobs are process-local and non-durable: restarting OpenCode loses them. They provide no filesystem isolation, so do not use background work for dependent phases or writers, and never run parallel writers in one worktree.

## Quick Start (TUI)

1. Launch the installer: `shevanio-ai` (or `go run ./cmd/shevanio-ai`).
2. Select **"OpenCode SDD Profiles"** from the welcome screen.
3. Select **"Create new profile"** (or press `n`).
4. Enter a profile name in slug format (lowercase, hyphens ok). Example: `cheap`.
5. Pick the orchestrator model (provider, then model -- reuses the existing model picker).
6. Assign sub-agent models (use "Set all phases" for a uniform config, or set each phase individually).
7. Confirm -- the installer writes the profile to `opencode.json` and runs sync.

Open OpenCode and press **Tab** -- your new orchestrator appears alongside `shevanio-orchestrator`, the default OpenCode SDD conductor.

### Reasoning effort levels (per-model variants)

For models that expose reasoning effort variants (e.g. OpenAI `gpt-5` with `low`/`medium`/`high`/`xhigh`), the picker shows an extra **Select reasoning effort level** step right after you choose the model. Pick `default` to use the provider's default, or pick a specific level to lock the assignment to that effort.

The effort options are populated from a cache file written by the bundled `model-variants` OpenCode plugin at `~/.shevanio-ai/cache/model-variants.json`. The plugin runs the first time OpenCode starts after `shevanio-ai sync` and refreshes the cache on every subsequent start.

**First-run order matters:**

1. Run `shevanio-ai` (installs the plugin into `~/.config/opencode/plugins/`).
2. Run `opencode` once -- on startup the plugin queries the provider list and writes `~/.shevanio-ai/cache/model-variants.json`.
3. Re-run `shevanio-ai` and open the model picker. Reasoning models now show the effort selector.

If the JSON does not exist yet (plugin has not run, no providers expose variants, or the request failed silently), reasoning models still work -- the picker simply skips the effort step and saves the assignment with the provider default. You will not see the `[effort]` annotation next to those rows in the phase list.

## Key Names To Remember

Use this table when reviewing configs or debugging profile sync:

| Agent key | Meaning | Safe to rename manually? |
|---|---|---|
| `shevanio-orchestrator` | Canonical base OpenCode SDD conductor. All `/sdd-*` commands point here by default. | No |
| `gentle-orchestrator`, `sdd-orchestrator` | Exact legacy base aliases read for compatibility. Unowned or modified entries are preserved. | No; use the canonical key |
| `shevanio-orchestrator-{name}` | Canonical named profile conductor, such as `shevanio-orchestrator-cheap`. | No; use TUI or CLI |
| `sdd-orchestrator-{name}` | Exact legacy named-profile alias, read only for profiles in the trusted managed inventory. | No; use TUI or CLI |
| `sdd-{phase}` | Default sub-agent for a phase, such as `sdd-apply`. | No |
| `sdd-{phase}-{name}` | Named profile sub-agent, such as `sdd-apply-cheap`. | No |

## Quick Start (CLI)

Create a profile during sync with `--profile name:provider/model`:

```bash
shevanio-ai sync --profile cheap:anthropic/claude-haiku-3.5-20241022
```

Multiple profiles in one command:

```bash
shevanio-ai sync \
  --profile cheap:anthropic/claude-haiku-3.5-20241022 \
  --profile premium:anthropic/claude-opus-4-20250514
```

Override a specific phase with `--profile-phase name:phase:provider/model`:

```bash
shevanio-ai sync \
  --profile cheap:anthropic/claude-haiku-3.5-20241022 \
  --profile-phase cheap:sdd-apply:anthropic/claude-sonnet-4-20250514
```

This creates a "cheap" profile where everything runs on Haiku except `sdd-apply`, which uses Sonnet.

## External Profile Managers

If you're using a community tool that stores profiles under `~/.config/opencode/profiles/*.json` and activates them at runtime, Shevanio AI can now sync OpenCode in a compatibility mode.

### Auto-detection

On `shevanio-ai sync`, if OpenCode profile files exist under:

```text
~/.config/opencode/profiles/*.json
```

Shevanio AI automatically switches to **`external-single-active`** strategy for OpenCode sync.

### Manual override

You can also force the strategy explicitly:

```bash
shevanio-ai sync --agent opencode --sdd-profile-strategy external-single-active
```

Or force the classic generated overlay behavior:

```bash
shevanio-ai sync --agent opencode --sdd-profile-strategy generated-multi
```

### What compatibility mode does

In `external-single-active` mode, Shevanio AI:

- keeps writing the base OpenCode SDD assets and shared prompt files
- **does not** auto-regenerate suffixed named profiles from `opencode.json`
- **preserves the current `shevanio-orchestrator` prompt** during sync so external tools can keep their runtime policy / fallback blocks intact

This is the important bit: Shevanio AI still maintains the SDD foundation, but it stops acting like `opencode.json` is the source of truth for every profile.

## Using Profiles in OpenCode

After creating profiles in generated multi-profile mode, each one appears as a selectable orchestrator in OpenCode:

| What you see in Tab | What it runs |
|---|---|
| `shevanio-orchestrator` | Default profile (your original config) |
| `shevanio-orchestrator-cheap` | "cheap" profile -- Haiku everywhere |
| `shevanio-orchestrator-premium` | "premium" profile -- Opus everywhere |

Press **Tab** to cycle between orchestrators. All SDD slash commands (`/sdd-new`, `/sdd-ff`, `/sdd-explore`, etc.) run against whichever orchestrator is currently selected. The orchestrator delegates to its own suffixed sub-agents (e.g., `sdd-apply-cheap`), so profiles never interfere with each other.

If you're using an external single-active manager instead, you typically keep working with the base `shevanio-orchestrator` while the external tool swaps its active model assignments at runtime.

## Managing Profiles

From the TUI profile list screen:

| Action | Key | Notes |
|---|---|---|
| Edit a profile | `Enter` on the profile | Change models, then sync |
| Delete a profile | `d` on the profile | Removes orchestrator + all sub-agents from JSON |
| Create a new profile | `n` (or select "Create new profile") | Full creation flow |

The `default` profile (`shevanio-orchestrator`) can be edited but not deleted -- it always exists when SDD is configured.

### Profile name rules

| Input | Valid? | Reason |
|---|---|---|
| `cheap` | Yes | Simple slug |
| `premium-v2` | Yes | Hyphens allowed |
| `my profile` | No | Spaces not allowed |
| `default` | No | Reserved for the base orchestrator |
| `LOUD` | Becomes `loud` | Auto-lowercased |

---

<details>
<summary><strong>How It Works</strong></summary>

In generated multi-profile mode, each named profile generates 11 agent entries in `opencode.json`: one orchestrator (`shevanio-orchestrator-{name}`, mode `primary`) and 10 SDD phase sub-agents (`sdd-{phase}-{name}`, mode `subagent`, hidden). The base/default conductor remains `shevanio-orchestrator`. Each named profile orchestrator's permissions are scoped so it can only delegate to its own suffixed sub-agents.

Sub-agent prompts are shared across all profiles as files under `~/.config/opencode/prompts/sdd/` (e.g., `sdd-apply.md`). Each agent entry references the shared file via `{file:~/.config/opencode/prompts/sdd/sdd-apply.md}` -- only the `model` field differs between profiles. Orchestrator prompts are inlined per-profile because they contain profile-specific model assignment tables and sub-agent references.

During sync or update, Shevanio AI now uses one of two strategies:

- **`generated-multi`** -- scan `opencode.json` for canonical named-profile actors (plus trusted exact legacy aliases), update shared prompts, regenerate profile orchestrators, preserve model assignments, and keep `shevanio-orchestrator` as the canonical base conductor
- **`external-single-active`** -- detect external profile files, keep the shared SDD assets current, and preserve the existing `shevanio-orchestrator` prompt instead of overwriting external runtime extensions

</details>

---

← [Back to README](../README.md)
