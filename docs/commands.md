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
```

An unqualified short name resolves when exactly one visible catalog defines it.
Namespaced IDs such as `github.com/phillarmonic/ai-skills/zensical` select the
catalog source and short skill name together. If several catalogs define a short
name, Repertoire lists every definition (with namespaced IDs) and requires
`--catalog` or a namespaced ID.

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

## Bootstrap and synchronize a project

From a Git worktree, install every skill declared in `.repertoire.yaml` into its
configured project or global scope:

```bash
repertoire bootstrap
```

If `.repertoire.yaml` is missing, `bootstrap` creates one that lists every skill
from the built-in `phillarmonic` catalog using namespaced IDs and
`scope: global` (manifest stays in the repo; skills install under home-directory
agent roots). `sync` does not create a missing file.

`bootstrap` uses local catalogs and the current catalog cache without fetching.
It skips intact installations and repairs missing managed copies. Use `sync`
when the project should refresh tracking catalogs before updating declarations:

```bash
repertoire sync
```

Both commands process skills by name and stop at the first error. Work completed
before an error remains installed and locked. They never remove skills omitted
from the bootstrap manifest.

Because scope belongs to each declaration, `bootstrap` and `sync` reject
`--global` and `--project`. They continue to honor `--force`. Replacing a
user-global skill that is already managed from another catalog source or ref
also requires `--force`; this prevents one project from silently changing a
shared home installation.

## List

The default view shows installed skills, their catalog, declared or ad-hoc
status, and targets. `--available` refreshes visible catalogs and reads their
manifests instead.

```bash
repertoire list
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

Repertoire does not copy, merge, execute, or print the asset contents. It uses
only installed state from the selected global or `--project` scope and returns
a path only from a complete skill copy matching the lockfile digest. Run
`repertoire install <skill>` to repair missing or locally modified copies.
Namespaced installed skill IDs are supported by treating the final path segment
as the stub name.

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
registrations in global and project scope,
`.repertoire.yaml` bootstrap catalogs, lock-file sources, and cached remotes.
`catalog add` also completes those known source URLs. Typing a namespaced skill
prefix (with `/` or `.`) switches skill completion to namespaced IDs.
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
