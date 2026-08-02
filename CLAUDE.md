# Claude Code project entrypoint

This repository uses a tool-neutral Agent contract. Before making changes:

1. Read [`AGENTS.md`](./AGENTS.md). It is the repository-wide human-readable contract.
2. Read `.mss/project.yaml`, `.mss/capabilities.yaml`, and `.mss/commands.yaml`. They are the machine-readable source of truth.
3. Read the closest directory-specific `AGENTS.md` for the files you will change.
4. Inspect existing capabilities before creating a parallel implementation.
5. Use repository-local workflows under `.agents/skills/` when one matches the task.
6. Prefer `go run ./cmd/mss ...` over workstation-specific command sequences.

## Canonical entry commands

```shell
go run ./cmd/mss context --format json
go run ./cmd/mss doctor --format json
go run ./cmd/mss skills list --format json
go run ./cmd/mss verify --changed
```

For a new management module, validate a structured `AdminModule` specification before generating code. For a new downstream application, run `mss new app` without `--write` first. For a foundation upgrade, run `mss upgrade plan` before `mss upgrade apply --yes`.

Do not duplicate the repository architecture, security rules, or test matrix in this file. When this file conflicts with `AGENTS.md` or `.mss/`, the source-of-truth order in `AGENTS.md` applies.
