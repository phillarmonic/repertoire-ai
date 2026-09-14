---
title: Private and company catalogs
description: Publish your own AI agent skills from a private Git repository and install them with Repertoire using your existing Git credentials.
---

# Private and company catalogs

The built-in catalog covers general-purpose skills. Skills that encode your
own conventions, internal tools, or review rules belong in a catalog you
control, and usually one that only your team can read.

A private catalog is an ordinary Git repository. Make it private on GitHub,
GitLab, Bitbucket, or any other host; Repertoire reads it with the same system
`git` credentials that already work for `git clone`. It has no authentication
system of its own and never stores tokens or passwords.

!!! tip "Let your agent build it"
    The built-in `repertoire` skill teaches your coding agent everything on
    this page. After `repertoire add repertoire`, you can ask the agent to
    "create a private skill catalog repository for our team" and it will
    scaffold the layout, `repertoire.yaml`, and `SKILL.md` files described
    below, then test them locally before pushing. See
    [Let your agent drive Repertoire](agent-skill.md).

## Build a private catalog

### 1. Create a Git repository

Create an empty private repository on your Git host (for example
`company/agent-skills`) and clone it:

```bash
git clone git@github.com:company/agent-skills.git
cd agent-skills
```

### 2. Lay out the catalog

Put each skill in its own directory containing a `SKILL.md`, and add a
`repertoire.yaml` at the root that lists them:

```text
agent-skills/
├── repertoire.yaml
└── skills/
    └── code-reviewer/
        └── SKILL.md
```

### 3. Declare the catalog in `repertoire.yaml`

```yaml
schema: 1
tool: https://github.com/phillarmonic/repertoire-ai

catalog:
  name: company
  description: Company-owned AI agent skills
  skills:
    code-reviewer:
      path: skills/code-reviewer
    shared-helpers:
      path: skills/shared-helpers
```

Rules:

- `schema` must be `1`. `tool` is optional and informational; pointing it at
  the Repertoire repository tells readers which program owns the file.
- Catalog and skill names are 1 to 64 lowercase letters, digits, or single
  hyphens (`company-skills` is valid; `Company_Skills` is not). The skill
  directory name must match the skill name exactly.
- Avoid generic skill names such as `code`, `docs`, or `review`. Agents often
  show only the short identifier, and once several catalogs are enabled those
  labels collide. Prefer an owner-prefixed name such as `phillarmonkey-code`.
- The catalog must declare at least one skill, and each `path` must be a
  relative path inside the repository (no absolute paths, no `..`).

### 4. Author each skill's `SKILL.md`

Every skill directory needs a `SKILL.md` with YAML frontmatter. `name` must
match both the key in `repertoire.yaml` and the directory name; `description`
is required and must not be empty:

```markdown
---
name: code-reviewer
description: Review pull requests against the company style guide.
---

# Code reviewer

Instructions for the agent go here.
```

You may include supporting files next to `SKILL.md` (scripts, templates,
references). Repertoire copies the directory as data and never executes skill
scripts during install. Symlinks are allowed only when they resolve inside the
skill directory.

??? note "Offering starter files with stubs.yaml"
    A skill can offer small starter files that agents fetch with
    `repertoire stub get`. Add a `stubs.yaml` beside `SKILL.md`:

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

    Stub names follow the same lowercase-and-hyphen rule as skill names. Every
    entry needs a non-empty description and instructions, and its relative
    path must resolve to one regular file inside the skill directory. Invalid
    manifests, missing files, directories, and escaping symlinks stop the
    skill from installing. See [`stub`](commands.md#stub-starter-files-from-installed-skills)
    for how agents consume them.

### 5. Commit and push

```bash
git add repertoire.yaml skills
git commit -m "Add company skill catalog"
git push -u origin main
```

Keep the repository private on your Git host so only authorized accounts can
clone it.

## Check Git access

Before registering the catalog, confirm the remote is readable with your
normal Git setup:

```bash
git ls-remote git@github.com:company/agent-skills.git
# or
git ls-remote https://github.com/company/agent-skills.git
```

| Transport | How credentials are supplied |
|-----------|------------------------------|
| SSH (`git@…`) | Active SSH agent and `~/.ssh/config` host keys |
| HTTPS (`https://…`) | Git credential helpers, or provider CLIs such as `gh auth setup-git` |

If `git ls-remote` fails, Repertoire will fail too. Fix the SSH agent or
credential helper first.

Never put usernames, passwords, or tokens in the URL. Repertoire rejects such
URLs so that secrets stay out of manifests, lock files, command output, and
error reports.

??? note "Why a missing credential fails immediately"
    Repertoire runs Git with terminal prompts disabled, so a missing
    credential surfaces as an immediate `could not read Username` error
    instead of a hidden password prompt. Catalog operations are also pinned to
    HTTP/1.1, because some networks answer GitHub's HTTP/2 POSTs with spurious
    401 responses that Git misreports as an authentication failure.

## Register and use the catalog

```bash
repertoire catalog add git@github.com:company/agent-skills.git --name company
# or
repertoire catalog add https://github.com/company/agent-skills.git --name company
```

Add `--ref` to pin a branch, tag, or commit; without it the remote default
branch is tracked. Then browse and install:

```bash
repertoire list --available --catalog company
repertoire add code-reviewer --catalog company --target all
```

Pick up new commits with:

```bash
repertoire update
```

## Share the catalog with your team

Declare the catalog and the skills in the project's `repertoire.yaml` so every
contributor and CI job installs the same set with `repertoire bootstrap`. The
`catalogs` section takes the same `source` and `ref` you passed to
`catalog add`. See [Set up a project or team](automation.md) for the full
manifest and workflow. Each developer still needs Git credentials that can
read the catalog repository.

## Test a catalog before pushing

While developing skills, point a catalog at your local checkout so you do not
have to push to try a change:

```bash
repertoire catalog add /path/to/agent-skills --name company
# or, without changing the registration:
repertoire --override company=/path/to/agent-skills add code-reviewer --catalog company
```

See [Local overrides for testing](concepts/catalogs.md#local-overrides-for-testing).
