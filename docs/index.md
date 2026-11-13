---
description: "Repertoire installs an agent skill into every coding agent you use and pins it with a lockfile you commit: a digest per target, a manifest in the repository, no telemetry."
---

<p align="center">
  <img src="images/repertoire500.png" alt="Repertoire logo" width="250">
</p>

# The lockfile for agent skills

Repertoire installs a skill into every coding agent you use, then pins it with a
lockfile you commit. Every laptop and CI job gets the same skills, and an edit
you made by hand is never overwritten.

```text
$ repertoire add zensical --target all
added zensical from catalog phillarmonic (claude,codex)

$ echo "my local note" >> ~/.claude/skills/zensical/SKILL.md
$ repertoire update
error: install "zensical" to claude: target is unmanaged or locally modified; use --force to replace it
$ echo $?
1
```

| Targets | Recorded | Telemetry |
| --- | --- | --- |
| 30 agent directories | catalog, commit, and a digest per target | none |

`update` compares the bytes it installed against the bytes on disk. When they
differ it stops, names the target and exits non-zero, so a CI job fails instead
of quietly shipping somebody's local edit. [The skill lockfile](concepts/lockfile.md)
is the page that spells out what is recorded and what the digest covers.

A *skill* is a folder with a `SKILL.md` file that teaches an AI coding agent how
to do something: review code against your style guide, write docs for your
static site generator, use an internal tool. Codex, Claude Code, Cursor, Gemini
CLI, Copilot and many others all read the same format, but each one looks for
skills in a different directory.

## Why a package manager and not a copier

If you have used `apt-get`, the model is familiar. Copying a `SKILL.md` into an
agent folder is enough for a one-off try. Repertoire treats skills as packages
instead:

- A **catalog** is a named Git source. Skills resolve by catalog and by
  [source-qualified ID](glossary.md), so two catalogs can both offer `review`
  without a silent pick.
- A **lock file** records the catalog commit and a content digest per target.
  `update` and `remove` refuse to touch a copy you edited by hand unless you
  pass `--force`. See [The skill lockfile](concepts/lockfile.md).
- A committed **manifest** (`repertoire.yaml`) plus `bootstrap` or `sync`
  reproduces the same set on a new laptop and in CI.

## What it replaces

[Repertoire vs copy-paste](vs-copy-paste.md) has the full comparison. The short
version:

| | Copying folders | Repertoire |
| --- | --- | --- |
| Installs into 30 targets | one at a time | one command |
| Records what was installed | no | manifest and lock |
| Notices a copy that changed | no | digest per target |
| Reproduces on a new machine | no | `repertoire bootstrap` |
| Needs no tooling at all | **yes** | no |

The last row is the one to read before adopting anything.

## What it does not do

- It does not host skills. A catalog is a Git repository you control.
- It does not run or sandbox a skill. A skill is instructions, and the agent
  acts on them.
- It does not manage `CLAUDE.md`, `AGENTS.md` or settings files, except files a
  skill itself ships.
- Signatures are opt-in per catalog. Without a [trust block](concepts/targets-security.md),
  a digest still detects change; it does not establish who published it.

## Coming from a one-off skill installer

You can keep using another installer to try a skill. Repertoire ignores copies
it does not manage, so the two can sit in the same agent folders. When a skill
is worth keeping, register its repository as a catalog (`repertoire catalog
add` or `repertoire add <git-url>`), then `repertoire add` so the lock tracks
it. From then on `update`, `bootstrap`, and `doctor` own that copy.

## Install

Repertoire ships as a single self-contained binary. Choose the method that fits
your platform.

=== "Windows"

    Install for the current user, no administrator rights required.

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

## Quick start

If you have used a Linux package manager, the model is familiar: a catalog is a
repository, `add` installs a package, and `repertoire.yaml` is the manifest you
commit so others get the same set. Underlined terms across these docs link to
the [glossary](glossary.md); hover one for a short definition.

```bash
repertoire list --available
```

Shows every skill offered by the catalogs you have configured. With a fresh
install that is the built-in `phillarmonic` catalog.

```bash
repertoire add zensical --target all
```

Installs `zensical` into every supported agent and records it as something you
want kept installed. Drop `--target all` to install only into agents Repertoire
detects on your machine, or name agents explicitly with
`--target codex --target claude`. On a machine with no agent installed nothing
is detected, so the first install names `--target all`.

That first command writes a lock under your home directory, which is right for
one machine and wrong for a team. To put the manifest and the lock in the
repository, run it from the Git root with `--project`:

```bash
repertoire --project add zensical --target all
```

Commit `repertoire.yaml` and `repertoire.lock.json`. A teammate clones the
repository and runs `repertoire bootstrap` to get the same skills into their
own agent directories.

To install from a repository you have not registered yet:

```bash
repertoire add /path/to/agent-skills --target all
```

That registers a catalog named after the directory and installs every skill it
offers. Pass `--name` and `--skill` (or a `/<skill>` tail) to choose the
catalog name and a subset.

```bash
repertoire list
```

Shows what Repertoire manages on this machine, where each skill came from, and
which agents have it.

```bash
repertoire update
```

Fetches catalog changes and refreshes every installed skill. If you edited an
installed copy by hand, `update` stops with an error instead of overwriting
it.

## Which command do I want?

- Install a skill and remember it: `repertoire add <skill>`
- Install from a Git URL or local repo: `repertoire add <git-url>`
- Start a project manifest without installing: `repertoire init`
- Inspect where an installed skill came from: `repertoire show <skill>`
- Scaffold a new catalog repository: `repertoire catalog init`
- Preview a mutating command without writing: add `--dry-run`
- Reinstall or repair skills already declared: `repertoire install`
- Pull newer versions: `repertoire update`
- Install everything a project declares (new machine, CI): `repertoire bootstrap`
- Same, but fetch catalog changes first: `repertoire sync`
- Something looks broken: `repertoire doctor`, then `repertoire doctor --fix`
- Use a private or local catalog: `repertoire catalog add <source>`
- Remove a skill: `repertoire remove <skill>`

Every command is described in the [command reference](commands.md).

## Let your agent drive Repertoire

Repertoire ships with a skill about itself. Install it once and your coding
agent knows how to run Repertoire for you:

```bash
repertoire add repertoire --target all
```

Then ask in plain language: "install the `zensical` skill for Codex and
Claude", "create a private skill catalog repository for our team", or "add a
`repertoire.yaml` to this project". The agent knows the commands, the manifest
formats, and the safety rules, and can scaffold a whole new catalog repository
from scratch. See [Let your agent drive Repertoire](agent-skill.md) for what it
covers and what it will refuse to do.

## Next steps

- [The skill lockfile](concepts/lockfile.md): what is recorded, what the digest
  covers, and what `update` refuses.
- [Repertoire vs copy-paste](vs-copy-paste.md): the comparison, and when copying
  is the right answer.
- [Set up a project or team](automation.md): commit a `repertoire.yaml` so
  every contributor and CI job gets the same skills.
- [Private and company catalogs](private-repositories.md): publish your own
  skills from a private Git repository.
- [Let your agent drive Repertoire](agent-skill.md): install the built-in
  `repertoire` skill and delegate all of the above to your agent.
- [Command reference](commands.md): every command, flag, and edge case.
- [Troubleshooting](troubleshooting.md): what the common error messages mean
  and how to fix them.
- [Glossary](glossary.md): the handful of terms Repertoire uses, in one place.
- Concepts: [Manifests and state](concepts/manifests.md),
  [Catalogs](concepts/catalogs.md),
  [Targets and security](concepts/targets-security.md).
- [Contributing](contributing.md): build and test Repertoire itself.
