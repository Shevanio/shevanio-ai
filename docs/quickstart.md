# Quickstart

## Install on macOS or Linux

Homebrew is the recommended installation path for the stable v2.5.0 release:

```bash
brew install shevanio/tap/shevanio-ai
shevanio-ai version
```

For official signed archives, artifact verification, or Go installation alternatives, see the [README installation guide](../README.md#other-installation-methods).

On Linux, Homebrew uses Bubblewrap for sandboxing. If the host blocks unprivileged user namespaces, see [Homebrew upgrade troubleshooting](usage.md#homebrew-upgrade-troubleshooting).

## Install on Windows

Windows 10/11 uses Go 1.25.10 or newer. Install the canonical stable module version:

```powershell
go install github.com/shevanio/shevanio-ai/v2/cmd/shevanio-ai@v2.5.0
shevanio-ai version
```

Ensure `%USERPROFILE%\go\bin` is on `PATH`. Official Windows archives and Scoop remain unavailable until trusted Authenticode signing is in place.

## Prerequisites

- An existing supported AI agent runtime. Shevanio AI configures agents; it does not install them.
- Node.js 18+ and npm for ecosystem installation. Missing prerequisites produce platform-specific guidance.
- Git 2.38+ for project-aware workflows.
- Homebrew for the recommended macOS/Linux path, or Go 1.25.10+ for Windows and Go installation.
- `pi` on `PATH` if you select the Pi agent.

## First use

Use the installed binary. Preview the plan before applying it, then run the read-only health check:

```bash
shevanio-ai install --dry-run
shevanio-ai install
shevanio-ai doctor
```

The installer detects the platform automatically. The dry run reports the detected OS, Linux distribution, package manager, support status, selected agents, and planned writes.

The agents selected during install become the default scope for future `sync` runs and are recorded in `~/.shevanio-ai/state.json`. Preview that scope after an upgrade with:

```bash
shevanio-ai sync --dry-run
```

To update a different set explicitly, pass every target agent:

```bash
shevanio-ai sync --agent claude-code --agent opencode
```

When checks pass, the installer reports:

`You're ready. Run 'claude' or 'opencode' and start building.`

## Contributor development

Running directly from a source checkout is for contributor and development work only, not end-user installation:

```bash
go run ./cmd/shevanio-ai install --dry-run
```

End users choosing Go should use the pinned module command above. Go and local source builds do not embed production release trust anchors, so update them by reinstalling or rebuilding rather than using binary self-upgrade.

For a Pi-only install, the plan shows the Pi package stack instead of Shevanio AI components. The current stable installer uses the published `gentle-pi` and `gentle-engram` compatibility names, adds `pi-mcp-adapter`, runs `pi-engram init`, then installs the documented companion packages. The canonical `shevanio-pi` and `shevanio-engram` npm names are not published; see the [Pi integration guide](pi.md) before using package-native commands.

## Hardening recommendations for users

Shevanio AI pins versions and disables postinstall scripts on every npm install it generates. When you install the `permissions` component, a sensitive-paths deny list is applied to Claude Code and OpenCode blocking access to `~/.ssh/*`, `**/*.pem`, `**/*.key`, `**/.env*`, `~/.aws/credentials`, and other credential paths. See [Components](components.md) for the full list.

For broader protection across npm packages you install yourself, set these once on your machine:

- `npm config set ignore-scripts true` - blocks postinstall scripts globally; the primary supply-chain attack vector.
- `npm config set min-release-age 3` - skips packages published in the last 3 days; catches malicious typosquats before you install them.
- `npm config set allow-git none` - blocks git dependencies, which can be moving targets.

Optional wrapper tools for extra defense:

- [`npq`](https://github.com/lirantal/npq) - audits a package against several heuristics before it installs.
- [`sfw`](https://socket.dev/) (Socket Firewall) - runtime guard that intercepts suspicious behavior at install/run time.

## Unsupported platforms

If you run the installer on an unsupported OS or Linux distribution, it exits immediately with an error:

- `unsupported operating system: only macOS, Linux, and Windows are supported (detected <os>)`
- `unsupported linux distro: Linux support is limited to Ubuntu/Debian, Arch, and Fedora/RHEL family (detected <distro>)`
