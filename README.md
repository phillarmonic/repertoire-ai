<p align="center">
  <img src="docs/images/repertoire500.png" alt="Repertoire logo" width="250">
</p>

# Repertoire

**The `apt-get` for AI agent skills.**

A *skill* is a folder with a `SKILL.md` file that teaches an AI coding agent how
to do something: review code against your style guide, write docs for your
static site generator, use an internal tool. Codex, Claude Code, Cursor, Gemini
CLI, Copilot and the rest all read the same format, but each one looks for
skills in a different directory.

Repertoire installs a skill into every agent you use with one command, keeps
those copies up to date, and never overwrites edits you made by hand.

```shell
repertoire add zensical --target all
```

That one line:

- finds `zensical` in a skill **catalog** (a Git repository of skills),
- checks that the package is well formed,
- copies it into each agent's own skills folder,
- records what was installed so `repertoire update` can refresh it safely.

## Why Repertoire

- **One skill, every agent.** Install once; Repertoire fans the skill out to
  [30+ agents and harnesses](https://phillarmonic.github.io/repertoire-ai/concepts/targets-security/),
  each in its native layout.
- **Safe updates.** Every installed copy is tracked by content digest. If you
  changed a file locally, Repertoire refuses to replace or delete it unless you
  pass `--force`.
- **Your own catalogs.** Use the built-in `phillarmonic` catalog, a local folder,
  or a private company repository. Access uses your normal Git credentials;
  Repertoire never stores tokens.
- **Reproducible.** Commit a `repertoire.yaml` and `repertoire bootstrap` installs
  everything a project needs on a new laptop or in CI.
- **Agent-operable.** Repertoire ships with a skill about itself, so your coding
  agent can install skills, write manifests, and scaffold whole new skill
  catalogs for you.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | bash
```

Or with Go 1.27 or newer:

```shell
go install github.com/phillarmonic/repertoire-ai/cmd/repertoire@latest
```

Windows installers, pinned versions, and custom install directories are covered in
[Install](https://phillarmonic.github.io/repertoire-ai/#install). Check and update
the binary any time with `repertoire --version` and `repertoire --self-update`.

## Quick start

```shell
# See what the built-in catalog offers
repertoire list --available

# Install a skill into every agent on this machine
repertoire add zensical --target all

# Later: pull newer versions of everything you installed
repertoire update
```

Skills install under your home directory by default, so they are available in
every project. Add `--project` to install into the current Git repository instead.

## Set up a team or project

Commit a `repertoire.yaml` that lists the skills a project needs. Anyone who
clones the repository (or any CI job) then runs:

```shell
repertoire bootstrap
```

and gets the same skills in the same agents. See
[Set up a project or team](https://phillarmonic.github.io/repertoire-ai/automation/)
for the manifest format, private catalogs, and keeping installs current.

## Let your agent drive Repertoire

```shell
repertoire add repertoire --target all
```

With the `repertoire` skill installed, ask your agent in plain language:
"install the `zensical` skill for Codex and Claude", "create a private skill
catalog repository for our team", or "add a `repertoire.yaml` to this project".
The agent knows the commands, the manifest formats, and the safety rules, and
can scaffold a complete new catalog repository from scratch. See
[Let your agent drive Repertoire](https://phillarmonic.github.io/repertoire-ai/agent-skill/).

## Which command do I want?

- Install a skill and remember it: `repertoire add <skill>`
- Reinstall or repair skills already declared: `repertoire install`
- Pull newer versions: `repertoire update`
- Install everything a project declares (new machine, CI): `repertoire bootstrap`
- Same, but fetch catalog changes first: `repertoire sync`
- Something looks broken: `repertoire doctor`, then `repertoire doctor --fix`
- Use a private or local catalog: `repertoire catalog add <source>`
- Remove a skill: `repertoire remove <skill>`

## Supported agents

Agent Skills (`.agents`), Aider, Amp, Antigravity, Claude Code, Cline, CodeBuddy,
Codex, GitHub Copilot, Cursor, Devin, Factory Droid, DeepSeek Harness, Gemini CLI,
Hermes, Junie, Kilo Code, Kimi Code, Kiro, OpenClaw, OpenCode, Pi, Roo Code, Trae,
VS Code, Windsurf. The full list of target names and install paths is in
[Targets and security](https://phillarmonic.github.io/repertoire-ai/concepts/targets-security/).

## Documentation

Full documentation lives at
[phillarmonic.github.io/repertoire-ai](https://phillarmonic.github.io/repertoire-ai/):

- [Set up a project or team](https://phillarmonic.github.io/repertoire-ai/automation/)
- [Let your agent drive Repertoire](https://phillarmonic.github.io/repertoire-ai/agent-skill/)
- [Command reference](https://phillarmonic.github.io/repertoire-ai/commands/)
- [Private and company catalogs](https://phillarmonic.github.io/repertoire-ai/private-repositories/)
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
