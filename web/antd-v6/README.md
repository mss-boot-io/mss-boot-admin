# MSS Admin — Ant Design 6

`web/antd-v6` is the sole Admin browser source in this repository. The same
React 19 and Ant Design 6.6.0 source builds the official application and the
complete `@mss-boot-io/admin-web` package consumed by Thin Hosts. It retains its
own lockfile, immutable image, `web/antd-v6/v{version}` tag namespace,
deployment, and previous-V6 rollback history. Retired frontend source and
protocols are not build, runtime, or recovery inputs.

The coordinated stable target is `web/antd-v6/v1.3.1` and
`@mss-boot-io/admin-web@1.3.1`, to be published from the same merged-main commit as
the v1.3.1 Framework, Admin module, and root distribution. The incomplete v1.3.0
train and all RC tags remain immutable partial-train and preview history.

The upstream engineering reference is Ant Design Pro v6.0.2 commit
`2b453c67b535b76f5f95d6542397a4b987b61de2`. Runtime and build dependencies are
resolved and pinned for this repository, including the reviewed ProComponents
v3 prerelease required by Ant Design 6.

## Product scope

The application covers the retained Admin product:

- browser-session login, registration, recovery, scheduled refresh, and logout;
- account center and settings, personal password change, PAT lifecycle, OAuth
  login, binding, reauthentication, and safe disconnect;
- workplace, users, roles, menus, departments, posts, notices, tasks, languages,
  options, logs, monitoring, online sessions, and system configuration;
- application configuration, layered application/personal theme settings, and
  synchronized Chinese and English locales;
- deterministic generated modules, with Supplier as the golden generator and
  external-consumer fixture using the same route, menu, permission, and locale
  contracts as downstream business modules.

Every retained page treats backend authorization as authoritative and provides
the applicable loading, empty, error, permission-denied, conflict, desktop,
mobile, and locale states. PWA, analytics, AI demos, arbitrary database-selected
components, and removed runtime developer tools are deliberately excluded.

## Architecture

- React Query owns server state and invalidation. Umi initial state contains
  only verified identity and startup-critical derived state.
- The compiled route registry is intersected with the backend-authorized menu;
  database component strings never select executable code.
- Ant Design 6 CSS variables and semantic tokens own the design system.
  Tailwind owns layout utilities, CSS Modules own local static styles, and
  antd-style owns complex token-aware rules.
- Responsive behavior is shared instead of maintaining parallel desktop and
  mobile business pages.
- Theme precedence is V6 defaults, application resource, then personal
  resource. Canonical ETags, visible `412` reconciliation, layer reset, and
  V6-namespaced cross-tab synchronization prevent lost updates and storage
  collisions.
- WebSocket events carry only revision hints and invalidate HTTP-owned state;
  they never become a second authorization or business-data cache.
- The supported browser baseline is Chromium/Edge 120+, Firefox 121+, and
  Safari/WebKit 17.4+.

## Browser security contract

Browser JavaScript never receives an Admin JWT or provider token. The sole
browser transport is an HttpOnly `mss_admin_session` cookie paired with a signed,
session-bound double-submit CSRF token.

| Operation | Endpoint / transport |
| --- | --- |
| Login | `POST /admin/api/user/session/login` |
| Refresh | `POST /admin/api/user/session/refresh-token` |
| Clear login cookie | `POST /admin/api/user/auth-cookie/clear` |
| Logout and revoke session | `POST /admin/api/online-sessions/logout` |
| OAuth authorize | `POST /admin/api/user/session/oauth2/authorize` |
| OAuth callback | `POST /admin/api/user/session/{provider}/callback` |
| WebSocket ticket | `POST /admin/api/ws/tickets` |
| WebSocket connect | `/admin/api/ws/connect` with a one-time ticket in `Sec-WebSocket-Protocol` |

Standard `Authorization: Bearer` and PAT authentication remain available for
documented non-browser API automation. Token-returning browser login/refresh,
bearer OAuth callback mode, the historical `jwt` cookie, query-token WebSocket
authentication, and alternate WebSocket routes are not registered.

Local development uses the committed `admin/config/application-local.yml`
overlay. Production must provide an exact HTTPS application origin, a strong
authentication key, Secure cookies, and shared Redis state:

