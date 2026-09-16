---
title: Set up a project or team
description: Commit a repertoire.yaml so every contributor and CI job installs the same AI agent skills into the same coding agents with one command.
---

# Set up a project or team

Every new laptop and every CI job has to get the same skills into the same
agents. Doing that by hand, or with a per-agent setup script, drifts quickly.

With Repertoire the whole setup is one file and one command: commit a
`repertoire.yaml` that lists the skills the project needs, and each contributor
runs `repertoire bootstrap`.

- One manifest configures every agent your team uses.
- Public, private, and local catalogs all work the same way.
- Installed copies are tracked by digest, so nobody's local edits are silently
  overwritten.

If you only want to install a skill for yourself, skip to
[Install one skill across agents](#install-one-skill-across-agents).

## Step 1: write `repertoire.yaml`

From the Git repository root, start a project manifest without installing
anything:

```bash
repertoire init
```

`init` writes a starter `repertoire.yaml` that lists every built-in
`phillarmonic` skill with source-qualified IDs and `scope: global`. Edit it
down to what the project needs, then run `repertoire bootstrap`. If the file
already has a `skills` section, `init` refuses unless you pass `--force`.

A filled-in example after editing:

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
    targets: [codex, claude, cursor]

  code-reviewer:
    catalog: company
    scope: global
    targets: [codex, claude, cursor, gemini, copilot]

  shared-helpers:
    catalog: company
    scope: project
    targets: [agents]
    hooks: true
```

What each part means:

- `catalogs` registers any catalog beyond the built-in `phillarmonic` one. The
  key is the name you will refer to it by; `source` is anything `git clone`
  accepts (or a local path); `ref` optionally pins a branch, tag, or commit.
  See [Private and company catalogs](private-repositories.md) for how to build
  one.
- `skills` lists what to install. Each key is either a short skill name
  (`code-reviewer`) or a source-qualified ID
  (`github.com/phillarmonic/ai-skills/zensical`) that names the catalog and the
  skill together. Use the qualified form when two catalogs could define the
  same short name.
- `catalog` names which catalog a short name comes from. Omit it for skills in
  the built-in catalog or when only one catalog defines the name.
- `scope` is `global` (default; installs under each contributor's home
  directory) or `project` (installs inside this repository).
- `targets` lists the agents to install into by name. Omit it to let
  Repertoire detect the agents present on each machine. The manifest does not
  accept the `all` shorthand; list the targets you want. The full list is in
  [Targets and security](concepts/targets-security.md).
- `hooks: true` also installs any optional hooks or integrations the skill
  ships (for example an agent hook configuration file). Without it, only the
  skill and any always-on project instructions are installed.

The `tool` line is informational; it tells people who find the file which
program reads it.

## Step 2: run `repertoire bootstrap`

```bash
repertoire bootstrap
```

`bootstrap` reads the `skills` section and installs each entry into its
declared scope and targets. It:

- installs anything that is missing and repairs managed copies that are broken;
- skips skills that are already intact, so repeated runs are cheap;
- never removes a skill you deleted from the file (run `repertoire remove` for
  that);
- never fetches from the network; it uses local catalogs and whatever catalog
  state is already cached.

Put it in your onboarding docs and in CI. It exits non-zero on the first error,
and work completed before that error stays installed.

??? note "Running bootstrap in a repository with no skills declared"
    If `repertoire.yaml` has no `skills` section, `bootstrap` writes a starter
    one that lists every skill from the built-in `phillarmonic` catalog with
    source-qualified IDs and `scope: global`, then installs them. Edit the file
    down to what you actually need and commit it.

## Step 3: keep it fresh with `repertoire sync`

```bash
repertoire sync
```

`sync` does the same work as `bootstrap` but refreshes the tracking catalogs
first, so it picks up skills that were updated upstream. Use `bootstrap` when
you want a deterministic install from current state (onboarding, CI) and `sync`
when you want the newest versions (a weekly job, or whenever a catalog
maintainer announces a change). Skills pinned to a tag or commit stay pinned in
both cases.

## Install one skill across agents

You do not need a manifest to use Repertoire. `add` installs a skill for the
current user and remembers it so `update` keeps it current:

```bash
repertoire add code-reviewer --catalog company --target all
```

`--target all` installs into every supported agent, whether or not that agent
is set up on the machine yet. Repeat `--target` to choose a subset, or omit it
to install only into agents Repertoire detects:

```bash
repertoire add code-reviewer --target codex --target claude
repertoire add code-reviewer
```

Skills install under your home directory by default. Add `--project` when a
skill should live inside the current Git repository instead:

```bash
repertoire --project add shared-helpers --target agents
```

Some catalog skills ship optional hooks or integrations. Interactive `add`
asks before installing them; in CI or scripts pass `--with-hooks` to accept or
`--no-hooks` to skip:

```bash
repertoire --project add graphify --target codex --with-hooks
```

## Details

??? note "Applying every skill to every agent after the fact"
    `install` and `update` accept `--target all` (or repeated `--target`) to
    override the targets stored in the manifest and lock for every skill they
    touch:

    ```bash
    repertoire install --target all
    repertoire update --target all
    ```

    The expanded target names, not the word `all`, are what gets saved.

??? note "Global-scope skills and files in the repository"
    A `scope: global` declaration keeps the skill under the contributor's home
    directory. If the catalog declares always-on project instructions for that
    skill (for example a short pointer section in `AGENTS.md`), `bootstrap`
    still writes those into the repository. Their state is stored in the
    global lock, so the repository does not gain a `repertoire.lock.json` just
    for a pointer. `hooks: true` additionally manages the skill's optional
    hooks in the repository.

??? note "Why bootstrap and sync reject --global and --project"
    Scope belongs to each declaration in the `skills` section, so a
    command-wide scope flag would be ambiguous. Both commands still honor
    `--force`. Replacing a home-directory skill that is already managed from a
    different catalog source or ref requires `--force`; this stops one project
    from silently changing an installation shared across projects.

??? note "Migrating from a legacy .repertoire.yaml"
    Earlier versions read project declarations from a standalone
    `.repertoire.yaml`. When `bootstrap` or `sync` finds that file and
    `repertoire.yaml` declares no skills, it merges the legacy `catalogs` and
    `skills` sections into `repertoire.yaml` and deletes the old file. If both
    files declare skills, `repertoire.yaml` wins and a warning asks you to
    merge and remove the legacy file by hand.

See the [command reference](commands.md), [Manifests and state](concepts/manifests.md),
and [Targets and security](concepts/targets-security.md) for more.
