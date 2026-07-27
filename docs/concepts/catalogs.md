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
