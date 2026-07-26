# Private repositories

Repertoire delegates repository access to the system `git` executable. It does
not implement an authentication protocol or store credentials.

Use any Git URL that already works in your shell:

```bash
repertoire catalog add git@github.com:company/agent-skills.git --name company
repertoire catalog add https://github.com/company/agent-skills.git --name company
```

SSH sources use the active SSH agent and host configuration. HTTPS sources use
Git credential helpers or provider CLI integrations such as `gh auth setup-git`.
Test access with `git ls-remote <source>` before registering a catalog.

URLs containing embedded usernames, passwords, or tokens are rejected. This
keeps secrets out of `repertoire.yaml`, lock files, command output, and error
snapshots.
