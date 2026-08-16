# Add recent re-authentication for account credential self-service

- Status: Accepted
- Date: 2026-08-16
- Owners: Admin Platform, Security, Frontend
- Feature contract: `.mss/features/admin-antd-v6-application.yaml`

## Context

Changing a password or disconnecting an OAuth identity changes how an account can be
entered. An ordinary authenticated browser session is not sufficient proof for these
operations: a stolen but still-valid session could otherwise replace the password or
remove a recovery path. The legacy password-reset and OAuth-unbinding endpoints did not
enforce a recent proof of identity, and the unbinding path could remove an account's
last verified login method.

The platform cannot recover or display a user's password. Local passwords are owned by
the user and persisted only as a salted, one-way verifier. OAuth provider credentials
and authorization codes must likewise remain server-side and must not become a second
password or enter browser storage.

## Decision

Sensitive account self-service uses a server-recorded recent-authentication property on
the durable browser session. A successful interactive login starts a five-minute recent
authentication window. Older sessions must complete one of these step-up methods:

- verify the current local password when local-password login is enabled; or
- complete a fresh OAuth authorization with a provider identity already bound to the
  same account.

The proof is bound to the current server session identifier. It is not represented by a
browser-readable token and cannot be copied to another session. Personal access tokens,
legacy JWTs without a server session, and anonymous requests cannot create or consume
the proof. Password attempts are rate limited in the durable session row; five failed
attempts lock step-up for five minutes. Successful proof clears the failure state.

The account-security read endpoint returns only local-password capability and bounded
proof/lock expiry metadata. Provider bindings remain on the existing metadata-only
OAuth connection endpoint. Neither endpoint returns a password, hash, salt, provider
token, or secret, and both are marked `Cache-Control: no-store`.

A password change requires recent authentication, validates the new password, rotates
the salt and one-way scrypt verifier, enables local-password login, and atomically
revokes all browser sessions and active personal access tokens. The response clears the
browser cookies and the Ant Design 6 client removes its in-memory server-state cache,
then redirects to login. Signing the initiating browser out is deliberate: no old
credential remains usable after a password boundary changes.

OAuth re-authentication uses a dedicated one-time `reauthentication` intent. Its state
keeps the existing provider, browser nonce, response transport, user identifier, and
credential fingerprint bindings. After provider exchange, the backend derives the
canonical provider identity and succeeds only if that exact active identity is already
bound to the current user; it neither creates nor moves a binding.

OAuth disconnect requires an unexpired recent-authentication window. A database
transaction locks the user and active bindings, confirms ownership, and refuses the
delete unless another verified login method remains: either an enabled local password
or another active OAuth binding. Concurrent disconnect attempts therefore cannot both
remove the final method. The compatibility unbinding endpoint delegates to the same
service and cannot bypass the guard.

## API and compatibility

The V6 browser client uses these additive self-service routes:

- `GET /admin/api/user/security`
- `POST /admin/api/user/security/reauthenticate`
- `PUT /admin/api/user/security/password`
- `DELETE /admin/api/user/oauth2/:provider`

OAuth authorization and callback routes accept the additive `reauthentication` intent.
Existing anonymous recovery remains an independent verified-email flow. The old
authenticated shortcut on `/user/reset-password` is removed; authenticated clients use
the recent-authentication password route. The legacy OAuth unbinding route remains only
as a compatibility facade with the same security checks.

## Deployment and rollback

The session columns are additive and nullable/defaulted so existing active sessions
remain valid for ordinary requests but must step up before a sensitive credential
change. Deploy the backend migration and API before exposing V6 controls. Provider
redirect URIs already used by V6 callbacks remain unchanged.

Rolling back the V6 UI hides the new controls. The additive session columns and routes
may remain. Do not roll back to the unsafe authenticated password shortcut or unguarded
OAuth delete. A backend rollback that cannot preserve these guards must disable the
credential self-service routes until a repaired version is deployed.

## Consequences

Credential changes require one extra proof when login is no longer recent, and password
changes intentionally sign the user out everywhere. OAuth-only users can still manage
their connections without inventing a local password. Security state is observable and
testable in one durable session source of truth, while the browser never handles a
re-authentication bearer secret.
