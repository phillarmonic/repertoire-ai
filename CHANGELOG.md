# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

- README and Zensical docs position Repertoire as a package manager for agent
  skills (named catalogs, a lock with content digests, a committed manifest)
  rather than a one-off copier.

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
