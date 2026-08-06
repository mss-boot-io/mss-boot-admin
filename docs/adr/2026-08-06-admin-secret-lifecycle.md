# Admin secret lifecycle and OAuth credential confinement

- Status: Accepted
- Date: 2026-08-06
- Baseline: `main` at `60aff0feeea7706efc3cdc7c4b309559f4026f2c`
- Specification: `.mss/features/admin-secret-lifecycle.yaml`
- Scope: Admin PAT persistence and rotation, OAuth callback completion, and short-lived generator credentials

## Context

The Admin application used signed JWTs as personal access tokens (PATs), but
persisted the complete bearer value and returned it from the list endpoint. The
database record ID was embedded in the signed `personAccessToken` claim and was
the only value checked after JWT verification. Consequently, retaining the raw
JWT in the database provided no authentication benefit, repeated list requests
could recover it, and replacing the stored text did not invalidate an older
bearer value.

The PAT refresh endpoint also generated a token from the current interactive
principal without restoring the PAT-only claims. A refreshed value could
therefore behave as an interactive session and bypass restrictions imposed on
automation credentials.

OAuth callbacks returned provider access and refresh tokens to frontend code.
The frontend persisted them in `localStorage`; the template generator then sent
the GitHub access token in query parameters and a request body. This exposed a
provider credential to browser persistence, XSS, URL and proxy logs, audit
records, and shared-machine recovery.

## Decision

### Personal access tokens

The existing signed JWT wire format and route paths remain in place. The signed
`personAccessToken` claim continues to select one owner-scoped database record.
The database stores a versioned SHA-256 digest of the complete raw bearer and a
short non-secret fingerprint. SHA-256 is appropriate here because the input is
a high-entropy signed credential rather than a human password.

Authentication performs these steps:

1. verify the JWT signature and expiry;
2. read the PAT record ID from the signed claim;
3. load the non-revoked record by ID;
4. hash the exact raw bearer presented by the client;
5. compare the stored and presented digests in constant time;
6. load the current user and role.

Create and rotate responses use an explicit secret-bearing DTO. The list uses a
metadata-only DTO. A raw PAT is never part of a persistent model and is shown by
the frontend only until the one-time dialog closes.

Rotation keeps the record ID but uses a digest compare-and-swap update. Exactly
one concurrent rotation may succeed. The successful replacement immediately
invalidates the previous bearer because its digest no longer matches. Rotation
and revocation remain owner-scoped, and rotated tokens retain both
`refreshTokenDisabled` and `personAccessToken` claims.

### Schema evolution

The legacy `token` column remains present for one compatibility interval but is
mapped as a non-JSON legacy field. The forward migration adds digest and
fingerprint columns, hashes each non-empty legacy value in process, writes the
new fields and clears plaintext in one parameterized row update. A row with no
recoverable legacy value and no digest is revoked to fail closed.

The migration is idempotent and safe to resume after additive DDL has already
run. It does not promise to reconstruct plaintext on rollback. Old instances
must be drained before enabling hardened rotation semantics because the old
refresh implementation can issue an interactive JWT.

### OAuth callbacks

Provider access and refresh tokens are internal server values. The browser
submits callback code and state to the Admin API with POST after immediately
removing them from the visible callback URL.

- Login: the backend verifies the provider identity, creates the normal Admin
  session and audit record, and returns only the Admin login result.
- Binding: the backend writes the state-bound user's provider binding directly
  and returns completion metadata.
- Integration: the backend encrypts the provider access token in a short-lived
  server-side credential store and returns an opaque handle.

The callback response never contains provider access or refresh token fields.
The legacy GET callback remains an explicit `405 Method Not Allowed` route for
clients that have not moved to POST.

### Integration credential handles

Integration handles contain 256 random bits. Only a SHA-256 digest is used as a
cache key. The credential record is encrypted with AES-256-GCM using a
domain-separated key derived from the configured Admin authentication secret;
the cache key is authenticated as associated data.

Each record binds the provider, integration intent, user ID, interactive session
credential fingerprint, and expiry. Generator endpoints receive the handle in
`X-MSS-OAuth-Credential`, validate all bindings in constant time, and only then
recover the provider access token. The handle remains in React memory and is
deleted after a successful generation or by expiry. Generator URLs and request
bodies no longer accept provider tokens.

Redis is required for multi-replica use. The encrypted in-process fallback is
only for a single development process; a callback and generator request landing
on different instances without Redis fail closed.

## Consequences

- A database disclosure no longer recovers active PAT bearer values.
- PAT rotation and revocation have deterministic, testable invalidation
  semantics.
- OAuth provider credentials no longer enter browser storage, application URLs,
  generator bodies, or audit request JSON.
- Rotating the Admin authentication secret also invalidates outstanding
  integration handles, which is a safe failure mode.
- Users must save a newly created or rotated PAT before closing the dialog. A
  lost value must be rotated; support staff cannot recover it.
- The primary Admin session JWT remains in `localStorage` for this atomic change.
  Moving it to an HttpOnly cookie requires a separate CSRF, CORS, WebSocket, and
  end-to-end test migration.
- WebSocket URL authentication and repeatable display of application client
  secrets remain separately tracked P1 work; neither is silently treated as
  solved by this decision.

## Validation and rollout

Required evidence is defined in `.mss/features/admin-secret-lifecycle.yaml` and
includes focused security, concurrency, migration, frontend, and browser tests.
The executable rollout is:

1. back up the database and drain old Admin instances;
2. apply the additive PAT migration and verify the plaintext count is zero
   without printing values;
3. deploy backend and frontend from the same revision;
4. smoke-test PAT create, use, rotate, old-token rejection, and revoke;
5. smoke-test OAuth login, binding, and generator integration;
6. verify dynamic menus and runtime language switching are unchanged.

Rollback never restores raw PATs. Disable create and rotate, revert the
application revision if necessary, clear temporary OAuth credentials, and
reissue affected PATs after returning to the hardened version.
