# Repertoire

**The `apt-get` for AI agent skills.**

Repertoire is a small, fast package manager for portable AI agent skills. You run
one command, and the same skill is installed, verified, and kept up to date across
every AI coding agent you use. No more cloning a skill and hand-copying it into a
dozen client-specific directories.

```shell
repertoire add zensical --target all
```

That single line resolves the skill from a catalog, validates the package,
installs a managed copy into each agent's native skills root, and records the
exact source and content digest so future updates never clobber your local edits.

## Why Repertoire

If you have used a Linux package manager, you already know how this works. Skills
live in **catalogs** (Git repositories), you **add** the ones you want, and you
**update** them when upstream changes. Repertoire brings that same discipline to
the messy reality of AI agent skills:

- **One skill, every agent.** Skills use the open `SKILL.md` format. Repertoire
  fans a single package out to Codex, Claude Code, Cursor, Copilot, Gemini CLI,
  and [many more](#supported-agents-and-harnesses), each in its native layout.
- **Managed, not copied.** Every install is tracked by content digest. Repertoire
  refuses to overwrite or remove a locally modified skill unless you pass
  `--force`, so updates stay safe.
- **Catalogs you control.** Use the built-in `phillarmonic` catalog, a local
  checkout, or a private company catalog on your own Git remote. Access is gated
  by normal Git auth; Repertoire never stores tokens or passwords.
- **Reproducible by design.** Commit a `repertoire.yaml` and a single
  `repertoire bootstrap` sets up every required skill for the whole team, in CI or
  on a new laptop.
- **Safe by default.** Packages are validated before copying, installs are staged
  and renamed atomically, symlinks cannot escape their skill directory, and skill
  scripts are copied as data rather than executed.

The built-in `phillarmonic` catalog ships
[Phillarmonic's official skill set](https://github.com/phillarmonic/ai-skills) and
works with no extra configuration. An unqualified skill name prefers this official
catalog; use `--catalog` or a source-qualified ID to pick another.

## Install

Install the latest prebuilt binary:

```bash
curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | bash
```

Or install with Go (requires Go 1.27 or newer):

```shell
go install github.com/phillarmonic/repertoire-ai/cmd/repertoire@latest
```

Verify the install and update in place when needed:

```shell
repertoire --version
repertoire --self-update
```

See [Install](https://phillarmonic.github.io/repertoire-ai/#install) for custom
install directories, pinned versions, and building from source.

## Quick start

```shell
# Browse the built-in Phillarmonic catalog
repertoire list --available --catalog phillarmonic

# Install a skill for every supported agent and record it as a requirement
repertoire add zensical --target all

# See what Repertoire manages, then keep it current
repertoire list
repertoire update --target all
```

Commands default to user-global configuration and install into home-directory
skill roots. Pass `--project` to install into the current Git worktree instead.

## Set up a whole project in one command

Commit a `repertoire.yaml` that declares the skills and targets your project
needs, including private company catalogs:

```yaml
schema: 1
tool: https://github.com/phillarmonic/repertoire-ai

catalogs:
  company:
    source: git@github.com:example/company-skills.git
    ref: main

skills:
  github.com/phillarmonic/ai-skills/zensical:
    scope: global
    targets: [ codex ]

  github.com/example/company-skills/phillarmonkey-code:
    scope: project
    targets: [ agents ]
```

Then install everything the project declares:

```shell
repertoire bootstrap   # install and repair declared skills
repertoire sync        # fetch catalog updates, then update declared skills
```

See [Automate agent skills](https://phillarmonic.github.io/repertoire-ai/automation/)
for the full manifest, private catalogs, project versus global scope, and removal.

## Supported agents and harnesses

| Agent or harness               | Target                |
| ------------------------------ | --------------------- |
| Agent Skills shared convention | `agents`              |
| Aider                          | `aider`               |
| Amp                            | `amp`                 |
| Antigravity                    | `antigravity`         |
| Antigravity on Windows         | `antigravity-windows` |
| Claude Code                    | `claude`              |
| OpenClaw native layout         | `claw`                |
| Cline                          | `cline`               |
| CodeBuddy                      | `codebuddy`           |
| Codex                          | `codex`               |
| GitHub Copilot                 | `copilot`             |
| Cursor                         | `cursor`              |
| Devin                          | `devin`               |
| Factory Droid                  | `droid`               |
| DeepSeek Harness               | `dsh`                 |
| Gemini CLI                     | `gemini`              |
| Hermes                         | `hermes`              |
| Junie                          | `junie`               |
| Kilo Code                      | `kilo`                |
| Kimi Code                      | `kimi`                |
| Kiro                           | `kiro`                |
| OpenClaw                       | `openclaw`            |
| OpenCode                       | `opencode`            |
| Pi                             | `pi`                  |
| Roo Code                       | `roo`                 |
| Trae                           | `trae`                |
| Trae China                     | `trae-cn`             |
| VS Code Copilot instructions   | `vscode`              |
| Claude Code on Windows         | `windows`             |
| Windsurf                       | `windsurf`            |

Pass `--target all` to install into every supported target, or repeat `--target`
with individual names for a subset. Without a target, Repertoire auto-detects the
agents already configured on your machine.

## Everyday commands

| Command                              | Purpose                                                                       |
| ------------------------------------ | ----------------------------------------------------------------------------- |
| `repertoire list --available`        | Refresh and browse skills visible in configured catalogs                      |
| `repertoire add <skill>`             | Install a skill and declare it as a requirement                               |
| `repertoire install [skill]`         | Install one skill or repair all requirements (`--target all` for every agent) |
| `repertoire update [skill\|catalog]` | Refresh catalogs and update managed skills (`--target all` for every agent)   |
| `repertoire remove <skill>`          | Safely remove a managed skill                                                 |
| `repertoire catalog add <source>`    | Register a public, private, or local catalog                                  |
| `repertoire bootstrap`               | Install the skills declared in `repertoire.yaml`                              |
| `repertoire sync`                    | Refresh catalogs and synchronize the declared skills                          |
| `repertoire doctor`                  | Diagnose broken or stale installs; `--fix` repairs, `--reset` reinstalls      |

## Documentation

Full documentation lives at
**[phillarmonic.github.io/repertoire-ai](https://phillarmonic.github.io/repertoire-ai/)**:

- [Automate agent skills](https://phillarmonic.github.io/repertoire-ai/automation/)
- [Commands](https://phillarmonic.github.io/repertoire-ai/commands/)
- [Private repositories](https://phillarmonic.github.io/repertoire-ai/private-repositories/)
- [Troubleshooting](https://phillarmonic.github.io/repertoire-ai/troubleshooting/)
- Concepts:
  [Manifests and state](https://phillarmonic.github.io/repertoire-ai/concepts/manifests/),
  [Catalogs](https://phillarmonic.github.io/repertoire-ai/concepts/catalogs/),
  [Targets and security](https://phillarmonic.github.io/repertoire-ai/concepts/targets-security/)

Browse the docs locally with `xdrun docs`.

## Development

```shell
# Run the full local verification pipeline
xdrun ci

# Build the documentation with strict validation
uv run zensical build --clean --strict
```

See [Contributing](https://phillarmonic.github.io/repertoire-ai/contributing/) to get started.
