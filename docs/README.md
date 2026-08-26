# mss-boot documentation

This source tree documents the package-first **v1.3.5** Complete Admin
Distribution candidate. It becomes public adopter guidance only after the
coordinated artifacts and deployed Docs site reconcile to one release commit.
Start at [`docs/getting-started`](./docs/getting-started/index.md); there is no
second quick start.

## Information architecture

- `docs/getting-started/`: installation, packages, tools, and mss-shop;
- `docs/guide/`: day-to-day operation and troubleshooting;
- `docs/admin/`: current Admin configuration, security, and operations;
- `docs/agent/`: specifications, generation, verification, and contributor workflows;
- `docs/architecture/`: current architecture summaries;
- `docs/releases/`: v1.3.5 and a clearly separated read-only archive;
- `../adr/`: durable decisions and their status.

Prompt dumps, one-off plans, test snapshots, and duplicate tutorials are not
documentation inputs.

## Documentation contributor workflow

This section is explicitly for contributors working in a Foundation source
checkout:

```sh
corepack pnpm@9.15.9 --dir docs install --frozen-lockfile
python3 tools/docs/check_current_docs.py
corepack pnpm@9.15.9 --dir docs build
```

The drift check verifies the active v1.3.5 version, package-first commands,
single quick-start route, navigation targets, internal links, archive banners,
and ADR status markers.
