# MSS Admin — Ant Design 6

This is the independent React 19 and Ant Design 6.6.0 Admin application. It has
its own dependency graph, build output, image, tag namespace, deployment, and
rollback history. The legacy application remains in `web/antd`.

This checkpoint is a buildable foundation, not a production release candidate.
Opt-in backend cookie/CSRF, OAuth transport binding, WebSocket-ticket support,
the typed identity/menu startup chain, and the layered Ant Design 6 theme editor
and runtime are implemented. The safe account slice now includes the account
center, allowlisted profile editing, one-time PAT handling, provider-gated OAuth
login/connection, notifications, synchronized language switching, and server-pushed
authorization revision reconciliation. The read-only operations slices replace
the workplace placeholder with a responsive service monitor backed by authoritative,
bounded server history and add a root-only online-session inventory with audited revoke
actions. The first core CRUD slices add bounded, permission-separated language
and Option management with optimistic revisions and constrained runtime behavior.
Supplier is the first dual-target golden module: one specification now emits an isolated
V6 typed contract, React Query data layer, responsive CRUD page, compiled route, locale
catalog, contract tests, and HttpOnly/CSRF-aware Playwright flow without writing into V5.
Retained business modules and required
browser evidence remain release blockers and are fail-closed by the qualification contract.

The runtime has one application-owned React Query client. Current identity and
the authorized menu are loaded through it, but only the verified identity and
startup-critical derived state enter Umi initial state. Backend `/welcome` is
mapped explicitly to compiled `/workplace`; database component strings and unknown
paths never select executable code. Theme values resolve field-by-field as
V6 code defaults < public application profile < authenticated personal resource
and are applied through Ant Design 6 CSS variables and semantic tokens.
Application and personal settings reuse one scope-explicit editor with canonical
ETag writes, visible 412 resolution, field inheritance, whole-layer reset, and
read-only RBAC states. A schema-versioned cross-tab channel converges monotonic
revisions. The optional 24-hour first-paint snapshot contains only the seven theme
fields and metadata; personal snapshots use a random session key additionally
bound to the verified current-user subject. Production builds alias transitional
Moment consumers to Day.js and fail if a Moment runtime enters the bundle. The
browser contract is Chromium/Edge 120+, Firefox 121+, and Safari/WebKit 17.4+.
Umi is compiled for that explicit evergreen baseline, so the release bundle also
fails if the legacy default `core-js` graph reappears.

Account identity fields are intentionally not copied from the legacy form contract:
username and email remain read-only until a verified identity-change workflow exists.
PAT secrets live only in a one-time component-memory dialog and never enter React
Query mutation data or browser persistence. OAuth disconnect and self-service
password change remain unavailable until the backend can require recent re-auth and
guarantee that an account retains another verified login method.

Authorization is reconciled from the server after explicit and cross-tab refresh
events, 403 responses, network recovery, and throttled focus/visibility changes. A
changed privilege/menu signature evicts domain queries and mutation results while
retaining only exact startup query families. An unconfirmed identity or menu replaces
the rendered application with a retryable fail-closed state. After a committed policy
revision has successfully reloaded, the backend sends only its monotonic decimal
revision through a non-blocking secure WebSocket event. The browser deduplicates the
hint across tabs and reloads current identity and the compiled-menu intersection over
HTTP. Bounded reconnect and focus/visibility/403 reconciliation remain correctness
fallbacks; WebSocket payloads never become an authorization cache.

The workplace monitor has its own React Query key and a strict transport contract.
Successful refresh follows the server sample interval, `503 Retry-After` is bounded and
honored, transient failures back off while retaining last-good data, and `401`/`403`
stop automatic polling. Loading, empty history, warm-up, forbidden, stale, and refresh
failure states are explicit in both locales. CPU and memory history is rendered with an
accessible, token-aware native SVG, so this slice adds no chart runtime dependency.

The online-session inventory intersects a compiled root-only route with the backend
root guard, validates every list/detail response, caps pages at 100 rows, and stops its
foreground-only 30-second polling after `401` or `403`. Refresh failures keep the last
authoritative page visible. Single-session and per-user revocation are row-bound and
audited; the legacy arbitrary user-ID action was intentionally not copied. This page uses
Ant Design 6 `Table` because its controlled list does not need ProTable schema features:
the release gate measured 952.10 KiB with ProTable versus 848.54 KiB with Table.

