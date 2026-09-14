---
title: Troubleshooting
description: What Repertoire's common error messages mean and the command that fixes each one.
---

# Troubleshooting

Each section is named after the message or symptom you see, followed by what it
means and what to do.

## Start with `repertoire doctor`

If you are not sure what is wrong, run:

```bash
repertoire doctor
```

It audits both the current project and your global installation and prints
each problem with a suggested remedy. `repertoire doctor --fix` applies those
remedies. See [`doctor`](commands.md#doctor-diagnose-and-repair) for the
escalation path from there.

## `no supported agent clients detected; use --target`

**What it means.** You ran `add` without `--target`, and Repertoire could not
find any agent on this machine (no known configuration directory, no known CLI
on `PATH`).

**What to do.** Name the agents you want:

```bash
repertoire add <skill> --target codex --target claude
repertoire add <skill> --target agents    # the portable .agents/skills layout
repertoire add <skill> --target all       # every supported agent
```

The full list of target names is in [Targets and security](concepts/targets-security.md).

## `skill "<name>" is defined in multiple catalogs`

**What it means.** Two or more catalogs you have registered define the same
short name, and the built-in `phillarmonic` catalog does not (if it did,
Repertoire would pick that definition). The error lists every match with its
source-qualified ID.

**What to do.** Say which one you mean, either with the catalog name or the
source-qualified ID from the list:

```bash
repertoire add code-reviewer --catalog company
repertoire add github.com/company/agent-skills/code-reviewer
```

## `skill "<name>" was not found`

**What it means.** No configured catalog offers a skill with that name.

**What to do.** Check the spelling against `repertoire list --available`. If
the skill lives in a catalog you have not registered yet, add it first with
`repertoire catalog add <source> --name <name>`. If the catalog was recently
updated, run `repertoire catalog update` to refresh the cache.

## `target is unmanaged or locally modified; use --force`

**What it means.** The destination already has content Repertoire does not
own, or a managed copy that you edited since it was installed. Repertoire
refuses to overwrite it.

**What to do.** Look at the directory named in the error. If the content is
yours and you want to keep it, leave it and skip that target or copy your
edits into a skill in your own catalog. If you are happy to discard it, rerun
with `--force`.

## `target is locally modified; use --force` (on remove)

Same cause as above, on `remove`. Review the copy, then rerun with `--force`
to delete it anyway.

## I edited an installed skill and now `update` refuses to touch it

**What it means.** This is by design. Every managed copy is tracked by content
digest, so Repertoire can tell the copy differs from what it installed and will
not replace your work silently.

**What to do.** Pick one:

- Move your edits upstream: put the changed skill in a catalog you control
  (see [Private and company catalogs](private-repositories.md)) and install
  from there. This is the durable fix.
- Discard your edits: `repertoire update <skill> --force`.
- Keep the edited copy but stop Repertoire from managing it:
  `repertoire remove <skill> --force` deletes the managed copy, so back it up
  first and put it back afterwards as a plain, unmanaged directory.

A bulk `repertoire update` stops at the first skill it cannot replace, so fix
or force that one before the rest will update.

## Where did my skill get installed?

**What it means.** You are looking for the files on disk.

**What to do.** `repertoire list --wide` shows every target for each skill.
Skills install into home-directory roots by default (for example
`~/.claude/skills`, `~/.codex/skills`, `~/.cursor/skills`); with `--project`
they install into the equivalent directory inside the Git repository (for
example `.claude/skills`). The per-target paths are in
[Targets and security](concepts/targets-security.md).

## A private catalog cannot be cloned or fails with `could not read Username`

**What it means.** Git could not authenticate to the catalog remote.
Repertoire runs Git with prompts disabled, so a missing credential fails
immediately rather than waiting for input.

**What to do.** Test the URL with plain Git:

```bash
git ls-remote <source>
```

Fix your SSH agent or Git credential helper until that succeeds, then run
`repertoire catalog update`. See
[Check Git access](private-repositories.md#check-git-access).

## A skill is missing or broken after an update or a manual deletion

**What to do.** Reinstall it from the locked catalog source:

```bash
repertoire install <skill>
```

or run `repertoire update <skill>` to repair and refresh in one step. For a
whole project, `repertoire bootstrap` repairs every declared skill.

## Managed files are missing, modified, or duplicated across agents

**What it means.** Skills that write shared files (for example a section in
`AGENTS.md` or a hooks configuration) can drift when several agents are
selected or when files are edited by hand.

**What to do.**

```bash
repertoire doctor          # see what is wrong
repertoire doctor --fix    # repair it
repertoire doctor --reset --yes   # last resort: reinstall everything for this project
```

## `--project requires a Git worktree`

**What it means.** You passed `--project` outside a Git repository.

**What to do.** Run the command from inside the repository, or drop
`--project` to install globally.
