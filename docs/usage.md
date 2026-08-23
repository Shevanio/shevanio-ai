# Usage

← [Back to README](../README.md)

---

## Persona Modes

| Persona   | ID          | Description                                                                       |
| --------- | ----------- | --------------------------------------------------------------------------------- |
| Gentleman | `gentleman` | Teaching-oriented mentor persona — pushes back on bad practices, explains the why |
| Neutral   | `neutral`   | Same teacher, same philosophy, no regional language — warm and professional       |
| Custom    | `custom`    | Keep your existing persona/config unmanaged — shevanio-ai does not inject a persona |

`custom` is a compatibility/ownership choice, not a persona editor. Use it when you already have your own persona instructions and want shevanio-ai to leave them alone.

---

## Interactive TUI

Just run it — the Bubbletea TUI guides you through agent selection, components, skills, presets, and managed uninstall flows:

```bash
shevanio-ai
```

The uninstall flow is also available from the TUI menu. It lets you:

- select one or more configured agents
- select which managed components to remove (for example `sdd`, `persona`, or `context7`)
- confirm the exact uninstall scope before applying changes

Before any managed file is modified, `shevanio-ai` creates a backup snapshot so the configuration can be restored later if needed.

---

## CLI Commands

### install

First-time setup — detects your tools, configures agents, injects all components. When installing a single agent with `--agent X`, shevanio-ai **merges** the new agent into the existing `installed_agents` list in `state.json` and **preserves** any existing `model_assignments` — it does not overwrite the full state.

```bash
# Full ecosystem for multiple agents
shevanio-ai install \
  --agent claude-code,opencode,gemini-cli \
  --preset full-gentleman

# Minimal setup for Cursor
shevanio-ai install \
  --agent cursor \
  --preset minimal

# OpenClaw setup after installing OpenClaw manually
shevanio-ai install \
  --agent openclaw \
  --preset full-gentleman

# Pick specific components and skills
shevanio-ai install \
  --agent claude-code \
  --component engram,sdd,skills,context7,persona,permissions \
  --skill go-testing,skill-creator,branch-pr,issue-creation \
  --persona gentleman

# Dry-run first (preview plan without applying changes)
shevanio-ai install --dry-run \
  --agent claude-code,opencode \
  --preset full-gentleman
```

### skill-registry refresh

Refresh the project-local skill registry used by orchestrators before they delegate work:

```bash
shevanio-ai skill-registry refresh
shevanio-ai skill-registry refresh --force
shevanio-ai skill-registry refresh --cwd /path/to/project --quiet
```

The command scans project skills first (`skills/`, `.opencode/skills/`, `.claude/skills/`, `.github/skills/`, and other supported workspace skill roots), then global agent skill directories. Project-local skills win over same-name global skills.

The command writes `.atl/skill-registry.md` and `.atl/.skill-registry.cache.json`. The cache fingerprint includes schema version plus each discovered `SKILL.md` file path, mtime, and size, so normal startup is a cheap cache-hit when skills have not changed.

Codex, Claude Code, and OpenCode installs wire this command into startup/plugin hooks. Pi gets the equivalent behavior from `gentle-pi`; keep those hook/plugin scan roots in sync when changing these discovery rules.

See [Skill Registry](skill-registry.md) for the full index-first flow and diagrams.

### sync

Refresh managed assets to the current version. Run it after replacing or upgrading the `shevanio-ai` binary, including with `brew upgrade`, `shevanio-ai upgrade`, or `go install`. It does NOT reinstall binaries (engram, GGA) — only updates prompt content, skills, MCP configs, and SDD orchestrators.

Managed reviewer and runtime assets are version-bound to the binary. Until sync succeeds, review lifecycle operations fail closed when managed writer provenance is missing or mismatched.

> **Important:** `shevanio-ai sync` updates the agents recorded as installed by Shevanio AI, not every AI agent config directory on your machine.
>
> Shevanio AI stores your selected install targets in `~/.shevanio-ai/state.json`. Future `sync` runs use that stored selection so Shevanio AI does not accidentally write into tools you did not choose to manage. If you rerun install and select only one agent, that new selection becomes the default sync scope.
>
> Before syncing, you can preview the active scope with `shevanio-ai sync --dry-run`. If you want to sync agents outside the stored selection, pass them explicitly with `--agent`.

