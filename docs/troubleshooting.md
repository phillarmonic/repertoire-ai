# Troubleshooting

## No supported agent clients detected

Repertoire only auto-selects clients whose configuration directory already
exists. Pass a target explicitly, such as `--target codex`, `--target gemini`,
`--target windsurf`, or the portable `--target agents`. To create managed
copies for every supported client regardless of detection, use `--target all`.

## Skill name is ambiguous

Two or more visible catalogs declare the same name. Repertoire lists every
matching catalog and source in the error. Repeat the command with
`--catalog <name>`; Repertoire never chooses a source silently.

## Target is unmanaged or modified

Repertoire found content it cannot safely replace or remove. Review the target
directory first. Use `--force` only when discarding it is intentional.

## A private catalog cannot be cloned

Run `git ls-remote <source>` using the same URL. Repair SSH-agent or Git
credential-helper access, then run `repertoire catalog update`.

## Repair a missing installation

Run `repertoire install <name>` or `repertoire update <name>`. Missing managed
copies are recreated from the locked catalog source.
