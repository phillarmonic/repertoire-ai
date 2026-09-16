---
title: Glossary
description: Definitions of the terms Repertoire uses: skill, catalog, loose catalog, target, scope, manifest, lock file, managed copy, source-qualified ID, provenance, and dry run.
---

# Glossary

If you have used a Linux package manager, the model is familiar: catalogs are
repositories, `add` installs a package, and `repertoire.yaml` is the manifest
you commit so others get the same set. Terms defined here are underlined
throughout the documentation; hover one for the short definition.

## Skill

A directory containing a `SKILL.md` with `name` and `description` frontmatter,
followed by instructions for an AI coding agent. It may include supporting
files such as templates or scripts.

The format is shared by many agents, which is what makes skills portable: the
same directory works in Codex, Claude Code, Cursor, Gemini CLI, and the rest.
Each agent simply looks for it in a different place.

## Catalog

A Git repository (or a local folder) that publishes skills through a
`repertoire.yaml` with a `catalog:` section.

The built-in `phillarmonic` catalog is preconfigured and holds
[Phillarmonic's official skills](https://github.com/phillarmonic/ai-skills).
You can register public, private, or local catalogs of your own with
`repertoire catalog add`. See [Catalogs](concepts/catalogs.md) and
[Private and company catalogs](private-repositories.md).

## Loose catalog

A Git repository (or local folder) of `SKILL.md` directories with no
`repertoire.yaml` catalog section. Repertoire discovers skills under a bounded
walk (skills trees, agent skill dirs, and marketplace layouts such as
`plugins/<plugin>/skills/<skill>`) and synthesizes a catalog in memory so you
can still register, install, and lock it. A written catalog manifest is what
unlocks variants, project instructions, hooks, and stubs.

## Target

One agent's skills folder, named after the agent: `codex`, `claude`, `cursor`,
`gemini`, and so on.

`--target all` means every supported agent. With no `--target`, `add` picks
the agents it detects on your machine. The full list of names and install
paths is in [Targets and security](concepts/targets-security.md).

## Scope

Where a skill lives. **Global** (the default) installs under your home
directory, so the skill is available in every project. **Project**
(`--project`) installs inside the current Git repository, for skills that
should ship with that repository.

## Manifest

The `repertoire.yaml` file that says what you want installed. In a project it
lists the catalogs and skills contributors need; in your user configuration
directory it records the skills you added globally. A catalog also uses a
`repertoire.yaml`, with a `catalog:` section, to publish skills.
See [Manifests and state](concepts/manifests.md).

## Lock file

`repertoire.lock.json`, written by Repertoire next to the manifest. It records
exactly what was installed: catalog commit, content digest, and target paths.
The lock file is how Repertoire notices when you have edited an installed
copy. Never edit it by hand.

## Managed

An installed copy that Repertoire owns and tracks in the lock file. Updates
and removals refuse to touch a managed copy you have modified locally unless
you pass `--force`.

## Source-qualified ID

A skill name prefixed with its catalog's host and path, such as
`github.com/phillarmonic/ai-skills/zensical`. It names the catalog and the
skill together, so it is never ambiguous when two catalogs define the same
short name.

## Provenance

Where an installed skill came from and whether each managed copy still matches
the lock: catalog name, source, commit, content digest, targets, and
intact / modified / missing status. `repertoire show <skill>` prints it.

## Dry run

`--dry-run` on a mutating command prints the writes and refusals that would
happen, and performs no disk, lock, or manifest changes.