```bash
# Preview which agents sync will update
shevanio-ai sync --dry-run

# Sync the agents currently registered in ~/.shevanio-ai/state.json
shevanio-ai sync

# Sync specific agents only
shevanio-ai sync --agent claude-code --agent opencode

# Refresh OpenClaw workspace instructions and MCP config
shevanio-ai sync --agent openclaw
```

Sync is safe and idempotent — running it twice produces no changes the second time. When files change, the summary reports the changed file count and lists the changed file paths.

`sync` refreshes the managed component set for the selected agents. It does not support `--component`; use `--include-permissions` or `--include-theme` for the opt-in components that are excluded from the default sync scope.

For OpenClaw, sync reads the active workspace from `~/.openclaw/openclaw.json` (`agents.defaults.workspace`). It writes `AGENTS.md` / `SOUL.md` into that workspace, while MCP servers stay in the global OpenClaw config under `mcp.servers`.

For Hermes, shevanio-ai is detect-only: it cannot install Hermes. Install Hermes manually first. Detection is driven by the `~/.hermes` config directory (the binary being on `PATH` is reported separately). Once Hermes is detected, `shevanio-ai install --agent hermes` injects context7 and Engram MCP blocks into `~/.hermes/config.yaml`, writes the SDD orchestrator and persona into `~/.hermes/SOUL.md`, and copies skills to `~/.hermes/skills/`. Use `shevanio-ai sync --agent hermes` to update the managed configuration after upgrades.

### uninstall

Remove only the `shevanio-ai` managed configuration from one or more agents. This does not uninstall external packages or binaries — it removes managed prompt sections, MCP entries, skills/config fragments, and other managed files, then updates `state.json` accordingly.

Before any change is applied, `shevanio-ai` creates a backup snapshot of the affected files.

```bash
# Partial uninstall for specific agents
shevanio-ai uninstall \
  --agent claude-code \
  --agent opencode

# Partial uninstall for specific components only
shevanio-ai uninstall \
  --agent claude-code \
  --component sdd,persona,context7

# Complete uninstall of managed config from all supported agents
shevanio-ai uninstall --all

# Skip confirmation prompt
shevanio-ai uninstall --agent cursor --component skills --yes
```

If no `--component` flag is provided for a partial uninstall, `shevanio-ai` removes all managed uninstallable components for the selected agent set.

### update / upgrade

Upgrade `shevanio-ai` with the same method used to install it. The official Homebrew formula and signed release binaries carry production trust anchors. Binaries produced by `go install` or a local source build do not, so their binary self-upgrader fails closed by design.

The pre-upgrade backup snapshot covers only the agents recorded in `state.InstalledAgents` (`~/.shevanio-ai/state.json`), not every agent config directory that exists on your machine.

**Homebrew:**

```bash
brew upgrade shevanio-ai
shevanio-ai sync
shevanio-ai doctor
```

**Official signed binary:**

```bash
# Check if a newer version is available
shevanio-ai update

# Upgrade to the latest release (downloads new binary, replaces current)
shevanio-ai upgrade
shevanio-ai sync
shevanio-ai doctor
```

**Go or source:**

```bash
go install github.com/shevanio/shevanio-ai/v2/cmd/shevanio-ai@latest
shevanio-ai sync
shevanio-ai doctor
```

For a local repository build, check out the intended release and rebuild into the same binary path instead of running `go install`, then run `shevanio-ai sync` and `shevanio-ai doctor`.

If GitHub rate-limits update checks, export `GITHUB_TOKEN` or `GH_TOKEN` before running `shevanio-ai update`/`upgrade`.

