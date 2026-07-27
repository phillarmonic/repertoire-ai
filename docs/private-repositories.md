# Private repositories

A private Repertoire catalog is a normal Git repository that holds your
company-owned AI skills. Make the repository private on GitHub, GitLab,
Bitbucket, or any other Git host; Repertoire reads it with the same system `git`
credentials that already work for `git clone` and `git ls-remote`. Repertoire
does not implement its own authentication protocol and never stores tokens or
passwords.

## Build a private catalog

### 1. Create a Git repository

Create an empty private repository on your Git host (for example
`company/agent-skills`). Clone it locally:

```bash
git clone git@github.com:company/agent-skills.git
cd agent-skills
```

### 2. Lay out the catalog

At the repository root, add a `repertoire.yaml` that declares the catalog and
every skill. Put each skill in its own directory that contains a `SKILL.md`
file. A minimal layout:

```text
agent-skills/
├── repertoire.yaml
└── skills/
    └── code-reviewer/
        └── SKILL.md
```

Skill names should always be kebab-case. The directory name must match the
skill name exactly. The path in `repertoire.yaml` is relative to the repository
root and must stay inside the repo.

### 3. Declare the catalog in `repertoire.yaml`

```yaml
schema: 1

catalog:
  name: company
  description: Company-owned AI agent skills
  skills:
    code-reviewer:
      path: skills/code-reviewer
```

Rules:

- `schema` must be `1`.
- Catalog names use 1–64 lowercase letters, digits, or single hyphens
  (`company-skills` is valid; `Company_Skills` is not).
- Skill names use the same lowercase-and-hyphen rule. Prefer owner-prefixed
  kebab-case for broad or generic skills (`code-reviewer` and
  `phillarmonkey-code` are valid; `Code_Reviewer` is not).
- Avoid generic skill names such as `code`, `docs`, or `review` in published
  catalogs. Agents often display only the skill title or short identifier, so
  generic names become confusing when several personal or vendor catalogs are
  enabled. Prefer a clear owner-prefixed kebab-case identifier such as
  `phillarmonkey-code`.
- The catalog must declare at least one skill.
- Each skill `path` must be a contained relative path (no absolute paths, no
  `..` escape).

Add more skills by creating another directory and listing it under `skills`:

```yaml
schema: 1

catalog:
  name: company
  description: Company-owned AI agent skills
  skills:
    code-reviewer:
      path: skills/code-reviewer
    shared-helpers:
      path: skills/shared-helpers
```

### 4. Author each skill's `SKILL.md`

Every skill directory must contain a `SKILL.md` with YAML frontmatter. The
`name` must match the key in `repertoire.yaml`, and the directory name must
match too. `description` is required and must be non-empty:

```markdown
---
name: code-reviewer
description: Review pull requests against the company style guide.
---

# Code reviewer

Instructions for the agent go here.
```

Keep the `name` specific when the skill covers a broad domain. For example,
use `phillarmonkey-code` instead of `code`, and reserve the description for the
human-readable explanation. Project `.repertoire.yaml` files may still refer to
the skill with a full source-qualified ID such as
`github.com/company/agent-skills/phillarmonkey-code` when that makes the source
clearer.

You may include supporting files next to `SKILL.md` (scripts, templates,
references). Repertoire copies the skill directory as data; it never executes
skill scripts during install. Symlinks are allowed only when they resolve inside
the skill directory.

### Expose reusable file stubs

Add an optional `stubs.yaml` beside `SKILL.md` when a skill should offer small
starter files to agents:

```yaml
schema: 1
stubs:
  editorconfig:
    description: Ensure text files end with a newline.
    path: assets/.editorconfig
    instructions: |
      Create or merge the repository-root .editorconfig while preserving
      existing settings.
```

Stub names use the same lowercase letters, digits, and single-hyphen rules as
skill names. Every entry needs a non-empty description and instructions, and
its contained relative path must resolve to one regular file inside the skill
directory. Invalid manifests, missing files, directories, and escaping
symlinks prevent the skill from being installed.

After installing the skill, use `repertoire stub list [skill]` to discover its
stubs and `repertoire stub get <skill>/<stub>` to give an agent the verified
asset path and instructions.

### 5. Commit and push

```bash
git add repertoire.yaml skills
git commit -m "Add company skill catalog"
git push -u origin main
```

Keep the repository **private** on your Git host so only authorized accounts can
clone it.

## Authenticate with Git

Before registering the catalog, confirm the remote is readable with your normal
Git setup:

```bash
git ls-remote git@github.com:company/agent-skills.git
# or
git ls-remote https://github.com/company/agent-skills.git
```

| Transport | How credentials are supplied |
|-----------|------------------------------|
| SSH (`git@…`) | Active SSH agent and `~/.ssh/config` host keys |
| HTTPS (`https://…`) | Git credential helpers, or provider CLIs such as `gh auth setup-git` |

Without credentials that can read the remote, Repertoire cannot materialize the
private catalog.

URLs that embed usernames, passwords, or tokens are rejected. Put credentials in
your Git/SSH configuration—not in `repertoire.yaml`, lock files, or command
arguments. That keeps secrets out of manifests, command output, and error
snapshots.

## Register and use the catalog

Once `git ls-remote` succeeds, register the catalog:

```bash
repertoire catalog add git@github.com:company/agent-skills.git --name company
# or
repertoire catalog add https://github.com/company/agent-skills.git --name company
```

Optional: pin a branch, tag, or commit with `--ref`. An omitted ref tracks the
remote default branch.

Then install skills from it:

```bash
repertoire list --available --catalog company
repertoire add code-reviewer --catalog company --target all
```

Refresh after publishers push new commits:

```bash
repertoire catalog update
repertoire update
```

## Share the catalog with your team

Commit a project `.repertoire.yaml` so contributors install the same private
skills after they can authenticate to the Git remote:

```yaml
schema: 1

catalogs:
  company:
    source: git@github.com:company/agent-skills.git
    ref: main

skills:
  code-reviewer:
    catalog: company
    scope: global
    targets: [codex, claude, cursor]
```

Each developer (or CI job) needs Git credentials that can read
`company/agent-skills`. Then:

```bash
repertoire bootstrap
```

`bootstrap` resolves from current catalog state. To fetch catalog updates before
synchronizing declarations:

```bash
repertoire sync
```

For local development against an unpublished checkout, register a path instead
of a remote URL:

```bash
repertoire catalog add /path/to/agent-skills --name company
```
