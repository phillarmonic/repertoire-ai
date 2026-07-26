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
repertoire add code-reviewer --catalog company --target codex --target claude
```

An unqualified name must occur in exactly one visible catalog.

## Synchronize

With no argument, `install` installs every declared requirement. A named skill
that is not declared is installed as tracked ad-hoc state without changing the
manifest.

```bash
repertoire install
repertoire install zensical --catalog phillarmonic
repertoire install --target all
```

`--target all` installs onto every supported agent target, whether or not that
agent is currently detected. It can be used with a named skill or the bulk
install form above.

## Bootstrap and synchronize a project

From a Git worktree containing `.repertoire.yaml`, install every declared skill
into its configured project or global scope:

```bash
repertoire bootstrap
```

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
status, and targets. `--available` reads catalog manifests instead.

```bash
repertoire list
repertoire list --available
repertoire list --available --catalog phillarmonic
```

## Update and remove

`update` refreshes tracking catalogs and reinstalls one or every installed
skill. Missing managed copies are repaired. Tags and commit refs remain pinned.

```bash
repertoire update
repertoire update code-reviewer
repertoire update --target all
repertoire remove code-reviewer
```

The update command also accepts `--target all` (or repeated individual
`--target` flags) to replace the stored target set while updating.

Updates and removals refuse locally modified targets. Review the changes before
using `--force` to replace or delete them.

All commands use project scope inside a Git worktree and global scope
elsewhere. Use `--project` or `--global` to make the choice explicit.

## Shell completion

Repertoire generates context-aware completion scripts for Bash, Zsh, Fish, and
PowerShell. Completions suggest installed skills, available skills from local or
already-cached catalogs, catalog names, and agent targets. Completion never
clones or refreshes a catalog.

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
