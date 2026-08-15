# MSS Admin — Ant Design 6

This is the independent React 19 and Ant Design 6.6.0 Admin application. It has
its own dependency graph, build output, image, tag namespace, deployment, and
rollback history. The legacy application remains in `web/antd`.

This checkpoint is a buildable foundation, not a production release candidate.
Opt-in backend cookie/CSRF, OAuth transport binding, and WebSocket-ticket support
are implemented. Retained business modules, the v6 generator target, and required
browser evidence remain release blockers and are fail-closed by the repository
qualification contract.

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
