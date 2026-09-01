# Commands

## Update Repertoire

Update the Repertoire executable to the newest stable GitHub release that
contains a binary for the current operating system and architecture:

```bash
repertoire --self-update
```

The command asks for confirmation, verifies the downloaded binary against the
release `checksums.txt`, checks its reported version, and then replaces the
running executable. The previous executable is retained under
`~/.repertoire/backups`; only the five newest backups are kept. A failed install
or post-install verification restores the backup automatically.

## Declare and install

`add` records a requirement in the selected scope and installs it immediately:

```bash
repertoire add code-reviewer
repertoire add github.com/phillarmonic/ai-skills/zensical
repertoire add code-reviewer --catalog company --target codex --target claude
repertoire --project add graphify --target codex --with-hooks
```

An unqualified short name resolves when exactly one visible catalog defines it.
Source-qualified IDs such as `github.com/phillarmonic/ai-skills/zensical` select the
catalog source and short skill name together. If several catalogs define a short
name, Repertoire lists every definition (with source-qualified IDs) and requires
`--catalog` or a source-qualified ID.

Catalog skills can expose always-on project instructions plus optional hooks
and integrations. Matching instructions are installed for every project-scope
installation. Interactive `add` asks before installing optional artifacts.
Noninteractive use skips optional artifacts unless `--with-hooks` is present;
`--no-hooks` makes the skip explicit. The selected choice is stored for
declared requirements.

## Synchronize

With no argument, `install` installs every declared requirement. A named skill
that is not declared is installed as tracked ad-hoc state without changing the
manifest.

```bash
repertoire install
repertoire install zensical
repertoire install --target all
```

`--target all` installs onto every supported agent target, whether or not that
agent is currently detected. It can be used with a named skill or the bulk
install form above.

Every command accepts a repeatable `--override name=path` flag (or the
`REPERTOIRE_OVERRIDES` environment variable) to redirect a catalog to a local
checkout, so skills can be tested before pushing. Flags win over environment
values; `catalog list` marks overridden sources.

## Bootstrap and synchronize a project

From a Git worktree, install every skill declared in the `skills` section of
the project `repertoire.yaml` into its configured project or global scope:

```bash
repertoire bootstrap
```

If `repertoire.yaml` declares no bootstrap skills, `bootstrap` adds a starter
`skills` section that lists every skill from the built-in `phillarmonic`
catalog using source-qualified IDs and `scope: global` (manifest stays in the
repo; skills install under home-directory agent roots). `sync` does not create
missing declarations. A legacy `.repertoire.yaml` is merged into
`repertoire.yaml` and removed automatically on either command; when both files
declare skills, the legacy file is ignored with a warning.

`bootstrap` uses local catalogs and the current catalog cache without fetching.
It skips intact installations and repairs missing managed copies. Use `sync`
when the project should refresh tracking catalogs before updating declarations:

```bash
repertoire sync
```

For a global-scope skill, bootstrap may also manage small catalog-declared
instruction pointers in the worktree. Their state is stored in the global
Repertoire lock, so the repository does not gain a project lock merely for the
pointer. `hooks: true` in the bootstrap declaration additionally installs
optional hooks and integrations into that worktree.

Both commands process skills by name and stop at the first error. Work completed
before an error remains installed and locked. They never remove skills omitted
from the `skills` section.

Because scope belongs to each declaration, `bootstrap` and `sync` reject
`--global` and `--project`. They continue to honor `--force`. Replacing a
user-global skill that is already managed from another catalog source or ref
also requires `--force`; this prevents one project from silently changing a
shared home installation.

## Diagnose and repair

`doctor` audits the current project and the global installation for broken or
stale managed state: missing or locally modified managed files, files managed
by multiple skills with conflicting content, managed Markdown sections no
lock entry claims, identical sections duplicated under per-target markers,
declarations in `repertoire.yaml` whose lock state does not match,
global-lock entries for projects that no longer exist, and broken global
skill installs.

```bash
repertoire doctor
```

