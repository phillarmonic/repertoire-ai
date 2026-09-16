---
title: Let your agent drive Repertoire
description: Install the built-in repertoire skill so your AI coding agent can install skills, write manifests, and scaffold new skill catalogs for you.
---

# Let your agent drive Repertoire

Repertoire ships with a skill about itself. It lives in the built-in
`phillarmonic` catalog under the name `repertoire`, and it teaches an AI coding
agent how to run Repertoire correctly: which command to use for which job, how
the manifests are shaped, and which safety rules never to bypass.

Once it is installed you stop typing Repertoire commands yourself and describe
the outcome you want instead.

## Install the skill

```bash
repertoire add repertoire --target all
```

Drop `--target all` to install only into the agents Repertoire detects on your
machine, or name them: `--target cursor --target claude`. Like any other skill
it is kept current by `repertoire update`.

To make sure every contributor's agent has it, declare it in the project
manifest alongside your other skills:

```yaml
skills:
  github.com/phillarmonic/ai-skills/repertoire:
    scope: global
    targets: [codex, claude, cursor]
```

See [Set up a project or team](automation.md) for the rest of the file.

## What the agent can do for you

Everything in these docs, on request. The skill covers:

**Install and manage skills**
:   "Install the `zensical` skill for Codex and Claude." "Update everything."
    "Remove `graphify` from this project." The agent picks the right scope
    and targets, checks `repertoire list` first, and uses `add` rather than
    editing files by hand.

**Set up a project**
:   "Add a `repertoire.yaml` to this repository so new contributors get our
    skills with `repertoire bootstrap`." The agent prefers `repertoire init`,
    then edits the starter to source-qualified IDs and `scope: global`, then
    runs `bootstrap` to verify it. It can preview with `--dry-run` first.

**Create a new skill catalog repository**
:   "Create a private skill catalog for our team with a `code-reviewer`
    skill." The agent prefers `repertoire catalog init`, then fills in each
    `SKILL.md`, following the naming rules in
    [Private and company catalogs](private-repositories.md). This is where the
    skill saves the most time.

**Inspect provenance**
:   "Where did `zensical` come from, and did anyone edit the Cursor copy?"
    The agent runs `repertoire show zensical` instead of reading the lock file
    by hand.

**Test catalog changes before pushing**
:   "Try my catalog changes locally." The agent uses `--override` or a
    disposable project-scope registration so your global state is untouched,
    then installs a skill from the local checkout to confirm it resolves.

**Author advanced catalog features**
:   Platform variants, always-on project instructions, optional hooks, and
    `stubs.yaml` starter files. The agent knows the schema and the artifact
    modes (`copy`, `markdown-section`, `json-merge`).

**Diagnose problems**
:   "Why does `update` refuse to touch `zensical`?" The agent recognises the
    managed-copy safety errors, explains them, and asks before using
    `--force`.

## What the agent will not do

The skill encodes the same guardrails the CLI enforces, so an agent following
it:

- inspects existing state (`repertoire list`, `repertoire catalog list`)
  before changing anything;
- never edits `repertoire.lock.json` or a managed skill copy directly;
- never combines `--project` and `--global`;
- does not pick silently when a skill name is ambiguous; it repeats the
  command with the catalog you intended;
- does not use `--force` to discard local changes just to make a command
  pass, and confirms before replacing a shared global installation;
- never puts tokens or passwords in catalog URLs or manifests;
- reports the scope, catalog, and targets it used when it finishes;
- prefers `--dry-run` before any `--force`.

## Example session

A typical exchange after the skill is installed:

> **You:** Create a private skill catalog repo at `~/work/acme-skills` with a
> `acme-pr-review` skill that checks pull requests against our style guide in
> `STYLE.md`, then install it for Cursor in this project.

The agent will create the directory, `git init`, write
`repertoire.yaml` with `catalog.name: acme` and the skill entry, write
`skills/acme-pr-review/SKILL.md` with matching `name` and a non-empty
`description`, test it with a local override, register it with
`repertoire --project catalog add ~/work/acme-skills --name acme`, run
`repertoire --project add acme-pr-review --catalog acme --target cursor`, and
show you the resulting `repertoire.yaml` and `repertoire.lock.json` changes
before it declares the job done. Pushing to a private remote and switching the
registration to the Git URL is the one step it will ask you about, because it
needs your host and access decisions.

## Keep it current

The skill is versioned with the rest of the built-in catalog, so
`repertoire update` (or `repertoire sync` in a project) picks up new guidance
when Repertoire gains features. If your agent suggests a command these docs
do not mention, run `repertoire update repertoire` and try again.
