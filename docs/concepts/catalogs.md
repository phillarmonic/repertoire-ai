# Catalogs

A catalog is a Git repository with a `repertoire.yaml` catalog section. The
built-in `phillarmonic` registration points to
`https://github.com/phillarmonic/ai-skills.git`.

The built-in catalog does not need to be declared separately. When `zensical`
has a single visible definition, these commands are equivalent:

```bash
repertoire add zensical
repertoire add zensical --catalog phillarmonic
```

When several visible catalogs define the same skill name, Repertoire lists
every matching catalog and source instead of choosing one. Repeat the command
with `--catalog <name>` to select the intended definition.

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
