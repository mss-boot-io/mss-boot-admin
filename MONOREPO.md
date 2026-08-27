# Monorepo guide

## Layout

| Path | Purpose |
| --- | --- |
| `/` | Agent/Foundation Go tooling and shared repository contracts |
| `admin/` | `github.com/mss-boot-io/mss-boot-admin/admin`, the complete importable Admin application |
| `mss-boot/` | `github.com/mss-boot-io/mss-boot-admin/mss-boot`, the reusable Go framework |
| `web/antd-v6/` | Sole React 19 + Ant Design 6 source and `@mss-boot-io/admin-web` package |
| `docs/` | Dumi documentation site |

`go.work` activates the backend and framework modules together. Do not add a local `replace` directive to either published `go.mod`; workspace mode is the source of truth for repository-local development.

`go.work` is only a repository-development convenience. Every publishable Go
module and the generated Thin Host are also qualified with `GOWORK=off`; normal
module files never contain a local checkout replacement.

## Common commands

```shell
make deps-all
make test-all
make web-install web-lint web-test web-build
make docs-install docs-build
```

## Release tags

- Root Admin Distribution: `vX.Y.Z`
- Framework module: `mss-boot/vX.Y.Z`
- Complete Admin Go module: `admin/vX.Y.Z`
- Admin Web package and image: `web/antd-v6/vX.Y.Z`
- Documentation: `docs/vX.Y.Z`

Runtime components have independently triggered workflows, but one Admin
Distribution release requires the same version core and exact merged-main commit.
v1.3.5 is permanently stopped as an immutable partial release and cannot be
qualified, resumed, completed, or reused.

For a future unused version, run one non-publishing Root preview on the exact
merged-main commit. After it succeeds, publish the Framework, Admin, and Admin Web
tags in order. One Root tag then starts the Root release, backend image, and
official npm publication independently and in parallel. Formal tag workflows
reuse the exact preview and do not repeat its expensive qualification or accept a
promotion, readiness run ID, second publish dispatch, or manual environment
approval. Docs follows later from its own tag and later merged-main documentation
commit without reissuing runtime components.

## Workflow location

Only workflows under the repository-root `.github/workflows/` directory are
active. Imported, nested workflow snapshots are not retained because GitHub does
not execute them in this monorepo.

## Import provenance

`MONOREPO_IMPORTS.md` records the source repositories and exact imported commits
as historical provenance. It does not define a current checkout, build, adoption,
or release path.
