# Monorepo source imports

The following repositories were imported with `git subtree` without `--squash`, so their source commit histories remain reachable from this repository.

| Imported destination | Source repository | Imported main revision | Current status |
| --- | --- | --- | --- |
| `mss-boot/` | `mss-boot-io/mss-boot` | `9d84520947e0a3189257c3c7a5209bdb6b5c38dd` | Active |
| `web/antd/` | `mss-boot-io/mss-boot-admin-antd` | `467e742900796baf6c6532e152fc101da604910b` | Retired; source removed, history retained in Git |
| `docs/` | `mss-boot-io/mss-boot-docs` | `3d98699c0df3361b2e63f9d49f545b51b5571a3f` | Active |

This file records import provenance, not the current repository map. The sole active frontend is
`web/antd-v6/`; new development is performed only in `mss-boot-io/mss-boot-admin`.
