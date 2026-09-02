# Components, Skills & Presets

← [Back to README](../README.md)

---

## Components

| Component | ID | Description |
|-----------|-----|-------------|
| Engram | `engram` | Persistent cross-session memory via MCP — project detection, search, git sync, and consolidation. See [Engram Commands](engram.md). |
| SDD | `sdd` | Spec-Driven Development workflow and native orchestration. It is explicit and optional; see [Intended Usage](intended-usage.md). |
| Skills | `skills` | Curated coding skill library |
| Context7 | `context7` | MCP server for live framework/library documentation |
| Persona | `persona` | Managed Shevanio/neutral persona injection, or unmanaged custom persona mode |
| Permissions | `permissions` | Security-first defaults and guardrails. Applied to Claude Code and OpenCode (the two adapters with permissions overlay support). Default sensitive-paths deny list: `~/.ssh/*`, `~/.ssh/**/*`, `**/*.pem`, `**/*.key`, `**/.env*`, `~/.credentials/*`, `~/.aws/credentials`, `~/.config/gh/hosts.yml`, `~/Library/Keychains/*`, `**/secrets/*`, `**/*.p12`, `**/*.pfx` |
| OpenCode Theme | `theme` | OpenCode visual theme overlay |
| Claude Code Theme | `claude-theme` | Claude Code visual theme overlay |
| OpenCode Logo | `opencode-gentle-logo` | Managed OpenCode home-logo plugin |

---

## Skills

### Included Skills (installed by shevanio-ai)

24 skill files organized by category, embedded in the binary and injected into your agent's configuration:

#### SDD (Spec-Driven Development)

| Skill | ID | Description |
|-------|-----|-------------|
| SDD Init | `sdd-init` | Bootstrap SDD context in a project |
| SDD Explore | `sdd-explore` | Investigate codebase before committing to a change |
| SDD Propose | `sdd-propose` | Create change proposal with intent, scope, approach |
| SDD Spec | `sdd-spec` | Write specifications with requirements and scenarios |
| SDD Design | `sdd-design` | Technical design with architecture decisions |
| SDD Tasks | `sdd-tasks` | Break down a change into implementation tasks |
| SDD Apply | `sdd-apply` | Implement tasks following specs and design |
| SDD Verify | `sdd-verify` | Validate implementation matches specs |
| SDD Archive | `sdd-archive` | Sync delta specs to main specs and archive |
| SDD Onboard | `sdd-onboard` | Guided end-to-end SDD walkthrough on the real codebase |
| Judgment Day | `judgment-day` | Parallel adversarial review — two independent judges review the same target |

#### Foundation

| Skill | ID | Description |
|-------|-----|-------------|
| Go Testing | `go-testing` | Go testing patterns including Bubbletea TUI testing |
| Shevanio AI Bench | `shevanio-ai-bench` | Verify the real-agent journey corpus and driven execution |
| Skill Creator | `skill-creator` | Create new AI agent skills following the Agent Skills spec |
| Skill Improver | `skill-improver` | Audit and improve existing skills against the repository style guide |
| Branch & PR | `branch-pr` | PR creation workflow with conventional commits, branch naming, and issue-first enforcement |
| Issue Creation | `issue-creation` | Issue filing workflow with bug report and feature request templates |
| Skill Registry | `skill-registry` | Build an index of installed skills with triggers, scopes, and exact `SKILL.md` paths |
| Chained PR | `chained-pr` | Plan and create reviewable stacked/chained pull requests |
| Cognitive Doc Design | `cognitive-doc-design` | Write docs that reduce review and onboarding cognitive load |
| Comment Writer | `comment-writer` | Draft warm, direct collaboration comments and review replies |
| Work Unit Commits | `work-unit-commits` | Split implementation into reviewable work units |
| RDD Defect Workflow | `rdd-defect-workflow` | Guide receipt-driven defect work with truthful evidence and authority boundaries |
| Systemic Issue Triage | `systemic-issue-triage` | Group issues by root cause and shrink the system |

These foundation skills are installed by default with both the `full-shevanio` (Dev Stack + Polish) and `ecosystem-only` (Dev Stack) presets.

### Coding Skills (separate repository)

For framework-specific skills (React 19, Angular, TypeScript, Tailwind 4, Zod 4, Playwright, etc.), see [Gentleman-Programming/Gentleman-Skills](https://github.com/Gentleman-Programming/Gentleman-Skills). These are maintained by the community and installed separately by cloning the repo and copying skills to your agent's skills directory.

---

## Presets

| Preset | ID | What's Included |
|--------|-----|-----------------|
| Dev Stack + Polish | `full-shevanio` | Dev Stack plus Persona, Permissions, Claude Code theme, and OpenCode logo |
| Dev Stack | `ecosystem-only` | Engram + SDD + Skills + Context7 + all bundled skills |
| Memory Only | `minimal` | Engram + SDD skills only |
| Custom | `custom` | You choose components and skills manually while keeping any existing persona/settings unmanaged |

Persona is selected separately on the Persona screen and applied independently of the preset.

## Optional integrations

CodeGraph is a community tool, not an installable component. When selected,
Shevanio AI installs or reconciles the external CLI and managed agent guidance;
the external runtime owns indexing and MCP behavior. OpenCode community plugins
are similarly registered by name, while OpenCode owns loading and execution.
See [Integrations](codebase/integrations.md) for ownership boundaries.
