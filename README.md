# Repertoire

Repertoire is a small package manager for portable AI agent skills. It finds skills in Git-backed catalogs and installs
them for Codex, Claude Code, Cursor, Gemini CLI, Windsurf, Cline, Roo Code, Kiro, Junie, Kimi Code, OpenCode, Copilot,
OpenClaw, or a shared `.agents` setup—without changing the open
`SKILL.md` format.

Use it to:

- discover reusable skills from public, private, or local catalogs;
- install the same `SKILL.md` package across multiple AI coding agents;
- keep managed skills updated without overwriting local changes;
- automate team onboarding and agent setup with a checked-in
  `.repertoire.yaml`.

The built-in `phillarmonic` catalog provides Phillarmonic's official vendored
skill set from [phillarmonic/ai-skills](https://github.com/phillarmonic/ai-skills).
Its skills can be referenced without declaring the repository or specifying a
catalog when their names are unique among the visible catalogs.

### Supported agents and harnesses

| Agent or harness               | Target     |
|--------------------------------|------------|
| Agent Skills shared convention | `agents`   |
| Claude Code                    | `claude`   |
| Cline                          | `cline`    |
| Codex                          | `codex`    |
| GitHub Copilot                 | `copilot`  |
| Cursor                         | `cursor`   |
| Gemini CLI                     | `gemini`   |
| Junie                          | `junie`    |
| Kimi Code                      | `kimi`     |
| Kiro                           | `kiro`     |
| OpenClaw                       | `openclaw` |
| OpenCode                       | `opencode` |
| Roo Code                       | `roo`      |
| Windsurf                       | `windsurf` |

## Automate AI agent skill installation

Repertoire automates the workflow of copying and maintaining Agent Skills for different coding harnesses. Instead of
manually cloning a skill and duplicating it into every client-specific directory, install it on every supported target
at once:

```shell
repertoire add zensical --target all
```

Repertoire validates the portable `SKILL.md` package, installs it into each agent's native home-directory skills
root by default, and records the exact catalog source and content digest. Unlike automatic target detection,
`--target all` includes every supported target even when its configuration directory does not exist yet. Repeat
`--target` with individual names when only a subset is needed. Use `--project` to install into a Git worktree
instead.

For repeatable developer onboarding and CI-managed environments, commit a
`.repertoire.yaml` manifest and run:

```shell
repertoire bootstrap
```

This gives teams one command to install or repair the required AI agent skills across supported tools without
maintaining separate setup scripts for every agent.

## Install

Install the latest prebuilt binary:

```bash
curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | bash
```

Set `INSTALL_DIR` to choose another destination, or pass a release version to install a specific build:

```bash
curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | INSTALL_DIR="$HOME/bin" bash
curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | bash -s -- v1.0.0
```

Alternatively, install the latest CLI with Go:

```shell
go install github.com/phillarmonic/repertoire-ai/cmd/repertoire@latest
```

This project currently requires Go 1.26.5 or newer. Make sure your Go bin directory is on `PATH`, then check the
installation:

```shell
repertoire --version
```

Update an installed release in place:

```shell
repertoire --self-update
```

The updater verifies the release checksum and downloaded binary before replacing
the current executable. It retains the five newest rollback copies under
`~/.repertoire/backups`.

To build from a local checkout instead:

```shell
git clone https://github.com/phillarmonic/repertoire-ai.git
cd repertoire-ai
go install ./cmd/repertoire
```

## Quick start

Repertoire includes the Phillarmonic skills catalog by default, so you can explore it immediately:

```shell
# See the available skills
repertoire list --available --catalog phillarmonic

# Install an official skill for Codex and record it as a requirement
repertoire add zensical --target codex

# Install it for every supported agent target instead
repertoire add zensical --target all

# See what Repertoire manages
repertoire list
```

By default, commands use user-global configuration and install into home-directory skill roots such as
`~/.codex/skills` or `~/.agents/skills`. Use `--project` when a skill should live in the Git worktree instead:

```shell
repertoire add zensical --target codex
repertoire add zensical --project --target agents
```

If no target is supplied, Repertoire detects existing Codex, Claude Code, Cursor, Gemini CLI, Windsurf, Cline, Roo Code,
Kiro, Junie, Kimi Code, OpenCode, Copilot, and OpenClaw configuration or skill directories. Run
`repertoire add <skill> --help` or use shell completion to see every target. Use `--target all` to skip detection and
install into every supported target.

The same target override works when repairing every declared skill or updating every managed skill:

```shell
repertoire install --target all
repertoire update --target all
```

## Set up a project in one command

Run `repertoire bootstrap` in a Git worktree. If `.repertoire.yaml` is missing, Repertoire creates one that lists
built-in catalog skills with namespaced IDs and `scope: global`—so the small manifest stays in the repo while skills
install under home-directory agent roots.

You can also commit a custom `.repertoire.yaml`:

```yaml
schema: 1

catalogs:
  company:
    source: git@github.com:example/company-skills.git
    ref: main

skills:
  github.com/phillarmonic/ai-skills/zensical:
    scope: global
    targets: [ codex ]

  shared-helpers:
    catalog: company
    scope: project
    targets: [ agents ]
```

Then install everything declared by the project:

```shell
repertoire bootstrap
```

`bootstrap` resolves from current catalog state, populates missing caches, and repairs missing managed copies without
refreshing an existing tracking cache. To fetch catalog updates before updating the declared skills, run:

```shell
repertoire sync
```

Project and global skills can be installed together. Removing an entry from the bootstrap manifest does not
automatically delete an installed skill.

## Everyday commands

| Command                           | Purpose                                                                       |
|-----------------------------------|-------------------------------------------------------------------------------|
| `repertoire list --available`     | Browse skills visible in configured catalogs                                  |
| `repertoire add <skill>`          | Install a skill and declare it as a requirement                               |
| `repertoire install [skill]`      | Install one skill or repair all requirements (`--target all` for every agent) |
| `repertoire update [skill]`       | Refresh and update one or all managed skills (`--target all` for every agent) |
| `repertoire remove <skill>`       | Safely remove a managed skill                                                 |
| `repertoire catalog add <source>` | Register a public, private, or local catalog                                  |
| `repertoire bootstrap`            | Install the skills in `.repertoire.yaml`                                      |
| `repertoire sync`                 | Refresh catalogs and synchronize `.repertoire.yaml`                           |

An unqualified skill name resolves automatically when exactly one visible
catalog defines it. If multiple catalogs define the same name, Repertoire lists
every matching catalog and source; repeat the command with `--catalog <name>`.

## Shell completion

Repertoire provides context-aware completion for commands, skill names, catalogs, installed skills, and targets. Enable
it for the current session:

```shell
# Bash
source <(repertoire completion bash)

# Zsh
source <(repertoire completion zsh)

# Fish
repertoire completion fish | source

# PowerShell
repertoire completion powershell | Out-String | Invoke-Expression
```

Completion reads only local state and existing catalog caches; pressing Tab never clones or refreshes a repository.

## Safe by default

Repertoire validates every catalog path and skill manifest before copying anything. Installations are staged and renamed
atomically, symlinks may not escape their skill directory, and skill scripts are copied as data rather than executed.

Managed content is tracked by digest. Repertoire refuses to replace or remove an unmanaged or locally modified target
unless you explicitly pass `--force`. Private catalog authentication is delegated to your normal Git credentials, so
tokens do not belong in Repertoire manifests.

## Learn more

- [Commands](docs/commands.md)
- [Manifests and project bootstrap](docs/concepts/manifests.md)
- [Catalogs](docs/concepts/catalogs.md)
- [Targets and security](docs/concepts/targets-security.md)
- [Private repositories](docs/private-repositories.md)
- [Troubleshooting](docs/troubleshooting.md)

To browse the complete documentation locally:

```shell
xdrun docs
```

## Development

Run the full local verification pipeline:

```shell
xdrun ci
```

Build the documentation with strict validation:

```shell
uv run zensical build --clean --strict
```
