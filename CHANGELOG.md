# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Deprecated

### Removed

### Fixed

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
