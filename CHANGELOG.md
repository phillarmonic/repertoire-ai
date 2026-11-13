# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Fixed

### Security

## [1.12.1] - 2026-11-13

### Added

### Fixed

- `repertoire bootstrap` and `sync` install the `requirements` section of a
  project manifest. A project set up with `repertoire --project add` writes its
  skills there, and a fresh clone used to see an empty manifest: it replaced
  the file with the starter list and then failed on target detection. The same
  command now restores a missing copy as well.

- `repertoire doctor` audits project-scope installs. A checkout whose agent
  directories were never restored reported `no issues found` while `show`
  listed every target as missing. The new `project-skill-health` check reports
  it, and `doctor --fix` reinstalls the missing copy.

- The error `add` prints when no agent client is detected names a usable value.
  It said `use --target`, which leaves the reader to guess the names; it now
  says `use --target <name> or --target all`, and the quickstart shows the
  `--project` form that writes a lock you can commit.

### Security

## [1.12.0] - 2026-11-12

### Added

- Slash-only skills are installed as commands for the `opencode` target. A skill
  whose frontmatter declares `disable-model-invocation: true` is meant to be
  invoked explicitly, but OpenCode offers every skill it finds to the model —
  its user-invoked entries live in a `command` directory instead. Such a skill
  is now written there too (`~/.config/opencode/command/<name>.md`, or
  `.opencode/command/` in a project), carrying the skill's description and body.
  When the skill declares `argument-hint`, the command template names
  `$ARGUMENTS`, because OpenCode substitutes the typed text only where the
  template asks for it. The skill directory is still installed, so companion
  files stay reachable. Removal, `--dry-run`, `show` and `doctor --fix` all
  account for the command file.

- Commands that take a skill or catalog name accept a comma-separated list as
  well as several arguments. `install`, `update`, `remove`, `show`,
  `stub list`, `stub get`, `catalog update`, and `catalog remove` now treat
  `repertoire catalog update phillarmonic,andy` as two catalogs instead of one
  name containing a comma. `add` already behaved this way, and shell
  completion keeps working after the comma.

### Fixed

- Reading a skill's invocation fields no longer rejects spells the convention
  allows. `argument-hint: [target]` is a YAML sequence unless the author quotes
  it, and the flag may be written as a quoted string; a strict field turned
  either into an invalid-frontmatter error, which fails the skill outright.

- The `opencode` target follows `XDG_CONFIG_HOME`. OpenCode resolves its
  configuration directory from that variable rather than hardcoding
  `~/.config`, so a global install on a machine that sets it wrote skills to
  `~/.config/opencode/skills` — a directory OpenCode never reads. The global
  root is now `$XDG_CONFIG_HOME/opencode/skills` when the variable is set and
  `~/.config/opencode/skills` otherwise; the project root is unchanged.
  `devin`, `kilo` and `amp` keep their fixed `~/.config` paths, because their
  clients' handling has not been verified and moving the path for a client that
  does hardcode `~/.config` would break it in the same way.

### Security

## [1.11.0] - 2026-09-25

### Added

- Trust blocks on catalogs. A registration may name armored public key files
  with the fingerprint each file must match; materializing the catalog then
  verifies the checked-out commit with `git verify-commit` through a temporary
  keyring, so the personal GnuPG keyring is neither read nor written and no
  keyserver is contacted. A registration with no `trust` block loads and
  installs exactly as before, and the manifest schema stays at `1`.

- A detached signature per skill. A trusted catalog skill may carry
  `REPERTOIRE.digest.asc`, a signature over the same content digest the lock
  records, and either declared key may sign it. The signature file is not part
  of the hash, so adding it does not change the digest. A changed skill, a
  missing signature, a fingerprint that does not match, a commit signed by none
  of the declared keys, or a missing `gpg` fails materialize, names the
  catalog, the skill and the commit, and leaves the lock unchanged.

- `docs/concepts/manifests.md` and `docs/concepts/targets-security.md`, which
  document the trust fields and what a trust block does and does not protect.

## [1.10.0] - 2026-09-16

### Added

- Loose catalogs: a Git repository of `SKILL.md` trees can be registered and
  used even when it has no `repertoire.yaml`.
- One-shot `add` from a catalog source (Git URL, `owner/repo`, or local path)
  registers the catalog and installs its skills in a single command.
- `repertoire show <skill>` prints provenance, catalog cache status, and
  per-target copy integrity.
- `repertoire init` writes a starter project `repertoire.yaml` without
  installing skills.
- `repertoire catalog init` scaffolds a catalog repository (optional
  `--skill` placeholders).
- Persistent `--dry-run` prints planned writes and refusals without changing
  disk, lock, or manifest.
- Shell completion covers `show`, `init`, `catalog init`, and `add --skill`.

### Changed

- Updated README and zensical docs

### Deprecated

### Removed

### Fixed

### Security

## [1.9.0] - 2026-09-15

### Added
Changelog
Visual identity

### Changed
Documentation is now clearer
### Deprecated

### Removed

### Fixed
Edge cases on repo checks

### Security
