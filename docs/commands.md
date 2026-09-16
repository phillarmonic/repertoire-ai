---
title: Command reference
description: Every Repertoire command with its purpose, common forms, and edge cases.
---

# Command reference

Commands are listed in the order most people meet them. Each section opens
with what the command is for and its common forms; less common behavior sits
in collapsible notes. Terms such as catalog, target, and scope are defined in
the [glossary](glossary.md).

## Flags every command accepts

`--project` / `--global`
:   Choose the scope. Global is the default: state lives in your user
    configuration directory and skills install under home-directory agent
    roots. `--project` reads `repertoire.yaml` and `repertoire.lock.json` from
    the current Git worktree root and installs into project-local agent
    directories. The two flags cannot be combined.

`--force`
:   Replace or remove a managed copy that Repertoire would otherwise protect
    because it is locally modified, unmanaged, or managed from a different
    catalog source. Review the destination before using it.

`--dry-run`
:   Print the writes and refusals a mutating command would perform, and leave
    the filesystem, lock, and manifest unchanged. `list`, `show`, `doctor`,
    `stub`, and `completion` ignore the flag and print a note on stderr.

`--override name=path`
:   Resolve a catalog from a local checkout instead of its registered remote.
    Repeatable, or set `REPERTOIRE_OVERRIDES="name=path,other=path"`. Flags win
    over the environment variable, and `catalog list` marks overridden sources.
    See [Local overrides for testing](concepts/catalogs.md#local-overrides-for-testing).

## `add`: install a skill and remember it

`add` records the skill as a requirement in the selected scope and installs it
right away. Later, `install` and `update` know to keep it.

```bash
repertoire add code-reviewer
repertoire add code-reviewer --target codex --target claude
repertoire add code-reviewer --catalog company --target all
repertoire add github.com/phillarmonic/ai-skills/zensical
repertoire --dry-run add code-reviewer --target agents
```

Without `--target`, `add` installs into the agents it detects on your machine
(an existing configuration directory, or a well-known CLI on `PATH` in global
scope). `--target all` installs into every supported agent regardless of
detection.

Install several skills at once with comma-separated names, multiple
arguments, or a glob pattern matched against the skills your catalogs offer.
Quote patterns so the shell does not expand them; a pattern that matches
nothing is an error.

```bash
repertoire add code-reviewer,shared-helpers
repertoire add "product-*"
```

### Add from a source

A Git URL, `owner/repo` GitHub shorthand, or local path is a catalog source.
`add` registers it in the selected scope (name from the repo basename, or
`--name`) and installs every skill it offers. Repeat `--skill` or append
`/<skill>` (or a GitHub `/tree/<ref>/<path>` tail) to install a subset.

```bash
repertoire add /path/to/agent-skills --target agents
repertoire add https://github.com/example/agent-skills.git --name company --skill code-reviewer
repertoire add github.com/example/agent-skills/code-reviewer --target all
```

Short names and source-qualified IDs still resolve first. `add zensical` and
`add github.com/phillarmonic/ai-skills/zensical` are unchanged. If the derived
catalog name is already used by a different source, pass `--name`.

### How skill names resolve

- A short name such as `code-reviewer` resolves when exactly one visible
  catalog defines it. If the built-in `phillarmonic` catalog defines the name,
  it wins over other catalogs.
- A source-qualified ID such as `github.com/phillarmonic/ai-skills/zensical`
  names the catalog source and the skill together and is never ambiguous.
- If several non-mainline catalogs define the same short name, Repertoire
  lists every match with its source-qualified ID and asks you to choose with
  `--catalog <name>` or the qualified ID.

### Optional hooks and integrations

Some skills ship always-on project instructions (installed automatically for
project scope) plus optional hooks or integration files. Interactive `add`
asks before installing the optional ones. In scripts and CI, pass
`--with-hooks` to accept or `--no-hooks` to skip; the choice is stored with
the requirement.

```bash
repertoire --project add graphify --target codex --with-hooks
```

## `install`: reinstall or repair

Where `add` declares a new requirement, `install` (re)installs what is already
declared. With no argument it installs every requirement in the selected
scope, repairing any managed copy that is missing or broken.

```bash
repertoire install
repertoire install zensical
repertoire install --target all
```

A named skill that is not declared is installed as tracked ad-hoc state; the
manifest is not changed. `--target all` applies to every skill the command
touches and replaces the stored target set.

## `update`: pull newer versions

`update` refreshes the tracking catalogs and reinstalls one or every installed
skill. Missing managed copies are repaired along the way. Skills pinned to a
tag or commit stay pinned.

```bash
repertoire update
repertoire update code-reviewer
repertoire update --target all
repertoire --dry-run update code-reviewer
```

Give a catalog name to refresh that catalog even when no installed skill
shares the name:

```bash
repertoire update company
```

`update` refuses to replace a locally modified copy. Review your changes, then
either move them into your own catalog or rerun with `--force`. `--with-hooks`
and `--no-hooks` add or remove a skill's optional hooks during the update.

??? note "Network behavior"
    When nothing is installed and the manifest declares no skills, `update`
    does nothing and never touches the network. All Git operations run with
    terminal prompts disabled, so a catalog that needs credentials fails with
    a clear error instead of hanging on a password prompt.

## `remove`: uninstall a skill

```bash
repertoire remove code-reviewer
repertoire --dry-run remove code-reviewer
```

Removes the managed copies from every target in the selected scope and drops
the requirement. Like `update`, it refuses to delete a locally modified copy
without `--force`.

## `list`: what is installed, what is available

```bash
repertoire list
repertoire list --wide
repertoire list --available
repertoire list --available --catalog phillarmonic
```

The default view is a compact table of installed skills with their catalog,
whether they were declared or installed ad hoc, and a target summary; `--wide`
shows every target. `--available` refreshes catalogs and lists the skills
they offer, optionally limited to one catalog.

Output is a table in a terminal and headerless TSV when redirected, so it is
safe to pipe. Force a format with `--format table`, `--format tsv`, or
`--format json`.

## `show`: where a skill came from and whether it is intact

```bash
repertoire show code-reviewer
repertoire show code-reviewer --format json
```

Prints the catalog, redacted source, ref, resolved commit, content digest,
whether the skill is declared or ad hoc, the hooks choice, and one row per
target with the installed path and status (`intact`, `modified`, or
`missing`). A loose catalog is marked as such. If the catalog cache is
absent, lock data still prints and `catalog_cache` is `absent`.

`--format` is the same as `list`: table in a terminal, TSV when redirected,
or `--format json` for a single object scripts can parse.

## `catalog`: manage where skills come from

```bash
repertoire catalog list
repertoire catalog init company --skill code-reviewer --skill shared-helpers
repertoire catalog add git@github.com:example/private-skills.git --name company
repertoire catalog add github.com/example/public-skills --name public --ref main
repertoire catalog add /path/to/ai-skills
repertoire --dry-run catalog add https://github.com/example/public-skills.git --name public
repertoire catalog update
repertoire catalog remove company
```

`init` writes a catalog `repertoire.yaml` and `SKILL.md` placeholders into the
current directory. It does not run Git. Omit `[name]` to derive the catalog
name from the directory basename (lower-case kebab). Omit `--skill` to create
one example skill named `<name>-example`. An existing `repertoire.yaml` is
refused unless you pass `--force`. See
[Private and company catalogs](private-repositories.md).

`add` registers a Git URL or a local path. `--ref` pins a branch, tag, or
commit; without it the remote default branch is tracked. `update` refreshes
the cached clones. Remote catalogs are read with your system `git`, so SSH
agents, credential helpers, and provider CLIs work without Repertoire storing
anything.

A local path (or clone) that has no `repertoire.yaml` catalog section is still
accepted as a [loose catalog](concepts/catalogs.md#loose-catalogs). Repertoire
discovers `SKILL.md` directories, synthesizes the catalog in memory, and marks
the source `(loose)` in `catalog list` and `list --available`. Pass `--name`
when the directory basename is not a valid catalog name.

## `bootstrap` and `sync`: install what a project declares

From a Git worktree, `bootstrap` installs every skill in the `skills` section
of the project `repertoire.yaml` into its declared scope and targets, without
fetching. `sync` does the same after refreshing the tracking catalogs.

```bash
repertoire bootstrap
repertoire sync
repertoire --dry-run bootstrap
```

Both skip intact installations, repair missing copies, stop at the first
error (work already done stays installed), and never remove skills that were
dropped from the manifest. Because scope is declared per skill, they reject
`--global` and `--project`. The full walkthrough, including the manifest
format, is in [Set up a project or team](automation.md).

??? note "Starter manifest and legacy .repertoire.yaml"
    If `repertoire.yaml` declares no skills, `bootstrap` (not `sync`) writes a
    starter `skills` section listing every built-in `phillarmonic` skill with
    source-qualified IDs and `scope: global`, then installs them.

    A legacy `.repertoire.yaml` found beside a `repertoire.yaml` with no
    skills is merged into `repertoire.yaml` and deleted by either command.
    When both files declare skills, `repertoire.yaml` wins and the legacy file
    is ignored with a warning.

??? note "Global skills, project pointers, and --force"
    For a `scope: global` declaration the skill stays under the home
    directory, but catalog-declared project instructions (small pointer
    sections) are still written into the worktree and tracked in the global
    lock. `hooks: true` also installs the skill's optional hooks there.
    Replacing a home-directory skill already managed from a different catalog
    source or ref requires `--force`, so one project cannot silently change an
    installation shared by others.

## `init`: start a project manifest

```bash
repertoire init
```

From a Git worktree, `init` writes a project `repertoire.yaml` with the same
starter `skills` section that `bootstrap` generates when none is present:
every built-in `phillarmonic` skill, source-qualified IDs, `scope: global`.
It does not install anything. Edit the file, then run `repertoire bootstrap`.

If the file already declares skills, `init` refuses unless you pass `--force`.
`--force` replaces the `skills` section and leaves `catalogs` and
`requirements` in place. `--global` is rejected; this command is project-only.

See [Set up a project or team](automation.md).

## `doctor`: diagnose and repair

`doctor` audits both the current project and the global installation and
reports anything broken or stale, each with a suggested remedy. Start here
when something looks wrong.

```bash
repertoire doctor
```

It checks for managed files that are missing or locally modified, files
managed by two skills with conflicting content, managed Markdown sections that
no lock entry claims, duplicated sections, declarations whose lock state has
drifted, global-lock entries for projects that no longer exist, and broken
global skill installs. It exits non-zero when anything is found, so it can
gate CI. Outside a Git worktree only the global checks run.

Escalate as needed:

```bash
repertoire doctor --fix                 # repair what it finds
repertoire doctor --reset --yes         # reinstall every managed artifact for this project
repertoire doctor --reset --global --yes  # wipe all local Repertoire state
repertoire doctor --format json
```

- `--fix` reinstalls managed content from the catalog cache, collapses or
  removes orphaned and duplicated sections, reconciles drift the same way
  `bootstrap` does, and prunes stale lock entries.
- `--reset` removes every managed artifact for the current project and
  reinstalls from `repertoire.yaml`. It asks for confirmation unless `--yes`
  is given.
- `--reset --global` removes every globally managed skill, wipes the global
  configuration directory (`repertoire.yaml` and `repertoire.lock.json`), and
  clears the catalog cache. Nothing is reinstalled afterwards, even with
  `--fix`. Use it when a stale global install points at catalogs that no
  longer exist.

`doctor` always inspects both scopes, so it rejects `--global` and
`--project` except in the `--reset --global` form above. Output is a table,
TSV when redirected, or JSON with `--format json`.

??? note "conflicting-destination"
    This report means two skills copy-manage the same file with different
    content, so no reinstall can satisfy both. `doctor` repairs it only once
    the catalogs resolve compatibly (for example after the skills switch to
    `markdown-section` mode); until then it reports the conflict rather than
    flip-flopping between the two.

??? note "After --reset --global"
    Skills are removed based on the global lock. If that lock was already
    lost, skill directories left in agent roots are simply unmanaged;
    reinstall over them with `repertoire add <skill> --force` or delete them
    by hand.

## `stub`: starter files from installed skills

Some skills ship small file stubs (an `.editorconfig`, a `.gitattributes`)
with instructions for how an agent should apply them.

```bash
repertoire stub list
repertoire stub list common-stubs
repertoire stub get common-stubs/editorconfig
repertoire stub get --raw common-stubs/gitattributes > .gitattributes
```

`stub get` prints a handoff with the stub ID, description, absolute asset
path, and the author's instructions:

```text
Stub: common-stubs/editorconfig
Description: Ensure text files end with a newline.
Asset: /home/user/.agents/skills/common-stubs/assets/.editorconfig
Instructions:
Create or update the repository-root .editorconfig ...
```

By default Repertoire does not copy, merge, execute, or print the asset; the
agent reads the path and follows the instructions, which matters when the
stub has to be merged into an existing file. `--raw` writes only the asset
bytes to stdout for direct redirection. Paths are returned only from a
complete installed copy that matches the lock digest; run
`repertoire install <skill>` to repair one that does not.

## `--self-update`: update Repertoire itself

```bash
repertoire --self-update
```

Downloads the newest stable release for your OS and architecture, verifies it
against the release `checksums.txt`, checks its reported version, and replaces
the running executable after asking for confirmation. The previous executable
is kept under `~/.repertoire/backups` (five newest), and a failed install or
verification restores it automatically.

## Shell completion

Repertoire generates context-aware completion for Bash, Zsh, Fish, and
PowerShell. Completions suggest installed skills (`show`, `update`, `remove`),
skills from local or cached catalogs (`add`, `install`), agent targets, and
known catalogs (built-in, registered in either scope, declared in the project
`repertoire.yaml`, recorded in lock files, or cached). `catalog add` completes
known source URLs. `add --skill` completes skills offered by the source
argument when that catalog is already cached or is a local path. `init` and
`catalog init` do not complete positional arguments as files. Typing a prefix
that contains `/` or `.` switches skill completion to source-qualified IDs.
Completion never clones or refreshes a catalog.

Enable completion for the current shell session:

```bash
# Bash
source <(repertoire completion bash)

# Zsh
source <(repertoire completion zsh)

# Fish
repertoire completion fish | source

# PowerShell
repertoire completion powershell | Out-String | Invoke-Expression
```

For persistent completion, write the generated script to the completion
directory used by the shell:

```bash
repertoire completion bash > ~/.local/share/bash-completion/completions/repertoire
repertoire completion zsh > "${fpath[1]}/_repertoire"
repertoire completion fish > ~/.config/fish/completions/repertoire.fish
```
