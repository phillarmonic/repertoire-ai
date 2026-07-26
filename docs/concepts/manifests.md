# Manifests and state

Repertoire uses one human-edited YAML format for catalogs, catalog
registrations, and declared requirements. A file may contain any combination of
the sections.

```yaml
schema: 1

catalog:
  name: phillarmonic
  description: Phillarmonic agent skills
  skills:
    zensical:
      path: skills/zensical

catalogs:
  company:
    source: git@github.com:example/company-skills.git
    ref: main

requirements:
  code-reviewer:
    catalog: company
    targets:
      - codex
```

Skill and catalog names use lowercase letters, digits, and hyphens. Skill paths
must be relative and contained within the catalog repository.

## Project and global scope

Inside a Git worktree, Repertoire reads `repertoire.yaml` and
`repertoire.lock.json` from the worktree root. Outside a worktree it uses the
operating system's user configuration directory. `--project` and `--global`
select a scope explicitly and cannot be combined.

The lock file is generated deterministically. It records resolved commits,
content digests, logical targets, installed locations, and whether an
installation came from a declared requirement or an ad-hoc install.

## Project bootstrap manifest

A Git project may also carry a standalone `.repertoire.yaml` at its worktree
root. This small onboarding manifest is separate from `repertoire.yaml`: it
declares which skills a contributor needs and whether each skill belongs in the
project or the contributor's home directory.

```yaml
schema: 1

catalogs:
  company:
    source: git@github.com:example/company-skills.git
    ref: main

skills:
  graphify:
    catalog: phillarmonic
    scope: project
    targets: [codex]

  code-reviewer:
    catalog: company
    scope: global
    targets: [codex, claude]
```

`skills` must contain at least one skill. `catalog` and `targets` are optional.
An omitted catalog uses normal unqualified resolution and must resolve
unambiguously. An omitted target uses agent detection for that scope. `scope`
accepts `project` or `global` and defaults to `project`.

Bootstrap installations are recorded with a `bootstrap` origin in the normal
project or global lock. They do not add requirements to either
`repertoire.yaml`, and removing one does not edit the bootstrap manifest.
Removing a declaration from `.repertoire.yaml` is additive: existing installed
copies remain until explicitly removed.
