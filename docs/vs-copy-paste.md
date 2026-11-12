---
description: How Repertoire compares with copying SKILL.md folders by hand and with a shared skills repository, including the cases where copying is the better answer.
---

# Repertoire vs copy-paste

The alternative to Repertoire is not another package manager. It is the thing
most teams do first: copy a `SKILL.md` folder into whichever agent directory
needs it, and keep a copy in a repository somewhere. That works, for a while. It
is worth knowing exactly where it stops.

## The comparison

| | Copying folders | A shared skills repository | Repertoire |
| --- | --- | --- | --- |
| Installs into 30 target directories | one at a time | one at a time | one command |
| Records what was installed | no | no | manifest and lock |
| Records where it came from | no | the repository URL | catalog, commit, digest |
| Notices a copy that drifted | no | only if someone diffs | digest per target |
| Refuses to overwrite your edit | n/a | n/a | yes, without `--force` |
| Reproduces on a new machine or in CI | no | clone, then copy again | `repertoire bootstrap` |
| Works with a private source | yes | yes | yes, over your Git credentials |
| Verifies a signature over a skill | no | no | opt-in per catalog |
| Sends telemetry | no | no | no |
| Needs no tool at all | **yes** | **yes** | no |
| Time to first skill | seconds | minutes | one install command |

The row where the alternatives win is deliberate. If you use one agent on one
machine, copying a folder is the correct answer and Repertoire is overhead. Do
not install a package manager to solve a problem you do not have.

## When copying is enough

- One coding agent, one machine, one or two skills.
- The skill never changes upstream, or you would not mind if it did.
- Nobody else needs your setup.

Stop copying when a second agent appears, because now there are two directories
to keep in step. Or when a teammate asks which version you run. Or when CI needs
the same skills, because the CI image is a third copy that nobody rebuilds.

## When a shared repository is enough

A repository of `SKILL.md` folders gives you a review trail and a single source,
which is real progress. It does not install anything: someone still copies or
symlinks into each agent directory, and nothing detects a copy that was edited
afterwards. If your team already has this and it is working, keep it. Repertoire
can read that repository as a [loose catalog](concepts/catalogs.md) without
converting it, so adopting it costs one command rather than a migration.

## What the lock adds

The lock is the difference between "we have the skills somewhere" and "this
machine runs the versions we approved":

- A digest per target, so drift on one agent does not hide behind an intact copy
  on another.
- An update that refuses an edited copy and exits non-zero, so CI fails instead
  of shipping it.
- A committed manifest, so a new laptop runs `bootstrap` and gets the same set.

[The skill lockfile](concepts/lockfile.md) has the file format and the exact
hash, if you want to check the mechanism before trusting it.

## Moving over

Five minutes, against a repository you already have:

```bash
# 1. Read the existing repository as a catalog, without converting it
repertoire add /path/to/agent-skills --target all

# 2. What Repertoire now manages, and where each copy came from
repertoire show <skill>

# 3. Commit the manifest so the team and CI can reproduce it
git add repertoire.yaml
repertoire bootstrap
```

Nothing is deleted. Repertoire ignores copies it does not manage, so the old
workflow keeps working while you move skills across one at a time. When a skill
is worth keeping, register the catalog and `add` it so the lock tracks it; until
then the two setups coexist in the same agent directories.

## What Repertoire does not replace

- It does not host skills. A catalog is a Git repository you control.
- It does not run or sandbox a skill. The agent acts on the instructions.
- It does not manage `CLAUDE.md`, `AGENTS.md` or settings files, except files a
  skill itself ships.
- It does not prove a skill is safe. A digest detects change; only an opt-in
  signature starts to establish who published it.
