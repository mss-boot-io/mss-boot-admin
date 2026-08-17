---
title: V6 browser sessions, PAT, and OAuth2
order: 20
nav:
  order: 1
  title: admin
description: mss-boot-admin V6 browser authentication, personal access tokens, OAuth2, and security boundaries
keywords: [admin session token oauth2 pat api security audit]
---

# V6 browser sessions, PAT, and OAuth2

MSS deliberately separates browser authentication from non-browser API automation.
The V6 application uses a server-side session represented by an HttpOnly cookie;
automation uses a user-created personal access token (PAT) in the standard
`Authorization: Bearer` header. A PAT is supported API authentication, not a browser
fallback.

## Browser session contract

| Route | Method | Purpose |
| --- | --- | --- |
| `/admin/api/user/session/login` | POST | Password or email-challenge login; establishes V6 cookies |
| `/admin/api/user/session/refresh-token` | POST | Rotates the session before expiry |
| `/admin/api/online-sessions/logout` | POST | Revokes the current session and clears cookies |
| `/admin/api/user/auth-cookie/clear` | POST | Clears stale V6 browser cookies before a new attempt |

Responses contain expiry metadata but never an Admin JWT. State-changing browser
requests require an exact trusted `Origin` and `X-CSRF-Token` matching the signed
double-submit cookie. The session cookie is HttpOnly and uses Secure in production.
Visible tabs coordinate pre-expiry refresh; logout and authorization failure clear the
non-secret local expiry hint.

The historical token-returning login and refresh routes and `jwt` cookie are not
registered.

## Personal access tokens

A signed PAT contains its record identifier and current identity snapshot. The database
stores only a versioned SHA-256 digest and a display-safe fingerprint; it never stores or
lists the raw value.

| Route | Method | Purpose |
| --- | --- | --- |
| `/admin/api/user-auth-tokens` | GET | List the current user's active token metadata |
| `/admin/api/user-auth-tokens` | POST | Create a PAT and display it once |
| `/admin/api/user-auth-token/:id/refresh` | PUT | Rotate an owned PAT and display the replacement once |
| `/admin/api/user-auth-token/:id/revoke` | PUT | Revoke an owned PAT |

Creation and rotation responses use `Cache-Control: no-store`. Raw values never enter a
shared React Query cache, URL, local storage, log, or audit body. PAT callers cannot
manage PATs, rotate passwords, change recovery identity, bind or disconnect OAuth, or
perform other interactive proof-gated actions; those cases fail with 403.

Example non-browser call:

```shell
curl https://admin.example.com/admin/api/user/userInfo \
  -H 'Authorization: Bearer <one-time-personal-access-token>'
```

PAT authorization still follows the associated user's current role and backend RBAC.
Disabling the user, changing the role, revoking the record, or rotating the PAT invalidates
the prior credential.

## OAuth2 browser flow

GitHub and Lark use one browser-session flow:

1. The V6 client posts provider and `login`, `binding`, or `reauthentication` intent to
   `/admin/api/user/session/oauth2/authorize`.
2. The server creates a high-entropy single-use state bound to provider, intent,
   browser nonce, attempt, and—when applicable—the verified user and durable session.
3. The provider redirects to `/user/oauth/callback/:provider`.
4. The callback page immediately removes `code` and `state` from the address bar and
   posts them to `/admin/api/user/session/:provider/callback`.
5. The server consumes state atomically, exchanges the provider code, and either creates
   the V6 session, completes a binding, or records recent reauthentication.

The callback response never contains an Admin token or provider access/refresh token.
Provider tokens are not stored in the user binding model. OAuth login return targets are
validated before authorization, stored by attempt in tab-scoped state, consumed once,
and never accepted from a callback query parameter.

Current-user OAuth routes are:

| Route | Method | Purpose |
| --- | --- | --- |
| `/admin/api/user/oauth2` | GET | List current user's provider bindings |
| `/admin/api/user/oauth2/:provider` | DELETE | Disconnect after recent server-side proof |
| `/admin/api/user/security/reauthenticate` | POST | Establish recent password proof |
| `/admin/api/user/security/password` | PUT | Change the user's own password |

Disconnect and password change fail closed if they would remove the final verified login
method. A successful password change revokes all other browser sessions and active PATs.
Passwords are one-way verifier material owned by the user; the platform cannot decrypt
or retrieve them.

## OAuth configuration

Only BrowserSession names are supported in the `security` application group:

- `githubBrowserSessionClientId`
- `githubBrowserSessionClientSecret`
- `githubBrowserSessionRedirectURI`
- `githubBrowserSessionScope`
- `larkBrowserSessionAppId`
- `larkBrowserSessionAppSecret`
- `larkBrowserSessionRedirectURI`

Secrets require explicit secret-read/write permissions, are write-only in normal UI
flows, and never enter the public application profile. Retired configuration names are
removed by an exact, idempotent migration and rejected by generic configuration writes.

Production OAuth requires HTTPS, exact callback URIs, a shared Redis state store for
multiple instances, and a unique random authentication key of at least 32 bytes.

## WebSocket authentication

The V6 client requests a short-lived single-use ticket over authenticated HTTP and sends
it through `Sec-WebSocket-Protocol` when connecting to `/ws/connect`. The server validates
the exact configured Origin and atomically consumes the ticket. Tokens in the URL and the
alternate historical WebSocket route are not accepted. Logs redact credentials before
persistence.

## Operational checks

- Audit successful and failed login, reauthentication, password, PAT, OAuth, and session
  revocation outcomes without request credentials.
- Rotate provider secrets and `auth.key` through controlled release procedures; rotating
  `auth.key` invalidates active sessions and PAT signatures.
- Test positive and negative identity, owner, role-drift, replay, Origin, CSRF, expiry,
  and final-login-method cases before release.
- Use the V6 application for interactive verification; never copy a PAT into browser
  source or storage.