Language management uses a small native `Table`, `Form.List`, and Ant Design 6.6
`Listy` rather than importing ProTable for capabilities it does not need. List payloads
omit definitions, detail is loaded on demand, writes contain only client-owned fields,
and `expectedUpdatedAt` protects edits without discarding a conflicting draft. The
backend canonicalizes BCP 47 tags, assigns identifiers, caps languages and definition
payloads (including a 1 MiB aggregate public-response cap), and keeps
read/create/update/delete grants independent. Public resources expose
only enabled projections. Runtime profile loading is optional and time-bounded; only the
complete shipped `zh-CN` and `en-US` catalogs can be overlaid. Adding another stored BCP 47
language does not silently advertise an incomplete application locale.
After replacing icon-barrel imports with public icon subpaths and aligning Umi's
polyfills with the supported browser contract, the resulting release build is
4.16 KiB at the entry, 881.98 KiB total JavaScript, and 199.59 KiB for the largest
asynchronous chunk after the Supplier golden. It passes the existing 900/250 KiB
budget with 18.02 KiB of total headroom without weakening the gate.

Option management replaces the legacy generic controller and client-generated
item identity with an explicit bounded contract. Lists use a summary projection,
details load on demand, and updates/deletes use strong `If-Match` revisions. The
backend records a complete prior-resource snapshot in the same transaction,
protects built-in identity and deletion, blocks deletion while an enabled usage
exists, and invalidates tenant-namespaced cache entries after commit. The editor
preserves its draft on 412 and requires an explicit reload; opaque `extra`, `icon`,
and `color` metadata are displayed as inert text rather than executable UI input.

The upstream engineering reference is Ant Design Pro v6.0.2 commit
`2b453c67b535b76f5f95d6542397a4b987b61de2`; runtime and build packages are
resolved and pinned independently for this repository.

Generated V6 artifacts are owned by `mss module generate`, so Biome does not rewrite or
reorder them after generation. They remain covered by Biome lint, strict TypeScript,
Vitest, production build, and deterministic drift checks. Regenerate and verify Supplier
from the repository root with:

```shell
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --frontend-target antd-v6 --write
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --frontend-target antd-v6 --check
```

The initial generator profile is deliberately narrower than the full AdminModule schema:
it qualifies timestamped, uuid/string-ID full CRUD+export modules with non-null
string/text/uuid/enum/bool fields. Numeric, file, relation, immutable-editor, batch,
import, and workflow semantics fail specification validation until their V6 controls and
contracts are implemented. Required create fields must be visible in the editor, and an
E2E-enabled module must expose deterministic visible marker/update fields; the generator
never substitutes a generic text input or emits an unrepeatable browser flow silently.

```shell
corepack pnpm@10.34.5 install --frozen-lockfile
corepack pnpm@10.34.5 lint
corepack pnpm@10.34.5 test:ci
corepack pnpm@10.34.5 build:release
```

## Browser-session backend

V6 deliberately does not use the legacy bearer response or URL WebSocket token.
Its browser contract is:

| Operation | Endpoint / transport |
| --- | --- |
| Login | `POST /admin/api/user/session/login` |
| Refresh | `POST /admin/api/user/session/refresh-token` |
| Logout | `POST /admin/api/online-sessions/logout` |
| OAuth authorize | `POST /admin/api/user/session/oauth2/authorize` |
| OAuth callback | `POST /admin/api/user/session/{provider}/callback` |
| WebSocket ticket | `POST /admin/api/ws/tickets` |
| WebSocket connect | `/admin/api/ws/connect-v6` with the ticket in `Sec-WebSocket-Protocol` |
| Authorization refresh | `authorization` event with a decimal revision hint; protected HTTP remains authoritative |

The backend capability is disabled by default so a backend deployment can precede
the V6 rollout without changing V5. A production environment must enable server
sessions and browser sessions explicitly:

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
  sessionEnabled: true
  browserSession:
    enabled: true
    secure: true
    sameSite: lax
    webSocketTicketTTL: 30s
    legacyWebSocketQueryTokenEnabled: true
```

Use one exact HTTPS origin and a shared Redis cache in production. Keep the legacy
WebSocket switch enabled while V5 is deployed; disable it only after V5 retirement.
The recommended production topology serves the static V6 app and `/admin/api` from
the same origin. Local development origins on port `8001` are present in the default
development configuration.

OAuth rollout also uses V6-specific provider applications so independently deployed
V5 callbacks and credentials are not overwritten. Configure
`security:githubBrowserSessionClientId`, `githubBrowserSessionClientSecret`,
`githubBrowserSessionRedirectURI` (and optional `githubBrowserSessionScope`), plus
the corresponding `larkBrowserSessionAppId`, `larkBrowserSessionAppSecret`, and
`larkBrowserSessionRedirectURI`. Register those exact callback URLs with the providers;
the legacy settings remain owned by V5 during the overlap window.

See `.mss/features/admin-antd-v6-application.yaml` and
`docs/adr/2026-08-15-independent-ant-design-v6-application.md` plus
`docs/adr/2026-08-15-browser-session-and-websocket-ticket.md` for the complete
contract and rollout boundary.
