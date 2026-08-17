# Use browser sessions and one-time WebSocket tickets for the V6 application

- Status: Accepted
- Date: 2026-08-15
- Updated: 2026-08-17
- Owners: Admin Platform, Security, Frontend
- Feature contract: `.mss/features/admin-antd-v6-application.yaml`
- Current cutover contract: `.mss/features/admin-antd-v6-cutover-retirement.yaml`

## Context

The V6 application is the only supported Admin browser client. Returning an Admin JWT
to browser JavaScript or accepting a credential in a WebSocket URL would expose a
long-lived credential to storage, URL logs, referrers, screenshots, and infrastructure
telemetry. Browser cookie authentication introduces CSRF risk, while the browser
WebSocket API cannot set an Authorization header.

## Decision

Use the browser-session capability as the only browser authentication contract. It is
mandatory and cannot be disabled independently from the Admin backend; production
deployments reject insecure cookies and invalid origins during startup.
Standard `Authorization: Bearer` and PAT authentication remain supported only for
documented non-browser API automation.

The canonical login and refresh endpoints use the verifier, session registry,
JWT signing, current-principal reload, and revocation checks. They return only status and
expiry metadata. The JWT is written to the host-only, HttpOnly
`mss_admin_session` cookie scoped to `/admin/api`; it is never returned in the V6 body.
The readable `mss_csrf` cookie is scoped to `/` so the frontend can copy it into the
`X-CSRF-Token` header.

Every unsafe cookie-authenticated request requires all three of:

- an exact normalized Origin from the application/CORS allowlist;
- an equal CSRF cookie and request header, compared in constant time;
- an HMAC-signed CSRF value bound to the digest of the current session JWT.

The CSRF value rotates whenever the session JWT rotates. A bearer header used by an API
client does not require browser CSRF. Query credentials are absent from the global JWT
lookup, so REST can never authenticate from a URL.

OAuth authorization state records the browser session attempt and is atomically consumed
before provider exchange. Browser callbacks set the session and CSRF cookies and omit the
Admin JWT. Provider access and refresh tokens remain server-only. OAuth uses the
`BrowserSession` provider client IDs, secrets, scopes, and redirect URI settings; the
retired bearer callback mode and its configuration keys are removed.

V6 WebSocket clients first request a 256-bit, short-lived, single-use ticket. Only a
digest is persisted. The ticket record binds the current user, role, server session, and
exact Origin. The browser sends `mss.v1` and `mss.ticket.{ticket}` in
`Sec-WebSocket-Protocol`; the URL contains no credential. Upgrade consumes the ticket
atomically, rechecks Origin, reloads current authority, and confirms the session is still
active. Production issuance and consumption require the shared cache.

After a committed global authorization revision has successfully reloaded into the
local policy engine, the server may broadcast an `authorization` event containing only
that revision as a decimal string. The fan-out queue is non-blocking and best-effort, so
realtime congestion cannot fail or delay the authoritative permission transaction. V6
deduplicates revision hints across its socket and sibling tabs, then reloads current
identity and the compiled authorized-menu intersection over protected HTTP. Bounded
reconnect, network/focus reconciliation, and 403-triggered refresh remain required
fallbacks; the WebSocket is never an authorization-policy cache.

`/ws/connect` accepts only the one-time ticket subprotocol. Query-token authentication
and its compatibility switch are removed.

## Deployment and rollback

Configure the exact HTTPS origin, shared Redis, production auth key, CSRF CORS header,
Secure cookies, and V6 OAuth provider applications with exact callback URIs. Run the
environment security smoke before publishing the V6 frontend and matching backend.

Rollback redeploys the preceding qualified V6 frontend and backend pair. It never
restores the retired query-token, token-returning browser, or bearer OAuth behavior.

## Consequences

The backend has one browser authentication transport and a separate standards-based API
automation transport. Cross-origin development remains possible only for explicit
origins; same-origin HTTPS is the production recommendation.

Tickets and OAuth states use an in-process fallback for single-process development.
WebSocket tickets fail closed without shared cache in production, and multi-replica
deployments must use Redis so issuance and consumption can land on different replicas.
