# Quickstart

## Prerequisites

### macOS

- Homebrew installed and available in PATH.
- `git` available.
- The future `shevanio/tap/shevanio-ai` formula is not published yet; use the local source build below.

### Ubuntu/Debian (and derivatives like Linux Mint, Pop!\_OS)

- `apt-get` available (standard on these distros).
- `sudo` access for package installs.
- `git` available.
- If Node.js is missing, `shevanio-ai install` prints this install hint: NodeSource LTS setup + `apt-get install -y nodejs` (npm comes bundled).
- If using Homebrew on Linux, Bubblewrap may require unprivileged user namespaces; see `docs/usage.md#homebrew-upgrade-troubleshooting`.

### Arch Linux (and derivatives like Manjaro, EndeavourOS)

- `pacman` available (standard on these distros).
- `sudo` access for package installs.
- `git` available.
- If Node.js is missing, `shevanio-ai install` prints this install hint: `pacman -S --noconfirm nodejs npm`.

### Fedora / RHEL family (Fedora, CentOS Stream, Rocky Linux, AlmaLinux)

- `dnf` available (standard on these distros).
- `sudo` access for package installs.
- `git` available.
- If Node.js is missing, `shevanio-ai install` prints this install hint: NodeSource LTS setup + `dnf install -y nodejs` (npm comes bundled).

### All platforms

- Git 2.38+.
- Go 1.25.10+ (for building from source).
- Node.js 18+ and npm: `shevanio-ai install` checks these as required prerequisites on every platform and prints a warning with a distro-specific install hint (see above) if either is missing — regardless of which agents/components you select. It does not install them for you, and it does not install agent runtimes either: if a selected agent isn't detected, `shevanio-ai install` refuses and prints the exact `npm install -g` (or equivalent) command for you to run yourself. Node.js/npm are strictly required if you select the CodeGraph community tool, which shevanio-ai does install via `npm install -g`.
- Pi installed and available as `pi` on `PATH` if you select the Pi agent.

### Windows

- Go 1.25.10+ is required to build `./cmd/shevanio-ai` from source.
- No official Windows archive or Scoop package is published.

## Version Policy

Shevanio AI currently has no published release. Its runtime baseline is derived from [Gentle AI v2.4.0](https://github.com/Gentleman-Programming/gentle-ai/tree/v2.4.0); upstream version milestones remain provenance, not Shevanio AI release numbers.

### Build and install locally

```bash
go build -trimpath -o ./bin/shevanio-ai ./cmd/shevanio-ai
mkdir -p "$HOME/.local/bin"
install -m 0755 ./bin/shevanio-ai "$HOME/.local/bin/shevanio-ai"
"$HOME/.local/bin/shevanio-ai" version
```

The future module command will be `go install github.com/shevanio/shevanio-ai/v2/cmd/shevanio-ai@<tag>` after a Shevanio AI tag is published.

## Run

```bash
go run ./cmd/shevanio-ai install --dry-run
```

Use `--dry-run` first to validate selections and execution plan without applying changes. The dry-run output includes a `Platform decision` line showing the detected OS, distro, package manager, and support status.

## First real install

```bash
go run ./cmd/shevanio-ai install
```

The installer detects your platform automatically — no flags needed to select macOS vs Linux. Install commands are resolved through the appropriate package manager (brew, apt, pacman, or dnf) based on detection.

After completion, verify that agent configs and selected components were installed to their expected paths.

The agents you select during install become the default scope for future `shevanio-ai sync` runs. Shevanio AI records that selection in `~/.shevanio-ai/state.json` and does not automatically sync every agent config directory that exists on your machine. To check what will be updated after an upgrade, run:

```bash
shevanio-ai sync --dry-run
```

To update a different set explicitly, pass every target agent:

```bash
shevanio-ai sync --agent claude-code --agent opencode
```

## Verification outcome

When checks pass, installer reports:

`You're ready. Run 'claude' or 'opencode' and start building.`

If something looks wrong after install, run `shevanio-ai doctor` for a read-only health check. It verifies tool binaries, `state.json` validity, Engram MCP reachability, and disk space — each check reports pass/warn/fail with a remedy hint.

For a Pi-only install, the plan shows the Pi package stack instead of Shevanio AI components. It installs `gentle-pi`, `gentle-engram`, and `pi-mcp-adapter`, runs `pi-engram init` through the pinned `gentle-engram` package, then installs `pi-subagents-j0k3r`, `@juicesharp/rpiv-ask-user-question`, `pi-web-access`, `@juicesharp/rpiv-todo`, and `pi-btw`.

## Hardening recommendations for users

Shevanio AI pins versions and disables postinstall scripts on every npm install it generates. When you install the `permissions` component, a sensitive-paths deny list is applied to Claude Code and OpenCode blocking access to `~/.ssh/*`, `**/*.pem`, `**/*.key`, `**/.env*`, `~/.aws/credentials`, and other credential paths. See [Components](../docs/components.md) for the full list.

For broader protection across npm packages you install yourself, set these once on your machine:

- `npm config set ignore-scripts true` — blocks postinstall scripts globally; the primary supply-chain attack vector.
- `npm config set min-release-age 3` — skip packages published in the last 3 days; catches malicious typosquats before you install them.
- `npm config set allow-git none` — block git: dependencies, which can be moving targets.

Optional wrapper tools for extra defense:

- [`npq`](https://github.com/lirantal/npq) — audits a package against several heuristics before it installs.
- [`sfw`](https://socket.dev/) (Socket Firewall) — runtime guard that intercepts suspicious behavior at install/run time.

## Unsupported platforms

If you run the installer on an unsupported OS or Linux distro, it exits immediately with an error:

- `unsupported operating system: only macOS, Linux, and Windows are supported (detected <os>)`
- `unsupported linux distro: Linux support is limited to Ubuntu/Debian, Arch, and Fedora/RHEL family (detected <distro>)`
