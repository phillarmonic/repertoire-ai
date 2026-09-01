# Contributing

Repertoire requires Go 1.27, Drun, `golangci-lint`, `gosec`, and the
Zensical development environment captured by `uv.lock`.

```bash
xdrun ci
uv run zensical build --clean --strict
```

Keep command behavior in `internal/cli`, state contracts in `internal/state`,
catalog transport in `internal/catalog`, and filesystem installation in
`internal/install`. Tests must isolate user configuration with temporary
directories and must not require live credentials or mutate the real user
profile.

When adding source files, update the explicit `.graphifyignore` whitelist and
run `graphify update .`.
