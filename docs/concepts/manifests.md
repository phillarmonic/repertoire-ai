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
    hooks: true
```

Catalog names use lowercase letters, digits, and hyphens. Catalog skill names
use the same kebab-case rule. Prefer owner-prefixed names such as
`phillarmonkey-code` for broad skills that would otherwise be generic. Skill
paths must be relative and contained within the catalog repository. Avoid
publishing generic catalog skill names such as `code`, `docs`, or `review`;
when agents show several personal or vendor skills together, those labels are
easy to confuse.

## Project and global scope

Commands default to user-global scope: state lives in the operating system's
user configuration directory, and skills install under home-directory agent
roots. `--project` reads `repertoire.yaml` and `repertoire.lock.json` from the
current Git worktree root and installs into project-local agent directories.
`--global` makes the default explicit. The two flags cannot be combined.

The optional `hooks` field records whether managed project artifacts should be
installed with a declared requirement. The lock file is generated
deterministically. It records resolved commits, per-target content digests,
logical targets, installed locations, managed artifact destinations and
digests, and whether an installation came from a declared requirement or an
ad-hoc install.

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
    hooks: true
```

`skills` must contain at least one skill. Keys may be skill names or
source-qualified IDs (`{catalog-host-and-path}/{skill-name}`). Prefer
owner-prefixed kebab-case skill names for broad or generic skills, such as
`phillarmonkey-code`. A catalog skill with that name can be referenced from a
source-qualified project manifest key such as
`github.com/example/company-skills/phillarmonkey-code`. `catalog` and
`targets` are optional. An omitted catalog uses unqualified resolution for skill
names, or the source-qualified ID's catalog source. An omitted target uses
agent detection for that scope.
`scope` accepts `project` or `global` and defaults to `global`.
`hooks: true` enables catalog-declared project artifacts. It has no effect for
global-scope skills because project artifact destinations are worktree-relative.

When `repertoire bootstrap` runs without `.repertoire.yaml`, it writes a starter
manifest that declares every built-in `phillarmonic` skill with a
source-qualified ID and `scope: global`.

Bootstrap installations are recorded with a `bootstrap` origin in the normal
project or global lock. They do not add requirements to either
`repertoire.yaml`, and removing one does not edit the bootstrap manifest.
Removing a declaration from `.repertoire.yaml` is additive: existing installed
copies remain until explicitly removed.
