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

v1.3.7 is the selected release candidate and is not stable or adoptable.
Candidate surfaces may be at different public stages; the remote release ledger
is authoritative. Installation, application creation, and upgrades remain
closed until stable promotion and the final policy/Docs reconciliation complete.
Run one non-publishing Root preview for v1.3.7 on the exact merged-main commit.
The reusable container call must receive `release_preview: true`, and the
preview must contain the verified Root image artifact. The npm Trusted
Publisher must match `npm-release.yml` plus `npm-auto`; no npm token is used.
After it succeeds, publish the Framework, Admin, and Admin Web tags in order.
The first Framework workflow performs one cheap exact-preview lookup before the
first irreversible Release; Admin then checks the exact public Framework without
repeating that lookup. Frontend, Root, and Container directly consume staged
preview artifacts. The Root tag starts only the Root Release and backend-image
candidate; it explicitly leaves GitHub Latest unchanged. After Root and both
versioned images reconcile, publish `docs/v1.3.7` from the same exact Root
commit and reconcile the candidate Docs site. Throughout this candidate phase,
GitHub Latest and npmjs `latest` remain v1.3.2.

Stable promotion is a separate reviewed policy decision. Only after that policy
binds v1.3.7 to the exact Root commit may `lwnmengjing` manually dispatch
`npm-release.yml` from the exact `v1.3.7` Root tag. That workflow publishes the
already-qualified Frontend Release tarball through Trusted Publishing/OIDC with
`npm publish --tag latest --provenance`; no `NPM_TOKEN`, `NODE_AUTH_TOKEN`,
temporary dist-tag, standalone `npm dist-tag`, or token fallback is allowed.
Only after npm version, `gitHead`, integrity, provenance, and `latest` reconcile
may the workflow promote the exact Root Release to GitHub Latest. A rerun must
reconstruct and verify the same Root tag, commit, candidate Docs, package,
image, npm, and alias identities before accepting existing public state.

The final stable-policy and human-documentation reconciliation follows through
another PR, whose exact merged commit becomes the only possible source for any
stable-wording Docs update. If candidate Docs needs that wording, a subsequent
reviewed one-shot authorization must bind `docsRevisionVersion` to the lowest
unused `v1.3.7+docs.N` identity and `docsRevisionCommit` to that exact source;
then publish the immutable `docs/v1.3.7+docs.N` revision instead of moving
`docs/v1.3.7`, and disable the consumed authorization. Formal tag and promotion
workflows do not repeat expensive qualification or accept a readiness run ID or
manual environment approval. A failure after any public v1.3.7 identity
requires another unused version when source repair is necessary, not a late
artifact or moved immutable ref.

## Workflow location

Only workflows under the repository-root `.github/workflows/` directory are
active. Imported, nested workflow snapshots are not retained because GitHub does
not execute them in this monorepo.

## Import provenance

`MONOREPO_IMPORTS.md` records the source repositories and exact imported commits
as historical provenance. It does not define a current checkout, build, adoption,
or release path.
