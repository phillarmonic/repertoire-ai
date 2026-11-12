<p align="center">
  <img src="docs/images/repertoire500.png" alt="Repertoire logo" width="250">
</p>

# The lockfile for agent skills

Repertoire installs a skill into every coding agent you use, then pins it with a
lockfile you commit. The same skills arrive on every laptop and CI job, and an
edit you made by hand is never overwritten.

```bash
repertoire add zensical --target all
```

That line finds `zensical` in a skill catalog, checks the package, copies it into
each agent's own skills directory, and records a content digest per target.
`repertoire update` refreshes the copies it owns and refuses the ones you
changed.

[Install](#install) · [Documentation](https://phillarmonic.github.io/repertoire-ai/) · [Why a lockfile](#why-a-lockfile)

## Why a lockfile

Copying a `SKILL.md` folder works once. It does not survive a second agent, a
teammate, or a CI job.

| | Copy-paste | A skills repo | Repertoire |
| --- | --- | --- | --- |
| Installs into 30+ agents | one at a time | one at a time | one command |
| Records what was installed | no | no | manifest and lock |
| Notices a changed skill | no | only if committed | digest per target |
| Refuses to overwrite your edit | n/a | n/a | yes, without `--force` |
| Reproduces on a new machine | no | partly | `repertoire bootstrap` |
| Works with a private catalog | n/a | yes | yes, over your Git credentials |
| Verifies a signature over each skill | no | no | optional, per catalog |

A *skill* is a folder with a `SKILL.md` that teaches an agent how to do
something. Codex, Claude Code, Cursor, Gemini CLI, Copilot and the rest read the
same format and look in different directories. That is the whole problem, and it
is a package-management problem.

## Install

Linux and macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.sh | bash
```

Windows: download `repertoire-setup-<version>.exe` from the
[latest release](https://github.com/phillarmonic/repertoire-ai/releases/latest),
or run the installer script in PowerShell:

```powershell
irm https://raw.githubusercontent.com/phillarmonic/repertoire-ai/master/install.ps1 | iex
```

Go 1.27 or newer:

```bash
go install github.com/phillarmonic/repertoire-ai/cmd/repertoire@latest
```

## Five minutes to a lockfile

```bash
# What the built-in catalog offers
repertoire list --available

# Install into every supported agent
repertoire add zensical --target all

# What Repertoire now manages, and where each copy came from
repertoire show zensical
```

`show` prints the catalog, the commit, the digest, and the integrity of every
copy. That output is the point of the tool.

## Prove it does not overwrite you

Edit an installed skill, then update:

```bash
echo "my local note" >> ~/.claude/skills/zensical/SKILL.md
repertoire update
```

`update` stops and exits 1 when a copy was edited locally, naming the skill and
the target. Pass `--force` when you mean it. This is the behaviour that separates
a package manager from a copier, and it takes about twenty seconds to check.

## A team, in one committed file

```yaml
# repertoire.yaml
schema: 1
skills:
  zensical:
    catalog: phillarmonic
  repertoire:
    catalog: phillarmonic
```

```bash
repertoire bootstrap   # on a new machine or in CI
repertoire sync        # same, after fetching catalog changes
```

## Catalogs

A catalog is a Git repository with a `repertoire.yaml`. Point at your existing
repository and it works even without one: a source with no manifest is read as a
[loose catalog](https://phillarmonic.github.io/repertoire-ai/concepts/catalogs/),
and any directory holding a valid `SKILL.md` becomes a skill.

```bash
repertoire catalog add git@github.com:example/private-skills.git --name company
repertoire catalog init            # scaffold a new catalog repository
```

Catalogs use your normal Git credentials. Repertoire stores no tokens and sends
no telemetry.

## Signed skills

A catalog can require signed commits, and a trusted skill ships
`REPERTOIRE.digest.asc`, a detached signature over its content digest.
Verification runs locally in a temporary keyring, imports only the keys named in
the trust block, and never contacts a keyserver. A failed check stops `add`,
`install`, `update` and `bootstrap` before the lock is written.

## Let your agent drive it

```bash
repertoire add repertoire --target all
```

Then ask the agent to install a skill, write a `repertoire.yaml`, or scaffold a
private catalog. The skill knows the commands and the safety rules.

## Which command do I want?

| I want to | Command |
| --- | --- |
| Install a skill and remember it | `repertoire add <skill>` |
| Install from a Git URL or a local path | `repertoire add <git-url>` |
| Start a manifest without installing | `repertoire init` |
| See where an installed skill came from | `repertoire show <skill>` |
| Scaffold a catalog repository | `repertoire catalog init` |
| Preview a change | add `--dry-run` |
| Repair what is declared | `repertoire install` |
| Pull newer versions | `repertoire update` |
| Set up a new machine or CI | `repertoire bootstrap` |
| Diagnose a broken install | `repertoire doctor` |

## Supported agents

Agent Skills, Aider, Amp, Antigravity, Claude Code, Cline, CodeBuddy, Codex,
GitHub Copilot, Cursor, Devin, Factory Droid, DeepSeek Harness, Gemini CLI,
Hermes, Junie, Kilo Code, Kimi Code, Kiro, OpenClaw, OpenCode, Pi, Roo Code,
Trae, VS Code, Windsurf, and more. The full target table is in
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

## What it does not do

- It does not host skills. A catalog is a Git repository you control.
- It does not run or sandbox a skill. A skill is instructions; the agent acts.
- It does not manage `CLAUDE.md`, `AGENTS.md` or settings files, except files a
  skill itself ships.
- Signatures are opt-in per catalog. Without a trust block, a digest still
  detects change, but it does not establish who published the change.

## Development

```bash
xdrun ci
uv run zensical build --clean --strict
```

See [Contributing](https://phillarmonic.github.io/repertoire-ai/contributing/).
