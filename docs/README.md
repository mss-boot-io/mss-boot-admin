# mss-boot documentation

This source tree records **v1.3.2** as its current stable baseline and **v1.3.5**
plus **v1.3.6** as permanently stopped immutable partial releases. **v1.3.7 is
the selected release candidate and is not stable or adoptable.** Candidate
surfaces can become public in stages, so the remote ledger is authoritative for
publication status until stable promotion and final policy/Docs reconciliation
complete. This tree documents the source-only architecture and the v1.3.7
Complete Admin Distribution candidate contract. Start at
[`docs/getting-started`](./docs/getting-started/index.md) for adoption status;
operational onboarding remains disabled until v1.3.7 has complete
public reconciliation.

## Audience and authority map

| Audience | Published or repository location | Authority |
| --- | --- | --- |
| Adopters, operators, and contributors | `docs/docs/`, root README files, and `CONTRIBUTING.md` | Human-facing guidance; the Dumi site publishes `docs/docs/` |
| Architecture maintainers | `docs/adr/` | Durable decisions and their status; repository-only, not an adopter tutorial |
| Foundation AI Agents | nearest `AGENTS.md` -> `.mss/` -> applicable `.agents/skills/` | Executable repository contract |
| Generated Thin Host AI Agents | generated `AGENTS.md`, `.mss/`, and local `.agents/skills/` | Executable downstream contract; separate from Foundation maintenance |

`docs/docs/agent/` is deliberately part of the first row: it is a human-readable
guide to Agent collaboration, not an executable instruction source. It links to
the authoritative local contracts instead of copying their release and command
logic.

Within the public site:

- `docs/getting-started/`: adoption status, package and tool availability, and mss-shop;
- `docs/guide/`: day-to-day operation and troubleshooting;
- `docs/admin/`: current Admin configuration, security, and operations;
- `docs/agent/`: human-readable Agent collaboration and contract navigation;
- `docs/architecture/`: current architecture summaries;
- `docs/releases/`: the v1.3.7 candidate, v1.3.6 and v1.3.5 immutable-partial records, and a clearly separated read-only archive.

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

The drift check verifies the v1.3.2 stable, v1.3.5/v1.3.6 immutable-partial, and
stage-sensitive v1.3.7 candidate boundaries; rejects candidate-stage v1.3.7
adopter commands; and checks the audience split, navigation targets, internal
links, archive banners, and ADR status markers.
