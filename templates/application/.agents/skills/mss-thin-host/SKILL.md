---
name: mss-thin-host
description: Safely develop and validate this generated MSS v1.3.6 Thin Host while keeping exact Admin Distribution dependencies and business-owned extensions separate.
---

# MSS Thin Host

1. Read `AGENTS.md`, `.mss/project.yaml`, `.mss/capabilities.yaml`, and the affected `.mss/modules/*.yaml` or `.mss/features/*.yaml`.
2. Use the installed release binaries. Confirm `mss --version` is v1.3.6 before generation or upgrade work; never compile a private local CLI as the adopter path.
3. Keep the Admin Go package, Framework, and Admin Web package at the coordinated exact Distribution version. Do not copy dependency source into this repository.
4. The v1.3.6 generated module profile is limited to `string`, `enum`, and `bool` fields, `ownership: none`, simple CRUD/export, coarse backend permissions, and the generated Admin UI. Relations, import, workflow, and row-scope behavior require a separate Feature specification and handwritten business-owned implementation.
5. Change `.mss/modules/<name>.yaml` before generated regions. Review `mss module generate <spec> --format json` before `--write`, then require `--check` to pass.
6. Keep custom backend code under `internal/modules/<name>/` and custom frontend code under `web/src/business/<name>/`. Backend authorization remains authoritative.
7. Run `mss verify --module <name>` while iterating and `mss verify --all` before delivery when the full application is in scope.
8. Upgrade only through a recorded `.mss/blueprint-manifest.json`: plan with `mss upgrade admin <version>`, review all conflicts, then use `--apply --yes` for the approved write.
