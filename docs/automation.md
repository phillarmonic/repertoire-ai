---
title: Automate AI agent skill installation
description: Install and synchronize portable SKILL.md packages across Codex, Claude Code, Cursor, Gemini CLI, Copilot, Windsurf, and other coding agents.
---

# Automate AI agent skill installation

Repertoire replaces per-agent setup scripts with one reproducible workflow for
installing and updating Agent Skills. Use it when a team needs to distribute the
same `SKILL.md` package across several AI coding assistants, terminal agents, or
agentic IDEs.

## Install a skill across coding agents

Use `all` to install into every supported native target:

```bash
repertoire add code-reviewer --catalog company --target all
```

Repertoire resolves the skill from its Git-backed catalog, validates the
package, and installs a managed copy into every target even when a client's
configuration directory does not exist yet. It supports Codex,
Claude Code, GitHub Copilot, Cursor, Gemini CLI, Windsurf, Cline, Roo Code,
Kiro, Junie, Kimi Code, OpenCode, OpenClaw, and the portable `.agents/skills`
layout.

Catalogs can also provide platform-specific variants, always-on project
instructions, and optional managed hooks. Instructions install automatically
for project scope. Use `--with-hooks` in CI or another noninteractive
environment when optional integrations are required:

```bash
repertoire --project add graphify --target codex --with-hooks
```

Repeat `--target` with individual names to install only a subset:

```bash
repertoire add code-reviewer --target codex --target claude
```

Skills install to the home directory by default. Use `--project` only when a
skill should live inside the Git worktree:

```bash
repertoire add code-reviewer --target codex --target claude
repertoire add shared-helpers --project --target agents
```

## Automate developer and CI setup

Commit `.repertoire.yaml` at the project root:

```yaml
schema: 1
tool: https://github.com/phillarmonic/repertoire-ai

catalogs:
  company:
    source: git@github.com:example/company-skills.git
    ref: main

skills:
  github.com/example/company-skills/code-reviewer:
    scope: global
    targets: [codex, claude, cursor, gemini, copilot]
  graphify:
    catalog: company
    scope: global
    targets: [codex]
    hooks: true
```

Then bootstrap every declared skill:

```bash
repertoire bootstrap
```

If `.repertoire.yaml` is missing, `bootstrap` creates a starter from the built-in
catalog (source-qualified IDs, `scope: global`) and installs those skills. The command
repairs missing managed copies. Run `repertoire sync` when the automation should
fetch catalog changes and update the declared installations.

Global declarations keep skill bundles under the user's home directory.
Catalog-provided instruction pointers and any explicitly enabled hooks are
managed in the worktree; their state remains in Repertoire's global lock.

To override the targets stored in the manifest or lock and apply every skill to
every supported agent, use:

```bash
repertoire install --target all
repertoire update --target all
```

## Why automate with Repertoire?

- One manifest configures multiple AI coding agents.
- Public, private, and local Git catalogs use the same workflow.
- Content digests protect locally modified skills from accidental replacement.
- Atomic installation prevents partial skill directories.
- Skill scripts are copied as data and never executed during installation.
- Shell completion exposes available catalogs, skills, and agent targets.

See [commands](commands.md), [manifests and state](concepts/manifests.md), and
[targets and security](concepts/targets-security.md) for the full reference.
