# Monorepo guide

## Layout

| Path | Purpose |
| --- | --- |
| `/` | `github.com/mss-boot-io/mss-boot-admin`, the Go admin backend |
| `mss-boot/` | `github.com/mss-boot-io/mss-boot-admin/mss-boot`, the reusable Go framework |
| `web/antd/` | React + Ant Design frontend |
| `docs/` | Dumi documentation site |

`go.work` activates the backend and framework modules together. Do not add a local `replace` directive to either published `go.mod`; workspace mode is the source of truth for repository-local development.

Until the first stable nested-module tag is published, `go.work` contains a version-scoped bridge from `mss-boot v1.0.0` to `./mss-boot`. After publishing `mss-boot/v1.0.0`, remove that bridge, refresh the root module metadata, and verify a build with `GOWORK=off`. Results produced for the unpublished v0.8.0 candidate do not prove that the v1.0.0 module resolves.

## Common commands

```shell
make deps-all
make test-all
make web-install web-lint web-test web-build
make docs-install docs-build
```

## Release tags

- Backend application: `vX.Y.Z`
- Framework module: `mss-boot/vX.Y.Z`
- Ant Design frontend: `web/antd/vX.Y.Z`
- Documentation: built and deployed from `main`

The first stable framework release after consolidation is `mss-boot/v1.0.0`. For a coordinated release, publish the framework tag before the backend tag so the backend's `go.mod` requirement is available outside workspace mode.

## Workflow location

Only workflows under the repository-root `.github/workflows/` directory are active. Workflow files preserved inside imported component directories are historical source snapshots and are not executed by GitHub Actions.

## Migration boundary

See `MONOREPO_IMPORTS.md` for the exact source repositories and imported commit revisions. After this migration is merged and production verification succeeds, the three former standalone repositories should be marked deprecated and archived rather than deleted, preserving issues, releases, tags, and external links.