If Homebrew 6 refuses the external tap, follow the [formula-scoped trust guidance](#homebrew-upgrade-troubleshooting) rather than trusting the entire tap.

**Self-update prompt behavior** (changed in v1.x slice 5 — `SHEVANIO_AI_CONFIRM_UPDATE` removed):

| Situation | Behavior |
|-----------|----------|
| Interactive terminal (TTY) | Always prompts `Apply now? [Y/n]`. Empty Enter accepts. |
| Non-TTY (CI, pipe, script) | Auto-declines — never hangs. |
| `SHEVANIO_AI_YES=1` | Auto-accepts without prompting (for scripted upgrades). This variable is inherited by subprocesses, so scope it to a single invocation when needed (e.g. `SHEVANIO_AI_YES=1 shevanio-ai …`). |
| `SHEVANIO_AI_NO_SELF_UPDATE=1` | Skips the self-update check entirely. |

`SHEVANIO_AI_CONFIRM_UPDATE` was removed in slice 5. It is now ignored if set.

`SHEVANIO_AI_SELF_UPDATE_DONE` is an internal loop guard and should not be set manually.

### transition compatibility

`SHEVANIO_AI_*` variables have priority. When a supported canonical variable is absent, the runtime imports the corresponding `GENTLE_AI_*` value for one transition window; an explicitly empty canonical variable still wins. The removed `GENTLE_AI_CONFIRM_UPDATE` switch remains ignored and has no Shevanio replacement.

State is written only under `~/.shevanio-ai`. If `~/.shevanio-ai/state.json` is absent, the runtime can read `~/.gentle-ai/state.json`; a later write creates the new state file and never deletes or modifies the legacy file. Managed marker blocks similarly accept `gentle-ai:*` input and rewrite only canonical `shevanio-ai:*` markers.

### model assignment

The TUI **Configure Models** screen can assign different models to SDD phases, `sdd-onboard`, and Judgment Day agents (`jd-judge-a`, `jd-judge-b`, `jd-fix-agent`) when the selected agent supports those slots. This lets you keep review or apply phases on stronger models while routing cheaper phases to faster models.

### doctor

Read-only ecosystem health diagnostics — no changes made to your configuration:

```bash
shevanio-ai doctor
```

Checks performed:

| Check | What it verifies |
|-------|-----------------|
| Tool binaries | Required tools present on `PATH`; shadow detection (wrong binary resolves first) |
| `state.json` validity | Parses `~/.shevanio-ai/state.json` and reports any schema/corruption issues |
| Engram MCP reachability | Confirms the Engram MCP server responds |
| Disk space | Warns when available space is critically low |

Each check reports **pass**, **warn**, or **fail** with an optional remedy hint. Run `doctor` first when troubleshooting an unexpected install or sync result.

### version

```bash
shevanio-ai version
shevanio-ai --version
shevanio-ai -v
```

---

## CLI Flags (install)

| Flag                          | Description                                                                                                       |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `--agent`, `--agents`         | Agents to configure (comma-separated)                                                                             |
| `--component`, `--components` | Components to install (comma-separated)                                                                           |
| `--skill`, `--skills`         | Skills to install (comma-separated)                                                                               |
| `--persona`                   | Persona mode: `gentleman`, `neutral`, `custom` (`custom` keeps your existing persona unmanaged)                   |
| `--preset`                    | Preset: `full-gentleman`, `ecosystem-only`, `minimal`, `custom` (`custom` means manual component/skill selection) |
| `--sdd-mode`                  | SDD orchestrator mode: `single` or `multi`                                                                        |
| `--scope`                     | Install scope for agent-scoped files: `global` (default, writes to each selected agent's global config directory) or `workspace` (writes to the current project root). Also settable via `SHEVANIO_AI_INSTALL_SCOPE` env var for CI/non-interactive use. |
| `--dry-run`                   | Preview the install plan without applying changes                                                                 |

## CLI Flags (sync)

| Flag                     | Description                                                                                          |
| ------------------------ | ---------------------------------------------------------------------------------------------------- |
| `--agent`, `--agents`    | Agents to sync (defaults to all installed agents)                                                    |
| `--skill`, `--skills`    | Skills to sync (comma-separated; defaults to selected preset skills)                                  |
| `--sdd-mode`             | SDD orchestrator mode: `single` or `multi`                                                           |
| `--strict-tdd`           | Enable Strict TDD Mode for SDD agents                                                                |
| `--profile`              | Create or update an SDD profile: `name:provider/model` (sets the default model for all phases)       |
| `--profile-phase`        | Override a specific phase in a profile: `name:phase:provider/model`                                  |
| `--sdd-profile-strategy` | OpenCode profile sync strategy: `generated-multi` or `external-single-active`                        |
| `--include-permissions`  | Include permissions sync (opt-in)                                                                    |
| `--include-theme`        | Include theme sync (opt-in)                                                                          |
| `--dry-run`              | Preview the sync plan without applying changes                                                       |

**Profile examples:**

```bash
# Create a "cheap" profile using a free model for all phases
shevanio-ai sync --profile cheap:openrouter/qwen/qwen3-30b-a3b:free

# Override the design phase to use a stronger model
shevanio-ai sync --profile-phase cheap:sdd-design:anthropic/claude-sonnet-4-20250514

# Create multiple profiles in one command
shevanio-ai sync \
  --profile cheap:openrouter/qwen/qwen3-30b-a3b:free \
  --profile premium:anthropic/claude-sonnet-4-20250514

# Use compatibility mode with an external OpenCode profile manager
shevanio-ai sync --agent opencode --sdd-profile-strategy external-single-active
```

See [OpenCode SDD Profiles](opencode-profiles.md) for the full guide.

## CLI Flags (uninstall)

| Flag                          | Description                                                             |
| ----------------------------- | ----------------------------------------------------------------------- |
| `--agent`, `--agents`         | Agents to uninstall managed config from (required unless using `--all`) |
| `--component`, `--components` | Managed components to remove only from the selected agents              |
| `--all`                       | Remove managed configuration from all supported agents                  |
| `--yes`, `-y`                 | Skip the confirmation prompt                                            |

---

## Typical Workflow

```bash
# First time: use the locally installed binary
shevanio-ai install --agent claude-code,cursor --preset full-gentleman

# After a new release: upgrade + sync
brew upgrade shevanio-ai
shevanio-ai sync
shevanio-ai doctor

# Remove only managed SDD + persona config from one agent
shevanio-ai uninstall --agent claude-code --component sdd,persona

# Adding a new agent later
shevanio-ai install --agent windsurf --preset full-gentleman
```

### Homebrew upgrade troubleshooting

The published Shevanio AI formula is available from `shevanio/tap`. Homebrew 6 can require explicit trust for non-official taps and, on Linux, can sandbox builds with Bubblewrap. `shevanio-ai upgrade` and `scripts/install.sh` auto-trust only the Shevanio AI formula, but manual installs or upgrades may still need this one-time command:

```bash
brew trust --formula shevanio/tap/shevanio-ai
brew upgrade shevanio-ai
```

On Linux, if Homebrew reports that Bubblewrap cannot create a rootless sandbox,
there is nothing for Shevanio AI to install: Bubblewrap is already present, but the
host blocks the rootless namespace primitives it needs. This is a security
tradeoff and should be an explicit admin decision. If your policy allows it,
fix the host namespace policy first:

```bash
sudo sysctl -w kernel.unprivileged_userns_clone=1
sudo sysctl -w user.max_user_namespaces=28633
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0 || true
```

Use `HOMEBREW_NO_SANDBOX_LINUX=1 brew upgrade shevanio-ai` only as a final
workaround when your distro policy forbids the namespace settings; it disables
Homebrew's Linux sandbox for that command.


---

## Dependency Management

`shevanio-ai` auto-detects prerequisites before installation and provides platform-specific guidance:

- **Detected tools**: git, curl, node, npm, brew, go
- **Version checks**: validates minimum versions where applicable
- **Platform-aware hints**: suggests `brew install`, `apt install`, `pacman -S`, `dnf install`, or `winget install` depending on your OS
- **Node LTS alignment**: on apt/dnf systems, Node.js hints use NodeSource LTS bootstrap before package install
- **Dependency-first approach**: detects what's installed, calculates what's needed, shows the full dependency tree before installing anything, then verifies each dependency after installation
