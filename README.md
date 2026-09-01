# Repertoire

Repertoire is a small package manager for portable AI agent skills. It finds skills in Git-backed catalogs and installs
them for Codex, Claude Code, Cursor, Gemini CLI, Windsurf, Cline, Roo Code, Kiro, Junie, Kimi Code, OpenCode, Copilot,
OpenClaw, or a shared `.agents` setup—without changing the open
`SKILL.md` format.

Use it to:

- discover reusable skills from public catalogs, local paths, or private company catalogs you host yourself;
- install the same `SKILL.md` package across multiple AI coding agents;
- keep managed skills updated without overwriting local changes;
- automate team onboarding and agent setup with a checked-in
  `repertoire.yaml`.

The built-in `phillarmonic` catalog provides Phillarmonic's official vendored skill set
from [phillarmonic/ai-skills](https://github.com/phillarmonic/ai-skills). Its skills can be referenced without declaring
the repository or specifying a catalog. An unqualified name prefers this official mainline catalog; use `--catalog` or a
source-qualified ID to select another catalog explicitly.

### Testing skill repositories locally

While developing a catalog, you can redirect any catalog to a local checkout so you do not have to push before testing.
Set the `REPERTOIRE_OVERRIDES` environment variable to a comma-separated list of `name=path` or `source=path` pairs,
or pass a repeatable `--override name=path` flag. Flags win over environment values.

```shell
REPERTOIRE_OVERRIDES="phillarmonic=/path/to/ai-skills" repertoire add zensical --target codex
repertoire --override company=/path/to/company-skills add code-reviewer --catalog company
repertoire catalog list   # marks overridden sources
```

The override path is read directly instead of the remote: `add`, `install`, `update`, `bootstrap`, `sync`, `list
--available`, and completion all resolve from the local checkout. Remove the override to return to the registered remote
source.

### Private Company Catalogs

You can point Repertoire at your own Git-backed catalog: a private catalog of company-owned AI skills—just as you would
a public one. Host the repository on GitHub, GitLab, Bitbucket, or any other Git remote your team already uses, then
register it:

```shell
repertoire catalog add git@github.com:your-org/company-skills.git --name company
```

Access is gated by normal Git authentication only. Repertoire never stores tokens or passwords; it uses the same SSH
agent, credential helper, or provider CLI (`gh auth setup-git`, and so on) that already lets `git clone` and
`git ls-remote` reach that repository. Without credentials that can read the remote, the private catalog is unavailable.
See
[Private repositories](https://phillarmonic.github.io/repertoire-ai/private-repositories/) for details.

### Supported agents and harnesses

| Agent or harness               | Target                |
|--------------------------------|-----------------------|
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

## Automate AI agent skill installation

Repertoire automates the workflow of copying and maintaining Agent Skills for different coding harnesses. Instead of
manually cloning a skill and duplicating it into every client-specific directory, install it on every supported target
at once:

```shell
repertoire add zensical --target all
```

Repertoire validates the portable `SKILL.md` package, installs it into each agent's native home-directory skills root by
default, and records the exact catalog source and content digest. Unlike automatic target detection,
`--target all` includes every supported target even when its configuration directory does not exist yet. Repeat
`--target` with individual names when only a subset is needed. Use `--project` to install into a Git worktree instead.

Installed skills can also expose small file-backed stubs. The
`repertoire stub get <skill>/<stub>` command gives an agent a verified asset path and authored instructions without
changing the project or printing the asset contents.

For repeatable developer onboarding and CI-managed environments, commit a
`repertoire.yaml` manifest with a `skills` section and run:

```shell
repertoire bootstrap
```

This gives teams one command to install or repair the required AI agent skills across supported tools without
maintaining separate setup scripts for every agent. Catalog-provided project instructions remain small and project-local
even when the full skill is installed globally.

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

The updater verifies the release checksum and downloaded binary before replacing the current executable. It retains the
five newest rollback copies under
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

Run `repertoire bootstrap` in a Git worktree. If `repertoire.yaml` declares no bootstrap skills, Repertoire adds a
starter `skills` section that lists built-in catalog skills with source-qualified IDs and `scope: global`—so the small
manifest stays in the repo while skills install under home-directory agent roots.

You can also commit a custom `repertoire.yaml`, including a private company catalog (reachable only with Git credentials
that can read that remote):

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

Then install everything declared by the project:

```shell
repertoire bootstrap
```

`bootstrap` resolves from current catalog state, populates missing caches, and repairs missing managed copies without
refreshing an existing tracking cache. To fetch catalog updates before updating the declared skills, run:

```shell
repertoire sync
```

Project and global skills can be installed together. Removing an entry from the `skills` section does not automatically
delete an installed skill.

Repertoire previously read project bootstrap declarations from a separate `.repertoire.yaml` file. When `bootstrap`
or `sync` finds that legacy file and `repertoire.yaml` has no `skills` section, it merges the declarations into
`repertoire.yaml` and removes `.repertoire.yaml` automatically.

Catalogs may distinguish always-on project instructions from optional hooks. Bootstrap installs instruction pointers
into the worktree even for
`scope: global` skills, while keeping their management state in Repertoire's global lock. Set `hooks: true` only when
that repository should also receive the catalog's hook and integration artifacts. Removing the global skill also removes
its recorded project artifacts without touching unrelated content.

For broad skills, prefer owner-prefixed kebab-case identifiers instead of generic names. For example, use a
source-qualified manifest key such as
`github.com/example/company-skills/phillarmonkey-code` rather than a bare
`code`, so agents and UIs can tell which vendor or personal catalog owns the behavior.

## Everyday commands

| Command                              | Purpose                                                                       |
|--------------------------------------|-------------------------------------------------------------------------------|
| `repertoire list --available`        | Refresh and browse skills visible in configured catalogs                      |
| `repertoire add <skill>`             | Install a skill and declare it as a requirement                               |
| `repertoire install [skill]`         | Install one skill or repair all requirements (`--target all` for every agent) |
| `repertoire update [skill\|catalog]` | Refresh catalogs and update managed skills (`--target all` for every agent)   |
| `repertoire remove <skill>`          | Safely remove a managed skill                                                 |
| `repertoire catalog add <source>`    | Register a public, private, or local catalog                                  |
| `repertoire bootstrap`               | Install the skills declared in `repertoire.yaml`                              |
| `repertoire sync`                    | Refresh catalogs and synchronize the declared skills                          |
| `repertoire doctor`                  | Diagnose broken or stale installs; `--fix` repairs, `--reset` reinstalls      |

An unqualified skill name prefers the official `phillarmonic` catalog when it defines that skill. Otherwise, if multiple
catalogs define the same name, Repertoire lists every match; repeat the command with `--catalog <name>` or use a
source-qualified ID.

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
unless you explicitly pass `--force`. Private company catalogs rely on your existing Git credentials—SSH keys,
credential helpers, or provider CLIs—so tokens never belong in Repertoire manifests.

## Learn more

- [Commands](https://phillarmonic.github.io/repertoire-ai/commands/)
- [Manifests and project bootstrap](https://phillarmonic.github.io/repertoire-ai/concepts/manifests/)
- [Catalogs](https://phillarmonic.github.io/repertoire-ai/concepts/catalogs/)
- [Targets and security](https://phillarmonic.github.io/repertoire-ai/concepts/targets-security/)
- [Private repositories](https://phillarmonic.github.io/repertoire-ai/private-repositories/)
- [Troubleshooting](https://phillarmonic.github.io/repertoire-ai/troubleshooting/)

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
