# Catalogs

A catalog is a Git repository with a `repertoire.yaml` catalog section. The
built-in `phillarmonic` registration points to
`https://github.com/phillarmonic/ai-skills.git`.

The built-in catalog does not need to be declared separately. Because
`phillarmonic` is the official mainline catalog, these commands select the same
`zensical` definition even when another visible catalog uses that short name:

```bash
repertoire add zensical
repertoire add zensical --catalog phillarmonic
repertoire add github.com/phillarmonic/ai-skills/zensical
```

Source-qualified IDs are `{catalog-host-and-path}/{skill-name}` and always select
one catalog source. An unqualified name prefers `phillarmonic` when the
mainline catalog defines it. If several non-mainline catalogs define the same
short name, Repertoire lists every match instead of choosing one. Repeat with
`--catalog <name>` or a source-qualified ID.

Use owner-prefixed kebab-case skill names whenever a short name is generic. A
skill named `code` may look fine inside one personal catalog, but agents and
UIs can become ambiguous once several catalogs are enabled. Prefer
`phillarmonkey-code` as the skill package name, and use a source-qualified ID
such as `github.com/company/agent-skills/phillarmonkey-code` when the command
or manifest also needs to pin the catalog source.

```bash
repertoire catalog list
repertoire catalog add /path/to/ai-skills
repertoire catalog add git@github.com:example/private-skills.git --name company
repertoire catalog add github.com/example/public-skills --name public --ref main
repertoire catalog update
```

Local paths are read directly. Remote catalogs are cloned into the operating
system's user cache and refreshed with system Git. HTTPS credential helpers,
SSH agents, and provider CLI integrations therefore work without Repertoire
storing credentials.

## Local overrides for testing

While developing a catalog, redirect any catalog to a local checkout so you do
not have to push before testing. Set the `REPERTOIRE_OVERRIDES` environment
variable to a comma-separated list of `name=path` or `source=path` pairs, or
pass a repeatable `--override name=path` flag:

```bash
REPERTOIRE_OVERRIDES="phillarmonic=/path/to/ai-skills" repertoire add zensical --target codex
repertoire --override company=/path/to/company-skills add code-reviewer --catalog company
repertoire catalog list   # marks overridden sources
```

An override matches a catalog by its registered name or by its normalized
source URL, and flag values win over environment values. The local path is read
directly instead of the remote, so `add`, `install`, `update`, `bootstrap`,
`sync`, `list --available`, and shell completion all resolve from the local
checkout. Remove the override to return to the registered remote source.

An omitted ref tracks the remote default branch. Named branches advance during
updates; tags and full commit hashes remain immutable. Registering an explicit
catalog named `phillarmonic` overrides the built-in source, which is useful for
local development.

## Platform variants and project artifacts

A catalog can keep one logical skill name while selecting a different package
for individual targets. The default `path` remains required and is used when a
target has no override:

```yaml
schema: 1
tool: https://github.com/phillarmonic/repertoire-ai
catalog:
  name: graphify
  skills:
    graphify:
      path: skills/graphify
      variants:
        codex: platforms/codex
        claude: platforms/claude
      instructions:
        codex:
          - id: guidance
            source: pointers/agents.md
            destination: AGENTS.md
            mode: markdown-section
      artifacts:
        codex:
          - id: hooks
            source: project-files/codex-hooks.json
            destination: .codex/hooks.json
            mode: json-merge
```

`instructions` are always-on, lightweight project pointers or rules.
`artifacts` are optional hooks, plugins, and integration configuration.
Project installs always apply matching instructions; optional artifacts retain
the interactive prompt and `--with-hooks`/`--no-hooks` controls.

Variant directories may have any directory name, but their `SKILL.md`
frontmatter name must match the logical catalog skill. Variant and artifact
source paths must remain inside the catalog. Artifact destinations must be
contained relative project paths.

Artifact modes are:

- `copy` manages a whole file. Set `executable: true` for a copied hook script.
  Because copy owns the whole file, two skills copying different content to
  the same destination conflict; use `markdown-section` for shared
  instruction files.
- `markdown-section` inserts a marked section and preserves text outside it.
- `json-merge` adds object keys and array entries while preserving unrelated
  configuration.

The special target `all` applies to every selected target in either section.

Identical `markdown-section` entries repeated across selected targets — the
same id, destination, and source content — are installed once under the `all`
marker rather than inlined once per target. This keeps shared instruction
files such as `AGENTS.md` small when many AGENTS.md-reading agents are
selected. Entries whose source content differs keep separate per-target
sections. Existing per-target sections migrate to the shared
`repertoire:<skill>:all:<id>` marker on the next install or sync.
Repertoire copies artifact data and updates configuration; it does not execute
hook scripts during installation.

Command-bearing project artifacts should invoke installed tools by binary name
and rely on `PATH`, for example `graphify hook-check`. Do not package an
installer-resolved path such as `/Users/name/.local/bin/graphify`; it will not
be portable to another contributor or machine.
