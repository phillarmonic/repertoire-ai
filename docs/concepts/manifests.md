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

Catalog names use lowercase letters, digits, and hyphens. Catalog skill names
use the same rule, and may include `/` to qualify broad names with an owner or
vendor segment, such as `a-vendor-name/code`. Skill paths must be relative and
contained within the catalog repository. Avoid publishing generic catalog skill
names such as `code`, `docs`, or `review`; when agents show several personal or
vendor skills together, those labels are easy to confuse.

## Project and global scope

Commands default to user-global scope: state lives in the operating system's
user configuration directory, and skills install under home-directory agent
roots. `--project` reads `repertoire.yaml` and `repertoire.lock.json` from the
current Git worktree root and installs into project-local agent directories.
`--global` makes the default explicit. The two flags cannot be combined.

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
  github.com/phillarmonic/ai-skills/zensical:
    scope: global
    targets: [codex]

  shared-helpers:
    catalog: company
    scope: project
    targets: [agents]
```

`skills` must contain at least one skill. Keys may be short names or namespaced
IDs (`{catalog-host-and-path}/{skill-name}`). Use the namespaced form for broad
or generic skills so the manifest records who owns the definition. A catalog
skill named `a-vendor-name/code` can be referenced from a source-qualified
project manifest key such as
`github.com/example/company-skills/a-vendor-name/code`. On-disk install
directories still use a safe flat form of the skill name. `catalog` and
`targets` are optional. An omitted catalog uses unqualified resolution for short
names, or the namespace's catalog source for namespaced IDs. An omitted target
uses agent detection for that scope.
`scope` accepts `project` or `global` and defaults to `global`.

When `repertoire bootstrap` runs without `.repertoire.yaml`, it writes a starter
manifest that declares every built-in `phillarmonic` skill with a namespaced ID
and `scope: global`.

Bootstrap installations are recorded with a `bootstrap` origin in the normal
project or global lock. They do not add requirements to either
`repertoire.yaml`, and removing one does not edit the bootstrap manifest.
Removing a declaration from `.repertoire.yaml` is additive: existing installed
copies remain until explicitly removed.
