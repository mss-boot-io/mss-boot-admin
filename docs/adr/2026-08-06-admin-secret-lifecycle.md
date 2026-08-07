# Admin secret lifecycle and OAuth credential confinement

- Status: Accepted
- Date: 2026-08-06
- Baseline: `main` at `60aff0feeea7706efc3cdc7c4b309559f4026f2c`
- Specification: `.mss/features/admin-secret-lifecycle.yaml`
- Scope: Admin PAT persistence and rotation, OAuth callback completion, and short-lived generator credentials
- Superseded in part by: `2026-08-06-remove-runtime-developer-tools-and-sample-monitoring.md`

> Supersession note: the PAT, OAuth login/binding, provider-identity,
> local-password, logging, and Admin-session decisions below remain active. The
> integration-credential-handle and browser Generator sections, their
> consequences, and their rollout steps are retained only as historical context;
> the later runtime-tools-removal decision deleted that product surface and
> prohibits restoring its routes or OAuth `integration` intent.

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

OAuth-created accounts also reused provider token material as a local password,
and provider identities had no database-enforced single-owner key. A concurrent
first login or binding could therefore create ambiguous ownership. The public
development `auth.key` was also accepted in production, even though it protects
Admin JWTs and the encrypted integration-credential store.

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

Before an Admin JWT is marshalled, provider access and refresh tokens are
cleared from the verifier object. Provider exchange failures are logged only as
redacted categories, never as raw provider responses or hook errors that may
contain credentials. New OAuth-created accounts receive a random internal
password and have local password authentication disabled.

Every active provider binding has a nullable, normalized identity key made from
the provider name and its exact, trimmed opaque identifier. The database unique
index is the final ownership boundary; MySQL uses binary collation so opaque IDs
that differ only by case have the same semantics as PostgreSQL and SQLite.
Historical active duplicates make the forward migration fail before a unique
index is installed. Soft-deleted historical rows keep a null key and therefore
do not reserve an identity indefinitely.

### Integration credential handles

Integration handles contain 256 random bits. Only a SHA-256 digest is used as a
cache key. The credential record is encrypted with AES-256-GCM using a
domain-separated key derived from the configured Admin authentication secret;
the cache key is authenticated as associated data.

Each record binds the provider, integration intent, user ID, interactive session
credential fingerprint, and expiry. Generator endpoints receive the handle in
`X-MSS-OAuth-Credential`, validate all bindings in constant time, and only then
recover the provider access token. Read-only repository, branch, path, and
parameter discovery may access a public canonical GitHub repository without a
handle; if a handle is supplied, an invalid handle still fails closed.

Generation always requires a handle. After pure source, destination, and path
validation succeeds, the store atomically claims and consumes it before clone,
render, or push. From that point the operation is one-shot: success and failure
both require fresh authorization. A malformed request rejected before the claim
has no side effect on the server credential, while the frontend conservatively
clears its in-memory handle after any sent Generate request. Generator URLs and
request bodies no longer accept provider tokens.

Source and destination repositories must use canonical `https://github.com`
owner/repository URLs. Userinfo, custom ports, query strings, fragments, encoded
or extra path segments, recursive submodules, symlinks, special files, rendered
path traversal, and writes outside a unique confined workspace are rejected.

Redis is required for multi-replica use. The encrypted in-process fallback is
only for a single development process; a callback and generator request landing
on different instances without Redis fail closed.

### Local passwords, sessions, and authentication key

The local-password migration deliberately fails closed for every account with
OAuth history, including accounts that were local first and later bound, were
subsequently unbound, or are now soft-deleted. Historical data cannot reliably
prove whether the stored hash originated from a provider token. OAuth login
continues to work. An explicit password reset or administrator-assisted recovery
writes a new hash and salt and clears `local_password_disabled`.

Starting in production requires a unique, non-default `auth.key` of at least 32
bytes. Rotating it intentionally invalidates Admin JWTs and outstanding
integration handles. Before a new OAuth login, the frontend clears stale
non-persistent bearer state and asks the backend to expire the HttpOnly auth
cookie; a failed or stale previous session therefore cannot silently become the
principal for the callback.

The existing `query: token` lookup remains temporarily compatible for the
separately tracked WebSocket flow. API and embedded-UI access/recovery logging
use a redacted copy of the request target, replacing every case-insensitive
`token` query value without mutating `Request.URL.RawQuery` seen by authentication
or handlers.

## Consequences

- A database disclosure no longer recovers active PAT bearer values.
- PAT rotation and revocation have deterministic, testable invalidation
  semantics.
- OAuth provider credentials no longer enter browser storage, application URLs,
  generator bodies, Admin JWT claims, local passwords, provider-error logs, or
  audit request JSON.
- Query-token compatibility no longer writes raw JWT or PAT values to Gin access
  or recovery logs.
- Active provider identities have one database-enforced owner, including during
  concurrent first-login and binding requests.
- A generation that fails after successfully claiming its one-time credential
  consumes it. This is an intentional security/UX tradeoff; the UI must request
  authorization again.
- Rotating the Admin authentication secret also invalidates outstanding
  integration handles, which is a safe failure mode.
- Users must save a newly created or rotated PAT before closing the dialog. A
  lost value must be rotated; support staff cannot recover it.
- Existing password, email, mobile, registration, and refresh sessions retain
  their `localStorage` compatibility behavior in this atomic change. An Admin
  JWT returned by OAuth login is held only in current-document memory; reload
  or tab closure therefore requires authentication again. Moving every session
  flow to an HttpOnly cookie requires a separate CSRF, CORS, WebSocket, and
  end-to-end test migration.
- The document-scoped OAuth session is intentionally not passed to the legacy
  WebSocket `token` query parameter. HTTP notification polling remains
  available; OAuth WebSocket support requires a separate one-time-ticket or
  post-connect authentication design.
- WebSocket URL authentication and repeatable display of application client
  secrets remain separately tracked P1 work; neither is silently treated as
  solved by this decision.

## Validation and rollout

Required evidence is defined in `.mss/features/admin-secret-lifecycle.yaml` and
includes focused security, concurrency, migration, frontend, and browser tests.
The executable rollout is:

1. back up the database; inventory ever-linked accounts and active duplicate
   GitHub/Lark identities without printing provider identifiers or secrets;
2. verify password-reset or administrator-assisted recovery and resolve every
   duplicate provider owner before migration;
3. drain all old Admin instances and configure a unique random production
   `auth.key` of at least 32 bytes plus shared Redis for multi-replica use;
4. apply the PAT digest, local-password-disable, and OAuth identity-key
   migrations, then verify zero plaintext PATs and zero active duplicate
   identities without printing their values;
5. deploy backend and frontend from the same revision;
6. smoke-test PAT create, use, rotate, old-token rejection, and revoke;
7. smoke-test OAuth login, binding, integration authorization, public read-only
   generator discovery, successful generation, and failed-attempt reauthorization;
8. verify stale-session cleanup, dynamic menus, and runtime language switching.

Rollback never restores raw PATs. Disable create and rotate, revert the
application revision if necessary, disable OAuth binding, clear temporary OAuth
credentials, and reissue affected PATs after returning to the hardened version.
The additive `local_password_disabled` and `identity_key` columns may remain.
Affected accounts recover only through OAuth or an explicit password reset; do
not repopulate legacy PAT or provider-token material.
