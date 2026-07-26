# Releasing

Repertoire releases are built from semantic-version tags such as `v1.0.0`.
The release workflow runs on one Ubuntu runner and uses Go cross-compilation to
produce binaries for:

- Linux on amd64 and arm64;
- macOS on amd64 and arm64;
- Windows on amd64 and arm64.

Before creating a tag, run the project checks and a local release build:

```bash
xdrun ci
./scripts/build-release.sh v1.0.0
```

The build writes six binaries and `checksums.txt` to `dist/`. It runs the native
binary's version and help commands as a smoke test. Release and CI binaries
report the version, short commit, and a human-readable UTC build time:

```text
repertoire version v1.0.0 (f5b1463, 2026-07-26 06:53:13 UTC)
```

Push the tag after reviewing the generated assets:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The `Build and Release` workflow repeats the checks and build on a single
Ubuntu runner, then creates a GitHub release with generated release notes.
Tags containing a suffix such as `v1.0.0-rc.1` are published as prereleases.

Manual release workflow runs execute the same cross-platform build without
publishing a release. Their binaries are retained as workflow artifacts for
inspection.

Pull requests use the separate `Test Suite` workflow. It runs quality and
security checks on Linux, then tests and smoke-builds the native binary on
Linux, macOS, and Windows. Both workflows install Drun with
`phillarmonic/setup-drun@v2` and execute the repository's `.drun/spec.drun`
tasks, keeping local and remote verification aligned. Draft pull requests skip
every job, including when new commits are pushed. Marking a pull request as
draft also cancels an active run; newer commits cancel superseded runs for the
same pull request or branch.
