# Targets and security

Repertoire copies a validated skill directory into one or more agent-specific skill roots. By default those roots are
under the home directory (Global root). Use `--project` for the Project root column. With no `--target`, it detects
existing client configuration or skill directories. An explicit target creates its skills directory when needed. The
special `--target all` value expands to every target in the table, including clients that are not currently detected.

| Target                | Project root                                   | Global root                                          |
|-----------------------|------------------------------------------------|------------------------------------------------------|
| `agents`              | `.agents/skills`                               | `~/.agents/skills`                                   |
| `aider`               | `.aider`                                       | `~/.aider`                                           |
| `amp`                 | `.agents/skills`                               | `~/.config/agents/skills`                            |
| `antigravity`         | `.agents/skills`                               | `~/.gemini/config/skills`                            |
| `antigravity-windows` | `.agents/skills`                               | `~/.gemini/config/skills`                            |
| `claude`              | `.claude/skills`                               | `~/.claude/skills`                                   |
| `claw`                | `.openclaw/skills`                             | `~/.openclaw/skills`                                 |
| `cline`               | `.cline/skills`                                | `~/.cline/skills`                                    |
| `codebuddy`           | `.codebuddy/skills`                            | `~/.codebuddy/skills`                                |
| `codex`               | `.codex/skills`                                | `$CODEX_HOME/skills` or `~/.codex/skills`            |
| `copilot`             | `.github/skills` or existing `.copilot/skills` | `~/.copilot/skills`                                  |
| `cursor`              | `.cursor/skills`                               | `~/.cursor/skills`                                   |
| `devin`               | `.devin/skills`                                | `~/.config/devin/skills`                             |
| `droid`               | `.factory/skills`                              | `~/.factory/skills`                                  |
| `gemini`              | `.gemini/skills`                               | `~/.gemini/skills`                                   |
| `hermes`              | `.hermes/skills`                               | `~/.hermes/skills`                                   |
| `junie`               | `.junie/skills`                                | `~/.junie/skills`                                    |
| `kilo`                | `.config/kilo/skills`                          | `~/.config/kilo/skills`                              |
| `kimi`                | `.kimi-code/skills`                            | `$KIMI_CODE_HOME/skills` or `~/.kimi-code/skills`    |
| `kiro`                | `.kiro/skills`                                 | `~/.kiro/skills`                                     |
| `opencode`            | `.opencode/skills`                             | `~/.config/opencode/skills`                          |
| `openclaw`            | `skills`                                       | `$OPENCLAW_STATE_DIR/skills` or `~/.openclaw/skills` |
| `pi`                  | `.pi/agent/skills`                             | `~/.pi/agent/skills`                                 |
| `roo`                 | `.roo/skills`                                  | `~/.roo/skills`                                      |
| `trae`                | `.trae/skills`                                 | `~/.trae/skills`                                     |
| `trae-cn`             | `.trae-cn/skills`                              | `~/.trae-cn/skills`                                  |
| `vscode`              | `.github/skills`                               | `~/.copilot/skills`                                  |
| `windows`             | `.claude/skills`                               | `~/.claude/skills`                                   |
| `windsurf`            | `.windsurf/skills`                             | `~/.codeium/windsurf/skills`                         |

The `all` value is accepted by `add`, `install`, and `update`. For bulk operations, these commands apply the override to
every selected skill:

```bash
repertoire install --target all
repertoire update --target all
```

The expanded concrete target names—not `all`—are saved in the manifest and lock, keeping installation state explicit.

## Installation safety

- `SKILL.md` must have valid YAML frontmatter with a matching `name` and a non-empty `description`.
- Optional `stubs.yaml` entries must have valid names, descriptions, instructions, and contained paths resolving to
  regular files.
- Catalog paths must stay inside the repository. Symlinks are allowed only when they resolve inside the skill directory.
- Repertoire hashes paths, modes, symlink targets, and file content.
- Installation is staged beside the destination and then renamed atomically.
- An unmanaged or locally modified destination is preserved. Replacement requires an explicit `--force`.
- Skill scripts are copied as data and are never executed during installation.
- Catalogs may declare always-on project instructions and optional managed artifacts. Repertoire supports whole-file
  copies, marked Markdown sections, and additive JSON-object merges.
- Project instructions install whenever their target is installed in project scope. Bootstrap also installs them into
  the worktree for global-scope skills while keeping the full skill under the user's home directory.
- Interactive `add` prompts before installing optional hooks and integrations; noninteractive commands require
  `--with-hooks`.
- `--no-hooks` removes previously managed optional artifacts while retaining project instructions. Unrelated Markdown,
  JSON entries, and locally modified files
  are preserved or rejected unless
  `--force` is explicit.
