# mss-boot-admin

[![CI](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/releases)
[![License](https://img.shields.io/github/license/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/blob/main/LICENSE)

English | [简体中文](./README.zh-CN.md)

`mss-boot-admin` is an Agent-native management-system foundation. It combines
a production-oriented Go Admin, a React 19 + Ant Design 6 application,
machine-readable project contracts, deterministic full-stack generation,
change-aware verification, and upgradeable Thin Host Blueprints in one source
repository.

The active stable target is **v1.3.0**. Formal publication qualifies it as one
Complete Admin Distribution from one exact merged `main` commit. Published
v1.3.0 RC1-RC6 tags are permanently immutable preview history; before stable
publication and public reconciliation they are preview evidence only, never a
substitute for the exact stable artifacts.

## Complete Admin Distribution

The coordinated v1.3.0 train keeps these independently publishable components
on the same version and source commit:

| Component | Stable identity |
| --- | --- |
| Foundation tools and root delivery | `v1.3.0` |
| Reusable Go framework | `mss-boot/v1.3.0` |
| Importable Admin Go module | `admin/v1.3.0` |
| Complete Admin Web package | `web/antd-v6/v1.3.0` / `@mss-boot-io/admin-web@1.3.0` |
| Documentation | Independently released with `docs/vX.Y.Z` |

Identity, browser sessions, RBAC, menus, layout, localization, and the shared
runtime stay in one Admin product. A downstream application is a Thin Host: it
pins the complete backend and frontend, adds only owned business modules and
composition glue, and still produces one backend binary, one frontend `dist`,
one session boundary, and one permission/menu model.

Runtime dynamic models, virtual CRUD, and browser-facing code generation have
been removed. New business capabilities are described by Feature and
AdminModule contracts and generated deterministically at development time.

```text
business intent
  -> Feature and Acceptance contract
  -> AdminModule contract
  -> deterministic backend, migration, permission, menu, frontend, and tests
  -> owned business rules
  -> change-aware verification and Agent Evals
  -> reviewable PR and upgradeable Thin Host
```

## Consume v1.3.0

After the stable tags are public, Go consumers should pin the exact coordinated
version. The Admin module has no committed local `replace`:

```shell
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.0
go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.0
```

The complete frontend is published publicly to npmjs, so the default Thin Host
installation needs no registry credential:

```shell
corepack pnpm@10.34.5 add --save-exact @mss-boot-io/admin-web@1.3.0
```

The same immutable tarball is retained in GitHub Packages as a compatibility
mirror. Consumers that explicitly select that mirror must inject a
`read:packages` token only for the install process and must never commit it.

Stable packages receive the `latest` distribution tag. Prereleases receive
`next`; generated Thin Hosts still pin an exact version rather than a moving
tag. The official npm publication is the final artifact publication, after all
coordinated component and Docs releases resolve to the same merged-main commit.

## Create and upgrade a Thin Host

Create a host from a clean Foundation checkout. Then initialize and validate a
version-controlled module specification before generating the first vertical
business module:

```shell
go run ./cmd/mss new app orders-admin \
  --module github.com/acme/orders-admin \
  --repository acme/orders-admin \
  --destination ../orders-admin \
  --write \
  --format json

go run ./cmd/mss --root ../orders-admin spec init supplier \
  --kind module \
  --output .mss/modules/supplier.yaml \
  --write

go run ./cmd/mss --root ../orders-admin spec validate \
  .mss/modules/supplier.yaml \
  --format json

go run ./cmd/mss --root ../orders-admin module generate \
  .mss/modules/supplier.yaml \
  --write \
  --frontend-target antd-v6 \
  --format json
```

Plan an Admin Distribution upgrade before applying it:

```shell
go run ./cmd/mss --root ../orders-admin upgrade admin v1.3.0 \
  --foundation . \
  --format json

go run ./cmd/mss --root ../orders-admin upgrade admin v1.3.0 \
  --foundation . \
  --apply --yes \
  --format json
```

The three-way upgrade engine changes only Blueprint-managed host files. Unknown
and business-owned files are preserved, and a conflicting plan must be reviewed
instead of applied automatically.

## Local development

Requirements:

- Go 1.26.6 or later in the 1.26 line;
- Node.js 24;
- pnpm 10.34.5 through Corepack;
- optional MySQL, PostgreSQL, or Redis only for the integrations being tested.

```shell
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin

go run ./cmd/mss doctor
go run ./cmd/mss setup
(cd admin && STAGE=local go run . server -a)
go run ./cmd/mss dev --detach
go run ./cmd/mss dev status --format json
```

The Admin Web development server listens on `http://localhost:8001` and proxies
`/admin/` to the Go backend. The one-shot `server -a` command synchronizes
mounted routes into the API registry; without it, menu API binding can remain
empty even when both services are healthy.

## Product capabilities

- identity, HttpOnly browser sessions, OAuth account binding, PAT lifecycle,
  and online-session revocation;
- Casbin-backed RBAC, organization and data scopes, menu/API binding, and
  fail-closed backend authorization;
- user, role, menu, API, department, post, option, language, configuration,
  notice, task, audit, storage, monitoring, and statistics modules;
- React Query server state, synchronized zh-CN/en-US locales, responsive dark
  and light themes, permission, loading, empty, conflict, and error states;
- deterministic full-stack module generation, upgrade-safe migrations,
  repository Skills, Agent Evals, and external-consumer qualification.

Browser JavaScript does not receive the Admin JWT or provider credentials. The
browser uses the HttpOnly `mss_admin_session` cookie, a signed session-bound
CSRF token, and one-time WebSocket tickets. Standard Bearer and PAT
authentication remain available for documented non-browser API automation.

## Repository layout

| Path | Responsibility |
| --- | --- |
| `/`, `cmd/mss/`, `internal/mss/` | Agent CLI, orchestration, generation, verification, and upgrades |
| `.mss/` | Machine-readable project, capability, release, module, and evaluation contracts |
| `admin/` | Complete reusable and deployable Admin Go application |
| `mss-boot/` | Reusable domain-neutral Go framework module |
| `web/antd-v6/` | Official Admin Web application and complete npm package |
| `templates/` | Deterministic application and module templates |
| `docs/` | Product, architecture, operations, and contributor documentation |

## Verification

Use the repository contracts rather than a workstation-specific command list:

```shell
go run ./cmd/mss context
go run ./cmd/mss verify --changed
go run ./cmd/mss verify --all
```

Focused component commands remain available:

```shell
make test-framework
make test-all

corepack pnpm@10.34.5 --dir web/antd-v6 run deps:check
corepack pnpm@10.34.5 --dir web/antd-v6 run lint
corepack pnpm@10.34.5 --dir web/antd-v6 run test:ci
corepack pnpm@10.34.5 --dir web/antd-v6 run build:release
corepack pnpm@10.34.5 --dir web/antd-v6 run test:e2e

make docs-build
```

Run the Playwright suite only for browser-contract changes or formal
qualification. Tests require no production credentials, and no coverage
percentage is claimed unless the same threshold is enforced by CI.

## Documentation and community

- [Online documentation](https://docs.mss-boot-io.top)
- [OpenAPI document](https://mss-boot-io.github.io/mss-boot-admin/swagger.json)
- [Releases](https://github.com/mss-boot-io/mss-boot-admin/releases)
- [Security policy](./SECURITY.md)
- [Contributing](./CONTRIBUTING.md)
- [Video tutorials](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)

## License

[MIT](./LICENSE)

Copyright (c) 2024-2026 mss-boot-io
