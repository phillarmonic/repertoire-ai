---
icon: lucide/library
---

# Repertoire

Repertoire automates installing, syncing, and managing portable AI agent skills.
It discovers `SKILL.md` packages in Git-backed catalogs and installs them into
the native home-directory skill roots used by Codex, Claude Code, Cursor,
Gemini CLI, Windsurf, Cline, Roo Code, Kiro, Junie, Kimi Code, OpenCode,
GitHub Copilot, OpenClaw, DeepSeek Harness, and shared `.agents` setups. Use `--project` only
when a skill should live inside a Git worktree.

The built-in `phillarmonic` catalog provides Phillarmonic's official vendored
skill set from [phillarmonic/ai-skills](https://github.com/phillarmonic/ai-skills).
Its skills can be referenced without declaring the repository or specifying a
catalog. Unqualified names prefer this official mainline catalog. Use
`--catalog <name>` or a source-qualified ID to choose a different definition;
ambiguity remains an error when only non-mainline catalogs match.

## Install

Repertoire ships as a single self-contained binary. Choose the method that fits
your platform.

=== "Windows"

    Install for the current user — no administrator rights required.

    **Installer (recommended)**

    Download `repertoire-setup-<version>.exe` from the
    [latest release](https://github.com/phillarmonic/repertoire-ai/releases/latest)
    and run it. The setup installs `repertoire.exe` under
    `%LOCALAPPDATA%\Programs\Repertoire`, automatically selects the build for your
    architecture (x64 or ARM64), and adds it to your user `PATH`. Open a new
    terminal afterwards so the `repertoire` command resolves.

    **PowerShell script**

    Prefer the command line? Run the installer script in PowerShell:

    ```powershell
    irm https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.ps1 | iex
    ```

    It downloads the binary for your architecture, verifies its SHA-256
    checksum, installs it to the same per-user location, and updates your `PATH`.
    Pin a specific version by setting an environment variable first:

    ```powershell
    $env:REPERTOIRE_VERSION = "v1.2.3"; irm https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.ps1 | iex
    ```

=== "Linux and macOS"

    Install the latest prebuilt binary:

    ```bash
    curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | bash
    ```

    Set `INSTALL_DIR` to change the target directory (default `~/.local/bin`), or
    pass a tag to pin the version:

    ```bash
    curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | bash -s -- v1.2.3
    ```

=== "Go"

    Build and install from source with Go 1.27 or newer:

    ```bash
    go install github.com/phillarmonic/repertoire-ai/cmd/repertoire@latest
    ```

Verify the installation, and update in place when needed:

```bash
repertoire --version
repertoire --self-update
```

## Install one skill across multiple AI coding agents

Use one command instead of manually copying the same skill into every
client-specific directory:

```bash
repertoire add code-reviewer --target all
```

Repertoire validates the package, installs each managed copy safely, and tracks
its catalog source and digest so later updates cannot silently overwrite local
changes. `--target all` includes every supported agent target, whether or not
that agent's configuration directory already exists.

Repair all declared skills or update all managed skills across the same target
set with:

```bash
repertoire install --target all
repertoire update --target all
```

For automated team onboarding, declare the required skills and targets in the
`skills` section of `repertoire.yaml`, commit it with the project, and run:

```bash
repertoire bootstrap
```

[See the complete automation workflow](automation.md), including private
catalogs, project versus global installation, updates, and removal.

## Project status

Repertoire is built in four layers:

1. a stable Go command-line interface and repeatable local CI;
2. versioned catalog and installation state;
3. safe catalog resolution and client installation;
4. complete add, install, update, remove, and list workflows.

## Development checks

The project uses [Drun](https://github.com/phillarmonic/drun) for repeatable
execution:

```bash
xdrun ci
```

This runs Go vet, formatting and lint checks, unit tests, package builds, and
high-confidence security checks.
