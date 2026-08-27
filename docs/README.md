# mss-boot documentation

This source tree records **v1.3.2** as the current stable baseline and **v1.3.5**
as a permanently stopped immutable partial release. **v1.3.6 is the selected
release candidate but is not published or adoptable yet.** This tree documents
the source-only architecture and the v1.3.6 Complete Admin Distribution
candidate contract. Start at
[`docs/getting-started`](./docs/getting-started/index.md) for adoption status;
operational onboarding remains disabled until v1.3.6 has complete
public reconciliation.

## Information architecture

- `docs/getting-started/`: adoption status, package and tool availability, and mss-shop;
- `docs/guide/`: day-to-day operation and troubleshooting;
- `docs/admin/`: current Admin configuration, security, and operations;
- `docs/agent/`: specifications, generation, verification, and contributor workflows;
- `docs/architecture/`: current architecture summaries;
- `docs/releases/`: the v1.3.6 candidate, v1.3.5 immutable-partial record, and a clearly separated read-only archive;
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

The drift check verifies the v1.3.2 stable, v1.3.5 immutable-partial, and
unpublished v1.3.6 candidate boundaries; rejects prepublication v1.3.6 adopter
commands; and checks navigation targets, internal links, archive banners, and
ADR status markers.