Report mode lists every issue with a suggested remedy and exits non-zero when
anything is found, so it can gate CI. `repertoire doctor --fix` repairs what
it finds: managed content is reinstalled from the catalog cache, orphaned and
duplicated sections are collapsed or removed, drift is reconciled through the
same path as `repertoire bootstrap`, and stale lock entries are pruned. When
the current directory is not a Git worktree, project checks are skipped and
only global hygiene runs.

A `conflicting-destination` report means two skills copy-manage the same
file with different content, so no reinstall can satisfy both. Doctor repairs
it only once the catalogs resolve compatibly (for example after the skills
switch to `markdown-section`); until then it reports the conflict instead of
flip-flopping modification warnings.

`repertoire doctor --reset` is the clean-slate escape hatch: it removes every
managed artifact for the current project and reinstalls from `repertoire.yaml`.
It asks for confirmation unless `--yes` is given.

```bash
repertoire doctor --fix
repertoire doctor --reset --yes
repertoire doctor --format json
```

Output is a table in a terminal, TSV when redirected, or JSON with
`--format json`. Like `bootstrap` and `sync`, `doctor` rejects `--global` and
`--project`; it always inspects both scopes.

## List

In a terminal, the default view shows a compact table of installed skills,
their catalog, declared or ad-hoc origin, and a target summary. Use `--wide`
to show every target. Redirected output remains headerless TSV for compatibility
with scripts. `--format table`, `--format tsv`, and `--format json` select a
format explicitly.

```bash
repertoire list
repertoire list --wide
repertoire list --format json
repertoire list --available
repertoire list --available --catalog phillarmonic
```

## Discover and get stubs

Installed skills may expose small file-backed stubs. List every stub in the
selected scope, or limit the result to one installed skill:

```bash
repertoire stub list
repertoire stub list common-stubs
```

Ask for a specific stub with its explicit `<skill>/<stub>` identifier:

```bash
repertoire stub get common-stubs/editorconfig
```

The command prints a stable handoff containing the stub identifier,
description, absolute asset path, and authored instructions:

```text
Stub: common-stubs/editorconfig
Description: Ensure text files end with a newline.
Asset: /home/user/.agents/skills/common-stubs/assets/.editorconfig
Instructions:
Create or update the repository-root .editorconfig ...
```

Repertoire does not copy, merge, execute, or print the asset contents by
default. It uses only installed state from the selected global or `--project`
scope and returns a path only from a complete skill copy matching the lockfile
digest. Run `repertoire install <skill>` to repair missing or locally modified
copies. Namespaced installed skill IDs are supported by treating the final path
segment as the stub name.

When an agent needs to materialize the asset directly, pass `--raw` to write
only the asset bytes to stdout, which is safe to redirect into a file:

```bash
repertoire stub get --raw common-stubs/gitattributes > .gitattributes
```

Use the default advisory output when the stub instructions require merging into
an existing file rather than a wholesale replacement.

## Update and remove

`update` refreshes tracking catalogs and reinstalls one or every installed
skill. With a catalog name, it refreshes that catalog even when no installed
skill has the same name. Missing managed copies are repaired. Tags and commit
refs remain pinned.

```bash
repertoire update
repertoire update code-reviewer
repertoire update company
repertoire update --target all
repertoire update graphify --with-hooks
repertoire update graphify --no-hooks
repertoire remove code-reviewer
```

The update command also accepts `--target all` (or repeated individual
`--target` flags) to replace the stored target set while updating.

Updates and removals refuse locally modified targets. Review the changes before
using `--force` to replace or delete them.

All commands default to user-global scope (home-directory skill roots). Use
`--project` to install into the current Git worktree, or `--global` to make the
default explicit.

## Shell completion

Repertoire generates context-aware completion scripts for Bash, Zsh, Fish, and
PowerShell. Completions suggest installed skills, available skills from local or
already-cached catalogs, agent targets, and known catalogs: the built-in catalog,
registrations in global and project scope, bootstrap catalogs declared in the
project `repertoire.yaml` (or a legacy `.repertoire.yaml` awaiting migration),
lock-file sources, and cached remotes.
`catalog add` also completes those known source URLs. Typing a source-qualified
skill prefix (with `/` or `.`) switches skill completion to source-qualified
IDs.
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
