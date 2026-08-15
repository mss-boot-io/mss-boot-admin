# MSS Admin — Ant Design 6

This is the independent React 19 and Ant Design 6.6.0 Admin application. It has
its own dependency graph, build output, image, tag namespace, deployment, and
rollback history. The legacy application remains in `web/antd`.

This checkpoint is a buildable foundation, not a production release candidate.
Opt-in backend cookie/CSRF, OAuth transport binding, WebSocket-ticket support,
the typed identity/menu startup chain, and the layered Ant Design 6 theme editor
and runtime are implemented. The safe account slice now includes the account
center, allowlisted profile editing, one-time PAT handling, provider-gated OAuth
login/connection, notifications, and synchronized language switching. Permission
server-pushed authorization revisions, retained business modules, the v6 generator
target, and required browser evidence remain release blockers and are fail-closed by
the qualification contract.

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
Moment consumers to Day.js and fail if a Moment runtime enters the bundle.

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
the rendered application with a retryable fail-closed state. A secure WebSocket
revision hint will reduce focus-independent latency later; it is not treated as the
only correctness mechanism.

The upstream engineering reference is Ant Design Pro v6.0.2 commit
`2b453c67b535b76f5f95d6542397a4b987b61de2`; runtime and build packages are
resolved and pinned independently for this repository.

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
