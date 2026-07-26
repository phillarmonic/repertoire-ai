---
icon: lucide/library
---

# Repertoire

Repertoire automates installing, syncing, and managing portable AI agent skills.
It discovers `SKILL.md` packages in Git-backed catalogs and installs them into
the native home-directory skill roots used by Codex, Claude Code, Cursor,
Gemini CLI, Windsurf, Cline, Roo Code, Kiro, Junie, Kimi Code, OpenCode,
GitHub Copilot, OpenClaw, and shared `.agents` setups. Use `--project` only
when a skill should live inside a Git worktree.

The built-in `phillarmonic` catalog provides Phillarmonic's official vendored
skill set from [phillarmonic/ai-skills](https://github.com/phillarmonic/ai-skills).
Its skills can be referenced without declaring the repository or specifying a
catalog when their names are unique among the visible catalogs. If several
catalogs define the same name, Repertoire lists them and requires an explicit
`--catalog <name>`.

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

For automated team onboarding, declare the required skills and targets in
`.repertoire.yaml`, commit it with the project, and run:

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
