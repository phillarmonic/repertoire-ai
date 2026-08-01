# Troubleshooting

## No supported agent clients detected

Repertoire only auto-selects clients whose configuration directory already
exists. Pass a target explicitly, such as `--target codex`, `--target gemini`,
`--target windsurf`, or the portable `--target agents`. To create managed
copies for every supported client regardless of detection, use `--target all`.

## Skill name is ambiguous

Two or more non-mainline catalogs declare the same name and the official
`phillarmonic` catalog does not define it. Repertoire lists every matching
catalog and source in the error. Repeat the command with `--catalog <name>` or
use a source-qualified ID. When `phillarmonic` does define an unqualified name,
Repertoire deliberately chooses that official definition.

## Target is unmanaged or modified

Repertoire found content it cannot safely replace or remove. Review the target
directory first. Use `--force` only when discarding it is intentional.

## A private catalog cannot be cloned

Run `git ls-remote <source>` using the same URL. Repair SSH-agent or Git
credential-helper access, then run `repertoire catalog update`.

## Repair a missing installation

Run `repertoire install <name>` or `repertoire update <name>`. Missing managed
copies are recreated from the locked catalog source.

## Managed files are missing, modified, or duplicated

Run `repertoire doctor`. It reports missing or locally modified managed
files, managed sections that no lock entry claims, identical sections
duplicated under per-target markers, declarations whose lock state has
drifted, and global-lock entries for projects that no longer exist, each with
a suggested remedy. `repertoire doctor --fix` repairs them, and
`repertoire doctor --reset --yes` reinstalls every managed artifact for the
project from scratch.
