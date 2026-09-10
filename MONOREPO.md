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
v1.3.5 is permanently stopped as an immutable partial release. v1.3.6 is
permanently stopped as an immutable partial release. Neither can be qualified,
resumed, completed, or reused. v1.3.6 published its Framework, Admin, Admin Web,
and Root releases, but its Root image and npm workflows failed and Docs was
never created.

v1.3.7 is the current stable and adoptable Distribution from exact release
commit `77b53d41092741eac62fa6418c0bdbf87413c7cd`. Framework, Admin,
Admin Web, Root tools, both images, official npm, npm `latest`, GitHub Latest,
and current-stable policy are reconciled. Docs is an asynchronous website
publication; its `docs/v*` tag, credential, workflow, Release, and site state
never gate the Distribution or adopter availability.

The v1.3.7 release ran one non-publishing Root preview on the exact merged-main
commit. The reusable container call received `release_preview: true`, and the
preview contained the verified Root image artifact. The npm Trusted Publisher
matched `npm-release.yml` plus `npm-auto`; no npm token was used. Framework,
Admin, Admin Web, and Root tags were then published in order. The Admin release
resolved the exact public Framework with `GOWORK=off`; Frontend, Root, and
Container consumed the staged preview artifacts.

Stable promotion was a separate reviewed policy decision. After it bound
v1.3.7 to the exact Root commit, `lwnmengjing` manually dispatched
`npm-release.yml` from the exact `v1.3.7` Root tag. The workflow published the
qualified Frontend Release tarball through Trusted Publishing/OIDC with
`npm publish --tag latest --provenance`; no `NPM_TOKEN`, `NODE_AUTH_TOKEN`,
temporary dist-tag, standalone `npm dist-tag`, or token fallback was used.
Only after npm version, `gitHead`, integrity, provenance, and `latest`
reconciled did the workflow promote the exact Root Release to GitHub Latest.
A rerun must reconstruct and verify the same Root tag, commit, package, image,
npm, and alias identities before accepting existing public state; it must not
inspect Docs.

The final current-stable policy reconciliation advanced current stable and
adopter availability immediately from the complete Distribution ledger. A
later qualified merged-main descendant may update the website by deleting the
current Docs Release and tag, proving both are absent, and recreating the same
`docs/<stable-version>` tag. Direct force-update is prohibited; core release
identities remain immutable.
Formal tag and promotion workflows do not repeat expensive qualification or
accept a readiness run ID or manual environment approval. A release-source
repair after public Distribution identity requires another unused product
version; a Docs failure uses only the independent Docs PR and same-tag
replacement path.

## Workflow location

Only workflows under the repository-root `.github/workflows/` directory are
active. Imported, nested workflow snapshots are not retained because GitHub does
not execute them in this monorepo.

## Import provenance

`MONOREPO_IMPORTS.md` records the source repositories and exact imported commits
as historical provenance. It does not define a current checkout, build, adoption,
or release path.
