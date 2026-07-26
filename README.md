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
repertoire add code-reviewer --target all
```

Repertoire validates the portable `SKILL.md` package, installs it into each agent's native project or global skills
directory, and records the exact catalog source and content digest. Unlike automatic target detection,
`--target all` includes every supported target even when its configuration directory does not exist yet. Repeat
`--target` with individual names when only a subset is needed.

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

# Install a skill for Codex and record it as a requirement
repertoire add zensical --catalog phillarmonic --target codex

# Install it for every supported agent target instead
repertoire add zensical --catalog phillarmonic --target all

# See what Repertoire manages
repertoire list
```

Inside a Git worktree, commands use project scope and install below directories such as `.codex/skills` or
`.agents/skills`. Outside a worktree, they use your user-global configuration. Choose explicitly when needed:

```shell
repertoire add zensical --global --target codex
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

Check a `.repertoire.yaml` file into the project root when contributors need a known set of skills:

```yaml
schema: 1

catalogs:
  company:
    source: git@github.com:example/company-skills.git
    ref: main

skills:
  zensical:
    catalog: phillarmonic
    scope: project
    targets: [ codex ]

  code-reviewer:
    catalog: company
    scope: global
    targets: [ codex, claude ]
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

Unqualified skill names must resolve to exactly one visible catalog. Use
`--catalog <name>` when the same name exists in more than one catalog.

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