```yaml
application:
  mode: prod
  origin: https://admin.example.com
cors:
  allowOrigins:
    - https://admin.example.com
  allowHeaders:
    - Authorization
    - Content-Type
    - If-Match
    - X-CSRF-Token
auth:
  browserSession:
    secure: true
    sameSite: lax
    webSocketTicketTTL: 30s
```

The browser session and its server-side revocation check are mandatory backend
contracts. There is no stateless or retired-browser compatibility switch.

OAuth uses only the `security:githubBrowserSession*` and
`security:larkBrowserSession*` configuration families. Provider secrets remain
server-side, and registered callback URLs must exactly match the configured
HTTPS V6 callback URLs.

## Development

The development application owns port `8001` and proxies `/admin/` to the Go
backend. The repository-local toolchain is Node 24 and pnpm 10.34.5 through
Corepack.

```shell
corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 start:dev
```

The local container profile serves the production build on the same port:

```shell
make web-build
docker compose -f compose/admin/docker-compose.yml up --detach --build
curl --fail http://localhost:8001/healthz
```

## Thin Host consumption

The package contains the complete core application. A downstream repository
adds only its business routes, generated menu registry, locales, and pages.

Packages are published publicly to npmjs and mirrored byte-for-byte in GitHub
Packages. Stable releases receive the `latest` distribution tag and
prereleases receive `next`; Thin Hosts pin an exact package version. The
generated `web/.npmrc` selects the public registry and needs no credential:

```shell
corepack pnpm@10.34.5 add --save-exact @mss-boot-io/admin-web@1.3.0
corepack pnpm@10.34.5 install --frozen-lockfile
```

The GitHub Packages mirror remains available for compatibility. A consumer
that opts into it must inject a short-lived `read:packages` token only for the
install process and must never commit the expanded token or a user-level
`.npmrc`.

```ts
// web/config/config.ts
import { defineBusinessAdmin } from '@mss-boot-io/admin-web/business';
import businessRoutes from './business-routes.generated';

export default defineBusinessAdmin({
  businessRoutes,
  routeRegistrations: './src/generated/routes.ts',
});
```

The generated host re-exports the package runtime from `src/app.tsx` and
`src/access.ts`, and merges package locales before business locales. It does not
copy core pages or `src/shared`, change the host `@` alias, load remote entries,
or start a second Umi application. Its normal commands are:

```shell
corepack pnpm@10.34.5 run dev
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test
corepack pnpm@10.34.5 run build
```

The generated `pnpm.overrides` block is part of the Admin Distribution
contract. Preserve it so package and business code resolve one React, Ant
Design, ProComponents, React Query, Axios, and Umi runtime graph.

## Generation

`mss module generate` owns generated V6 artifacts. `antd-v6` is the only
frontend target. Paths come from the target Project contract, so the same spec
writes to `web/antd-v6` in the Foundation and `web` in a Thin Host.

```shell
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --frontend-target antd-v6 --write
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --frontend-target antd-v6 --check
```

The initial generator profile is intentionally strict: unsupported relations,
files, numeric semantics, immutable editors, batch actions, imports, and
workflows fail specification validation instead of producing approximate UI.

## Qualification

Development uses focused checks. The complete suite is run together before PR
handoff and again from the exact merged-main release commit:

```shell
corepack pnpm@10.34.5 run deps:check
corepack pnpm@10.34.5 run lint
corepack pnpm@10.34.5 run test:ci
corepack pnpm@10.34.5 run test:e2e
corepack pnpm@10.34.5 run build:release
corepack pnpm@10.34.5 run delivery:smoke
corepack pnpm@10.34.5 pack
```

`test:e2e` can start a disposable Go backend on `127.0.0.1:18080`, V6 on
`127.0.0.1:8001`, and a repository-local SQLite database under
`.mss/run/antd-v6-e2e`. It requires no production credentials.

The repository-level compatibility gate additionally installs the packed
tarball in a freshly generated repository outside the monorepo, builds exactly
one `dist`, checks the installed runtime graph, and runs the Supplier and core
security Playwright paths against that external application.

Current long-lived contracts are recorded in:

- `.mss/features/admin-antd-v6-application.yaml`;
- `.mss/features/admin-antd-v6-cutover-retirement.yaml`;
- `docs/adr/2026-08-17-ant-design-v6-default-cutover.md`;
- `docs/adr/2026-08-15-browser-session-and-websocket-ticket.md`.
