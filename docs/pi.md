# Shevanio Pi Integration

← [Back to README](../README.md)

Shevanio AI delivers Pi support through the **Shevanio Pi** package/runtime harness. The canonical source is [Shevanio/shevanio-pi](https://github.com/Shevanio/shevanio-pi), while the current stable Shevanio AI installer still uses published compatibility packages until the canonical npm names become available.

## Quick Start

1. Install Pi and make sure `pi` is available on `PATH`.
2. Install the Pi support stack from Shevanio AI:

```bash
shevanio-ai install --agent pi
```

3. Start Pi in your project:

```bash
pi
```

Shevanio AI detects the `pi` binary first. If Pi is the only selected agent, the installer still provisions the real Engram component, but skips persona, ecosystem component selection, and Strict TDD prompts because the Pi harness owns those choices inside Pi.

## Current Compatibility Installation

The canonical npm packages `shevanio-pi` and `shevanio-engram` are not published. Do not substitute those names into installation commands yet. The current executable installation contract remains:

Shevanio AI runs exactly these Pi setup steps:

```bash
pi install npm:gentle-pi
pi install npm:gentle-engram
pi install npm:pi-mcp-adapter
npm exec --yes --package gentle-engram@latest -- pi-engram init
pi install npm:pi-subagents-j0k3r
pi install npm:@juicesharp/rpiv-ask-user-question
pi install npm:pi-web-access
pi install npm:@juicesharp/rpiv-todo
pi install npm:pi-btw
```

| Package                                                  | What it adds                                                                                                              |
| -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| [`gentle-pi`](https://www.npmjs.com/package/gentle-pi)   | Transitional distribution of the Shevanio Pi harness: persona, SDD/OpenSpec, strict TDD, safety, skills, agents, and chains. |
| [`gentle-engram`](https://www.npmjs.com/package/gentle-engram) | Transitional Pi integration for [shevanio-engram](https://github.com/Shevanio/shevanio-engram). It is not the Engram binary itself. |
| `pi-mcp-adapter`                                         | Lets Pi expose MCP servers, including Engram, through Pi's MCP runtime.                                                   |
| `pi-engram init`                                         | Initializes the Pi Engram MCP config shape owned by `gentle-engram`.                                                      |
| `pi-subagents-j0k3r`                                      | Runs SDD agents discovered from `.pi/agents/`; installed from the published Pi package `npm:pi-subagents-j0k3r`.                 |
| `@juicesharp/rpiv-ask-user-question`                     | Lets Pi child agents ask the active user session for clarification when they need human input.                            |
| `pi-web-access`                                          | Adds web access tools for Pi.                                                                                             |
| `@juicesharp/rpiv-todo`                                  | Adds todo/task tracking support for Pi sessions.                                                                          |
| `pi-btw`                                                 | Adds BTW companion workflow support for Pi.                                                                               |

The registry artifacts verified for this guide were `gentle-pi@2.3.0` and `gentle-engram@0.1.10`. The installer intentionally follows the unpinned commands above, so source code—not these observed versions—remains the executable contract.

The transitional Pi package owns Pi's runtime behavior. Its current harness delegates context-heavy exploration, uses one writer for multi-file changes, follows runtime-supplied review instructions when available, investigates operational incidents before resuming, and pauses long monolithic sessions before they drift.

The real Engram component is provisioned separately by Shevanio AI so the Pi integration has an Engram runtime to talk to.
During that Engram provisioning step, Shevanio AI declares `npm:pi-mcp-adapter` in Pi's agent settings and adds the npm dependency. Existing unrelated Pi settings, package entries, and npm dependencies are preserved.

Files updated by Shevanio AI's Engram provisioning:

```text
.pi/agent/settings.json    # packages includes npm:pi-mcp-adapter
.pi/npm/package.json       # dependencies.pi-mcp-adapter = ^2.6.0
```

The transitional `gentle-engram` package owns the MCP schema used by the current installer. `pi-engram init` initializes Pi's Engram MCP config under the Pi agent config directory instead of having Shevanio AI hand-write that file.

## Optional CodeGraph

Select CodeGraph during Shevanio AI installation to add its read-only MCP server to Pi. This integration is optional and owned entirely by Shevanio AI; the Pi package is not modified.

| Area | Shevanio AI behavior |
| --- | --- |
| MCP | Merges `mcpServers.codegraph` with `codegraph serve --mcp`; a conflicting user entry is reported, never overwritten. |
| Children | Discovers effective user and project Pi child definitions. Compatible children (`bash` plus explicit tools) receive `mcp`; every readable child receives lazy-init guidance. |
| Intelligence | Prefers `codegraph_explore`; when MCP is unavailable, guidance uses the upstream CLI's read-only intelligence commands directly rather than routing them through Shevanio AI. |
| Indexes | Guidance resolves a safe project root, initializes a missing `.codegraph/` once, relies on watcher auto-sync after edits, and uses `codegraph sync` only for stale/disabled-watcher recovery. Full rebuild and destructive/admin commands are excluded from routine agent use. |
| Sync | `shevanio-ai sync` reconciles the owned manifest after Pi assets refresh, restoring missing overlays without duplicates. This configuration sync is separate from upstream index freshness. |
| Removal | Uninstall removes only manifest-owned MCP and child blocks. Drifted child files are preserved and reported for manual review. |

Package-owned child files are never edited. Shevanio AI creates a same-name overlay in Pi's agent directory when needed. A parent `APPEND_SYSTEM.md` CodeGraph marker is not considered proof that any child has CodeGraph tools or guidance.

## Commands Available After the Current Install

Run these inside Pi after installing the current compatibility package stack:

| Command                          | What it does                                                                                                    |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `/gentle:status` | Shows package, SDD asset, OpenSpec, and global model config status. |
| `/gentle:doctor` | Runs read-only diagnostics for assets, config, tools, memory, and guards. |
| `/gentle:persona` | Switches between the current `gentleman` and `neutral` modes. |
| `/gentle:models` | Opens the Pi-native model and effort assignment modal. |
| `/gentle:background-subagents` | Shows or changes the user-owned background-subagent policy. |
| `/gentle-sdd-init` | Initializes or refreshes `openspec/config.yaml`. |
| `/gentle:install-sdd` | Repairs missing global SDD assets without overwriting files. |
| `/gentle:install-sdd --force` | Force-refreshes package-owned global SDD assets. |

Canonical Shevanio Pi source uses `/shevanio-pi:*` commands and canonical `.pi/shevanio-pi` configuration paths. Those commands belong to the unpublished canonical package and are not a substitute for `/gentle:*` in the current stable installer. The canonical package preserves selected `gentle-pi.*`, `.pi/gentle-ai`, and `GENTLE_PI_*` values as compatibility surfaces.

## Persona Selection

Pi persona selection belongs to the currently installed Pi package, not the Shevanio AI installer.

```text
/gentle:persona
```

| Persona     | Behavior                                                                                                                   |
| ----------- | -------------------------------------------------------------------------------------------------------------------------- |
| `gentleman` | Teaching-oriented senior architect persona with Rioplatense Spanish/voseo when the user writes Spanish.                    |
| `neutral`   | Same senior architect discipline and teaching philosophy, but with warm professional language and no regional expressions. |

The current compatibility package saves the global selection at:

```text
~/.pi/gentle-ai/persona.json
```

Run `/reload` or start a new Pi session after switching if the current session already injected the previous persona.

## Model Assignments

Pi model assignment belongs to the currently installed Pi package, not the Shevanio AI installer.

```text
/gentle:models
```

The modal discovers project, user, and built-in agents. SDD agents are shown first so you can tune the phases that matter most.

| Agent kind                     | Recommended model shape                                              |
| ------------------------------ | -------------------------------------------------------------------- |
| Exploration, proposal, archive | Fast and cheap is usually enough.                                    |
| Spec, design, tasks            | Strong reasoning model, because these phases shape implementation.   |
| Apply                          | Strong coding model with reliable tool use.                          |
| Verify / review agents         | Strong fresh-context model. Verification benefits from independence. |
| Tiny utility agents            | Inherit the active/default model unless they become a bottleneck.    |

Saved global config:

```text
~/.pi/gentle-ai/models.json
```

Applied configuration:

```text
.pi/subagents.json
~/.pi/agent/subagents.json
```

Use `Inherit active/default model` to remove an agent override.

## Project Files

On normal Pi `session_start`, the current package ensures package-owned global assets without overwriting local edits:

```text
~/.pi/agent/agents/sdd-*.md
~/.pi/agent/chains/sdd-*.chain.md
~/.pi/agent/gentle-ai/support/strict-tdd*.md
```

Project-local `.pi/agents/` and `.pi/chains/` files remain manual overrides. Use `/gentle:install-sdd --force` only when you explicitly want to replace package-owned global SDD assets.

If you start Pi with `pi -ns`, Pi skips startup skill loading/hooks. That mode is useful for a clean or faster Pi session, but it also means package startup work such as asset checks and skill-registry refreshes will not run automatically.

## Troubleshooting

| Symptom                                                | Fix                                                                                                                                                                  |
| ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Shevanio AI says Pi is missing                           | Install Pi first and make sure `pi` is on `PATH`.                                                                                                                    |
| SDD agents are missing in Pi                           | Start Pi normally so the package can run `session_start`, or run `/gentle:install-sdd`. If you used `pi -ns`, startup hooks were skipped.                         |
| Persona did not change immediately                     | Run `/reload` or start a new Pi session.                                                                                                                             |
| Model override should be removed                       | Open `/gentle:models` and choose `Inherit active/default model`.                                                                                                    |
| Memory tools or `/mcp` are missing                     | Re-run `shevanio-ai install --agent pi` to refresh `.pi/agent/settings.json`, `.pi/npm/package.json`, and `pi-engram init`, then check `/gentle:status`.              |
| `gentle-engram` is installed but Engram is unavailable | Re-run `shevanio-ai install --agent pi` so the real Engram component is provisioned.                                                                                   |

## Next Steps

- Read the canonical [Shevanio Pi guide](https://github.com/Shevanio/shevanio-pi#readme) for source-level behavior and publication status.
- Read [Supported Agents](agents.md) for the full agent matrix.
- Read [Engram Commands](engram.md) if you want to inspect or sync persistent memory.
- Read [Usage](usage.md) for the general Shevanio AI CLI and TUI flow.

← [Back to README](../README.md)
