# Catalogs

A catalog is a Git repository with a `repertoire.yaml` catalog section. The
built-in `phillarmonic` registration points to
`https://github.com/phillarmonic/ai-skills.git`.

The built-in catalog does not need to be declared separately. When `zensical`
has a single visible definition, these commands are equivalent:

```bash
repertoire add zensical
repertoire add zensical --catalog phillarmonic
repertoire add github.com/phillarmonic/ai-skills/zensical
```

Source-qualified IDs are `{catalog-host-and-path}/{skill-name}` and always select one
catalog source. When several visible catalogs define the same short skill name,
Repertoire lists every matching catalog (with source-qualified IDs) instead of
choosing one. Repeat with `--catalog <name>` or a source-qualified ID.

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
catalog:
  name: graphify
  skills:
    graphify:
      path: skills/graphify
      variants:
        codex: platforms/codex
        claude: platforms/claude
      artifacts:
        codex:
          - id: guidance
            source: project-files/agents.md
            destination: AGENTS.md
            mode: markdown-section
          - id: hooks
            source: project-files/codex-hooks.json
            destination: .codex/hooks.json
            mode: json-merge
```

Variant directories may have any directory name, but their `SKILL.md`
frontmatter name must match the logical catalog skill. Variant and artifact
source paths must remain inside the catalog. Artifact destinations must be
contained relative project paths.

Artifact modes are:

- `copy` manages a whole file. Set `executable: true` for a copied hook script.
- `markdown-section` inserts a marked section and preserves text outside it.
- `json-merge` adds object keys and array entries while preserving unrelated
  configuration.

The special artifact target `all` applies to every selected target. Repertoire
copies artifact data and updates configuration; it does not execute hook
scripts during installation.
