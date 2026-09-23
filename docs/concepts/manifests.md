# Manifests and state

Repertoire uses one human-edited YAML format for catalogs, catalog
registrations, bootstrap skills, and declared requirements. A file may contain
any combination of the sections.

```yaml
schema: 1
tool: https://github.com/phillarmonic/repertoire-ai

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

`tool` is an informational marker for people who find the file outside its
original context. Repertoire writes it as the project repository URL, but it
does not affect schema validation or installation behavior.

Catalog names use lowercase letters, digits, and hyphens. Catalog skill names
use the same kebab-case rule. Prefer owner-prefixed names such as
`phillarmonkey-code` for broad skills that would otherwise be generic. Skill
paths must be relative and contained within the catalog repository. Avoid
publishing generic catalog skill names such as `code`, `docs`, or `review`;
when agents show several personal or vendor skills together, those labels are
easy to confuse.

## Trusted catalog keys

A catalog registration can list the public keys allowed to speak for that
catalog. Registrations with no `trust` block load and install as they do
today. When `trust` is present, each entry under `trust.keys` names an armored
public key file and the fingerprint that file must match:

```yaml
catalogs:
  company:
    source: git@github.com:example/company-skills.git
    ref: main
    trust:
      keys:
        - path: keys/company.asc
          fingerprint: ABCDEF0123456789ABCDEF0123456789ABCDEF01
```

`path` is relative to the directory that contains the manifest, and it must
stay inside that directory. `fingerprint` is the key the armored file must
match. An empty path, an empty fingerprint, or a path that leaves the manifest
directory is rejected. The manifest schema stays at `1`.

When `trust` is present, materializing the catalog checks the checked-out
commit with `git verify-commit`. Repertoire imports only those public keys
into a temporary keyring, confirms each file's fingerprint before import, and
deletes the keyring when the check finishes. It does not read or write your
personal GnuPG keyring, and it does not contact a keyserver. A registration
with no `trust` block skips the check. A local path or an override still runs
it: the commit that was actually checked out has to verify. If `gpg` is
missing, the commit is unsigned, the fingerprint does not match, or the
signature is not from one of the declared keys, materialize fails and the
error names the catalog, the skill, and the commit. `add`, `install`,
`update`, and `bootstrap` then leave the lock unchanged.

The same keys sign each skill. A trusted catalog skill carries
`REPERTOIRE.digest.asc` in the skill directory: a detached ASCII signature
over that skill's content digest. The digest is the same hex hash recorded in
the lock (paths, modes, symlink targets, and file bytes). The signature file
is not part of the hash, so adding it does not change the digest. Either
declared key may sign. Verification uses the same temporary keyring and the
same fingerprint check as the commit. A changed skill, a missing
`REPERTOIRE.digest.asc`, or a signature from any other key fails, and the
error names the catalog, the skill, and the commit. `add`, `install`,
`update`, and `bootstrap` leave the lock unchanged. A loose catalog skill uses the
same file in its skill directory. A registration with no `trust` block does
not require it.

## Project and global scope

Commands default to user-global scope: state lives in the operating system's
user configuration directory, and skills install under home-directory agent
roots. `--project` reads `repertoire.yaml` and `repertoire.lock.json` from the
current Git worktree root and installs into project-local agent directories.
`--global` makes the default explicit. The two flags cannot be combined.

The optional `hooks` field records whether optional managed project artifacts
should be installed with a declared requirement. Catalog-provided project
instructions are installed independently of this setting. The lock file is generated
deterministically. It records resolved commits, per-target content digests,
logical targets, installed locations, managed artifact destinations and
digests, and whether an installation came from a declared requirement or an
ad-hoc install.

## Project bootstrap skills

A Git project declares the skills a contributor needs in the top-level
`skills` section of its `repertoire.yaml`, next to any `catalogs` and
`requirements` sections:

```yaml
schema: 1
tool: https://github.com/phillarmonic/repertoire-ai

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

`skills` keys may be skill names or
source-qualified IDs (`{catalog-host-and-path}/{skill-name}`). Prefer
owner-prefixed kebab-case skill names for broad or generic skills, such as
`phillarmonkey-code`. A catalog skill with that name can be referenced from a
source-qualified project manifest key such as
`github.com/example/company-skills/phillarmonkey-code`. `catalog` and
`targets` are optional. An omitted catalog uses unqualified resolution for skill
names, or the source-qualified ID's catalog source. An omitted target uses
agent detection for that scope.
`scope` accepts `project` or `global` and defaults to `global`.
Catalog-declared project instructions are installed into the worktree for both
project- and global-scope bootstrap declarations. This supports a small
repository pointer to a globally installed skill without copying the complete
skill package into the repository. The per-project instruction state is stored
in Repertoire's global lock, so bootstrap does not create a project lock solely
for these pointers.

`hooks: true` additionally enables catalog-declared hooks and integrations.
For global-scope declarations, the skill stays in the user's home directory
while these optional artifacts are managed in the bootstrapping worktree.
Removing the globally managed skill also removes its recorded project
instructions and optional artifacts, subject to the normal local-modification
checks.

When `repertoire bootstrap` runs in a project whose `repertoire.yaml` declares
no skills, it adds a starter `skills` section that declares every built-in
`phillarmonic` skill with a source-qualified ID and `scope: global`.

Bootstrap installations are recorded with a `bootstrap` origin in the normal
project or global lock. They do not add requirements to
`repertoire.yaml`, and removing one does not edit the manifest.
Removing a declaration from the `skills` section is additive: existing installed
copies remain until explicitly removed.

### Migrating from `.repertoire.yaml`

Repertoire previously read bootstrap declarations from a standalone
`.repertoire.yaml` at the worktree root. When `repertoire bootstrap` or
`repertoire sync` finds that legacy file and `repertoire.yaml` declares no
skills, it merges the legacy `catalogs` and `skills` sections into
`repertoire.yaml` and deletes `.repertoire.yaml`. When both files declare
skills, `repertoire.yaml` wins and the legacy file is ignored with a warning so
you can merge and delete it manually.
