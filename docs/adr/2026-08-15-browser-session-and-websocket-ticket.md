# Add an opt-in browser session and one-time WebSocket ticket transport

- Status: Accepted
- Date: 2026-08-15
- Owners: Admin Platform, Security, Frontend
- Feature contract: `.mss/features/admin-antd-v6-application.yaml`

## Context

The independently released Ant Design 6 browser application must coexist with the
legacy frontend. V5 receives an Admin JWT in a login response and uses a bearer header;
its WebSocket route historically accepts that credential in the URL query. Copying
those patterns into V6 would expose a long-lived credential to browser JavaScript,
storage, URL logs, referrers, screenshots, and infrastructure telemetry.

Changing the existing endpoints in place would also break a separately deployed V5
artifact and remove the proven rollback path. Browser cookie authentication introduces
CSRF risk, and the browser WebSocket API cannot set an Authorization header.

## Decision

Add a disabled-by-default browser-session capability alongside the existing bearer
contract. It is enabled only when both `auth.sessionEnabled` and
`auth.browserSession.enabled` are true. Production startup rejects the capability unless
cookies are configured Secure.

Dedicated V6 login and refresh endpoints use the canonical verifier, session registry,
JWT signing, current-principal reload, and revocation checks. They return only status and
expiry metadata. The JWT is written to the host-only, HttpOnly
`mss_admin_session` cookie scoped to `/admin/api`; it is never returned in the V6 body.
The readable `mss_csrf` cookie is scoped to `/` so the frontend can copy it into the
`X-CSRF-Token` header.

Every unsafe cookie-authenticated request requires all three of:

- an exact normalized Origin from the application/CORS allowlist;
- an equal CSRF cookie and request header, compared in constant time;
- an HMAC-signed CSRF value bound to the digest of the current session JWT.

The CSRF value rotates whenever the session JWT rotates. A bearer header remains the V5
and automation transport and does not require CSRF. Query credentials are removed from
the global JWT lookup, so REST can never authenticate from a URL.

OAuth authorization state records the selected `bearer` or `browser-cookie` transport.
The state is atomically consumed before provider exchange and cannot be replayed through
the other callback mode. Browser callbacks set the session and CSRF cookies and omit the
Admin JWT. Provider access and refresh tokens remain server-only. V6 uses distinct
browser-session provider client IDs, secrets, scopes, and redirect URI settings so
enabling its provider application cannot overwrite V5 credentials or callback
configuration.

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

The old `/ws/connect?token=...` path retains a dedicated compatibility middleware and an
explicit `legacyWebSocketQueryTokenEnabled` switch. No other route can read that query
credential.

## Deployment and rollback

Deploy the compatible backend with browser sessions disabled first. Configure the exact
HTTPS origin, shared Redis, production auth key, CSRF CORS header, Secure cookies, and
dedicated V6 OAuth provider applications with exact callback URIs; then enable
server/browser sessions and run the environment security smoke before publishing the
independent V6 image. Keep the legacy WebSocket switch on until V5 is retired.

Rolling back V6 redeploys its previous immutable image. The additive backend endpoints
remain in place and V5 bearer behavior is unchanged. Removing compatibility endpoints or
turning off the legacy WebSocket path requires a separate reviewed change after the
overlap window.

## Consequences

The backend carries two explicit authentication transports temporarily. This adds route,
configuration, test, and operational surface, but makes the browser boundary observable
and prevents an all-at-once migration. Cross-origin development remains possible only
for explicit origins; same-origin HTTPS is the production recommendation.

Tickets and OAuth states use an in-process fallback for single-process development.
WebSocket tickets fail closed without shared cache in production, and multi-replica
deployments must use Redis so issuance and consumption can land on different replicas.
