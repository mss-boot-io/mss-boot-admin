# Resolve Admin themes from immutable defaults and sparse scoped overrides

- Status: Accepted
- Date: 2026-08-07
- Owners: Admin Platform, Frontend
- Implementation status: P1/P2 core and the P3 revision/synchronization implementation are present on the current branch; external MySQL/PostgreSQL verification and the complete browser E2E matrix remain release gates, so the capability stays `planned`
- Feature contract: `.mss/features/admin-theme-settings-precedence.yaml`

## Context

The baseline Admin exposed theme settings in Application Settings and Personal Settings and also rendered a Pro
`SettingDrawer`. The application and personal pages reused one component, but that component always read and wrote
the application configuration group. Bootstrap then merged user, application, and code values with truthiness
operators while mutating the imported default object.

That baseline did not define reliable ownership or inheritance. In particular, it allowed these failure modes:

- a normal account could see a personal theme form that attempted an application mutation;
- a root account could unintentionally change the global theme from Personal Settings;
- `false` could not reliably override a lower layer, and legacy user booleans were returned as strings;
- inherited values could not be removed because there was no per-field or whole-scope reset contract;
- login, logout, account switching, saving, other tabs, and the first paint did not share one resolver;
- tests and documentation did not prove the intended precedence or permission boundaries.

P1/P2 corrected those ownership and resolution defects. This decision also defines the P3 concurrency and runtime
contract so backend resources, frontend state, caches, tabs, tests, and upgrade behavior cannot diverge.

## Current implementation boundary

The current branch contains the implementation, but implementation presence is not the same as release evidence:

| Area | Present on this branch | Still unverified or incomplete |
| --- | --- | --- |
| P1/P2 scope and precedence | Typed seven-field resolver, sparse application/user adapters, inherited-source UI, reset, RBAC/self ownership, and identity cleanup | Complete production browser matrix |
| P3 backend | Composite-key `ConfigRevision`, canonical revisioned resources, ETag/`If-Match`, 412 conflict responses, versioned public-profile cache, and structured theme audit metadata | External MySQL/PostgreSQL fresh/upgrade/concurrency matrix and production Redis fault exercise |
| P3 runtime | `BroadcastChannel` plus `storage` fallback, stale-event rejection, dirty-draft conflict state, visibility reconciliation, and 24-hour identity-bound snapshots | Real two-user/multi-page E2E and first-paint browser trace |
| Release status | Local unit/integration evidence can be accumulated during implementation | Capability remains `planned` until every required acceptance gate passes |

## Decision

### Resolve every field through three layers

For each supported field:

```text
effective = immutable code default
          + sparse application override
          + sparse current-user override
```

Overlay is field-specific and based on the presence of a valid value, not truthiness. A valid user value wins over
an application value; a valid application value wins over the code default. `false` is an explicit value.

The resolver returns both the complete effective theme and a source for every field:

- `user` when a valid current-user override exists;
- `application` when no user override exists and a valid application override exists;
- `code` otherwise.

The imported code defaults are immutable. Resolution always creates a new value and cannot modify the module-level
default object.

### Define the runtime field set

| Field | Code default | Accepted values |
| --- | --- | --- |
| `navTheme` | `realDark` | `light`, `realDark` |
| `colorPrimary` | `#1890ff` | normalized six-digit hexadecimal color |
| `layout` | `mix` | `side`, `top`, `mix` |
| `contentWidth` | `Fluid` | `Fluid`, `Fixed` |
| `fixedHeader` | `false` | boolean |
| `fixSiderbar` | `true` | boolean |
| `colorWeak` | `false` | boolean |

The legacy `fixSiderbar` spelling remains the persistence and ProLayout compatibility key for this delivery.
`pwa` is a deployment and service-worker concern, and `splitMenus` is not currently active; neither belongs to the
runtime application or personal theme contract.

Legacy string `"true"` and `"false"` values are normalized before form display and resolution. Empty strings,
unknown keys, malformed colors, and unsupported enums are not inheritance markers: they are invalid and fall back
to the next valid layer with diagnostics.

### Reuse the existing group resources

All paths below are relative to the existing `/admin/api` group.

| Scope | Read sparse overrides | Sparse update | Reset all |
| --- | --- | --- | --- |
| Application | `GET /app-configs/theme` | `PUT /app-configs/theme` | `DELETE /app-configs/theme` |
| Current user | `GET /user-configs/theme` | `PUT /user-configs/theme` | `DELETE /user-configs/theme` |

PUT retains the existing group route but has explicit sparse-patch semantics for theme:

- omitted field: no mutation;
- valid non-null field: set or replace this scope's override;
- `null`: delete this field's override and restore inheritance;
- `false`: store an explicit false;
- empty string: reject the complete request.

Canonical clients send `Accept: application/vnd.mss.theme.v1+json`. That representation returns only normalized sparse
overrides owned by the requested scope, not a flattened effective theme. The seven fields remain at the response top
level and a reserved `_meta` member contains `{v, scope, revision}`. New clients use its decimal-string revision and
the matching strong `ETag`; PUT, DELETE, and `412` return the same vendor `Content-Type` representation. An Accept
entry with `q=0` explicitly declines it and therefore stays on the legacy representation. `Vary: Accept` makes the
temporary content negotiation boundary explicit, and authoritative theme/profile reads use `Cache-Control: no-store`.

During the rolling window, a request without the canonical media type receives the preceding flat legacy
representation without `_meta`. Application legacy GET/PUT continues to expose and accept `pwa` so already deployed
frontend assets do not silently turn an existing service-worker setting off. User scope never accepts `pwa`, and a
canonical application request that supplies it is rejected. The new resolver ignores the temporary `pwa` extension.
This compatibility path is removed only in a separately reviewed release after preceding frontend assets are drained.
DELETE removes the seven runtime overrides in exactly that scope and does not turn `pwa` into a runtime theme field.
The public unauthenticated bootstrap remains `GET /app-configs/profile`; its allowlist temporarily retains legacy
application `pwa`, which the new runtime resolver ignores.

For example, a versioned application resource is shaped as follows:

```http
Accept: application/vnd.mss.theme.v1+json
Content-Type: application/vnd.mss.theme.v1+json
ETag: "theme-application-12"
Cache-Control: no-store
Vary: Accept
```

```json
{
  "navTheme": "realDark",
  "fixedHeader": false,
  "_meta": { "v": 1, "scope": "application", "revision": "12" }
}
```

The canonical overrides intentionally remain flat. Nesting them below an `overrides` member would add needless drift;
legacy compatibility is instead explicit through media-type negotiation.

### Keep one editor and inject scope

Application Settings and Personal Settings continue to reuse one editor. The editor requires an explicit
`scope="application"` or `scope="user"` and receives a scope-specific adapter.

The component renders inherited effective values but tracks explicit overrides separately. Displaying an inherited
value must not cause that value to be saved into the higher layer. Each field has a restore-inheritance action, and
the form has a reset-current-scope action.

The settings drawer must use the user adapter or be an explicitly temporary preview that can be saved into user
scope. It cannot silently introduce a fourth authority layer.

### Enforce permissions and ownership

- Application GET requires `config:read`.
- Application PUT and DELETE require `config:write`.
- User GET, PUT, and DELETE are `authenticated-self` operations.
- The backend derives the user ID from the verified identity. A client cannot select another user in any request
  field, path, query, cached identity, or header.
- Frontend permission states are user experience only; backend authorization remains mandatory.

Successful and failed saves and resets produce value-free structured audit metadata with actor, scope, sorted changed
keys, outcome, and the canonical revision when available. The raw theme body is omitted once this metadata is set,
and general sensitive fields remain redacted. Audit skip matching must use path-segment boundaries and must not
accidentally exclude `/user-configs/*`.

### Make a save one atomic operation

The backend validates all supplied keys before beginning writes. Sets and deletes execute in one explicit database
transaction. A validation or database failure leaves all overrides unchanged and emits no successful change event.

The successful response returns the canonical normalized sparse scope and a monotonic database revision. A new
client sends the strong resource ETag in `If-Match`; a stale precondition returns HTTP 412, the current ETag, and the
current canonical resource under `data.current`, and performs no write. A malformed, weak, wildcard, multi-value, or
wrong-scope precondition is rejected. Requests without `If-Match` remain temporarily accepted so an older frontend
can be rolled through the deployment without losing write access.

Theme revisions live in the additive `mss_boot_config_revisions` metadata table rather than in the seven value rows.
Its composite primary key is `(scope, owner_id, resource)`: application resources use an empty `owner_id`, while user
theme resources use the authenticated user ID. Application theme writes advance both `application/theme` and
`application/public-profile` revisions in the same transaction. The public profile cache is keyed by its database
revision, validates both profile and theme revisions in its envelope, and currently has a 15-minute TTL. Redis is a
disposable derived cache: read, write, or cleanup failures are logged but cannot turn an already committed mutation
into an HTTP failure, and an entry for an older database revision is never accepted.

Reset must account for GORM soft deletion and the unique indexes on configuration keys. A reset followed by setting
the same key must work on SQLite, MySQL, and PostgreSQL; implementation may use an exact hard delete or explicitly
restore `deleted_at`, but must not leave an invisible conflicting row.

Configuration identifiers that collide with reserved `theme` or public-profile keys under a database's
case-, accent-, Unicode-normalization-, or trailing-space-insensitive collation cannot enter the generic write path.
Existing single aliases are canonicalized by exact primary key inside the transaction; ambiguous candidates fail
closed. This guard is limited to reserved collisions and does not globally prohibit unrelated downstream custom keys.

### Recompute through identity and browser transitions

Anonymous and login pages resolve code plus application. Authentication adds the verified user's sparse layer
without requiring a full reload. Logout clears the personal layer and its identity-bound cache before resolving the
application theme. Switching from user A to user B cannot show or persist A's personal overrides.

Save and reset apply the returned canonical resource immediately in the current tab. Same-origin tabs converge through
schema-versioned `BroadcastChannel` events with a `storage` fallback. Events are time-bounded and deduplicated; older
revisions are ignored, while equal revisions with different payloads trigger an authoritative read. A dirty editor
keeps touched draft fields and enters an explicit conflict state instead of silently overwriting or retrying them.
Foreground visibility triggers a throttled authoritative reconciliation. A read failure falls back only for rendering;
it never writes the fallback as an override.

A versioned, theme-only last-known snapshot may be used before React renders to prevent a warm-load light/dark
flash. It expires within 24 hours and contains only the supported theme values and resource metadata. Personal
snapshots and cross-tab events bind to a random authentication-session identifier, never a client-provided user ID.
The snapshot is a performance hint, not a fourth source of truth, and is reconciled with authoritative revisions.
Snapshot persistence serializes the complete read/compare/write operation with a Web Lock scoped to the storage key.
If Web Locks are unavailable or denied, the client skips persistence instead of risking a stale cross-tab overwrite;
the authoritative runtime theme still works, but that browser does not receive the warm-start snapshot optimization.

## Compatibility and migration

The implementation reuses `mss_boot_app_configs` and `mss_boot_user_configs` and adds only
`mss_boot_config_revisions` for resource metadata. The migration is additive, forward-only, idempotent, and does not
rewrite the existing configuration schemas or rows.

- Preserve all existing rows and normalize supported legacy values at the read/write boundary.
- Preserve unknown theme rows for downgrade safety but exclude them from the effective runtime theme.
- Inventory invalid values before rollout without logging other application configuration.
- If a forward normalization migration becomes necessary, it must be additive or narrowly targeted, idempotent,
  and tested on fresh and upgraded SQLite, MySQL, and PostgreSQL databases.
- The new frontend negotiates `Accept: application/vnd.mss.theme.v1+json`. `Accept` is CORS-safelisted and is ignored by
  the preceding backend, so a newly loaded frontend can still parse its legacy response without adding a custom
  preflight header. The new backend accepts a temporarily missing `If-Match` and retains application-only legacy `pwa`
  behavior when the canonical media type is absent.
- Regenerate OpenAPI clients with the backend DTO change and keep the handwritten compatibility adapter tested until
  generation is part of the repository workflow.

### Rolling upgrade order

1. Back up the database and inventory invalid theme values without recording unrelated or sensitive configuration.
2. Apply the additive revision migration; do not remove either existing configuration table.
3. Deploy revision-aware backend instances and drain every old backend writer before relying on revision ordering,
   because an old instance does not advance `ConfigRevision`.
4. Deploy the revision-aware frontend. It asks for the canonical media type, interoperates with an old success body by
   performing another GET, and sends `If-Match` only after it has loaded a versioned resource.
5. Monitor migration errors, 412 rates, legacy-media and missing-precondition requests, Redis warnings, and audit
   outcomes. Remove both legacy application `pwa` projection and missing-`If-Match` allowance only in a later
   compatibility release after old frontend assets are drained.
6. Promote the capability from `planned` only after SQLite, MySQL, PostgreSQL, and the complete browser matrix pass.

## Rollback

Before rollout, back up the database. To roll back, first stop theme mutations so no writer can bypass the chosen
revision contract, then deploy the preceding frontend and backend together. Compatible remaining rows may stay in
place, and the additive revision table may remain unused until a separately reviewed cleanup release.

An explicit `null` or DELETE reset intentionally removes overrides. Rolling back code cannot recreate those user
choices; restore only requested overrides from the backup or redacted audit evidence. Delete revision-keyed public
profile cache entries and clear the new browser theme snapshot, synchronization event, and authentication-session
keys when reverting runtime synchronization. Redis cleanup failure does not require database rollback because cached
entries are revision-bound and expire.

## Consequences

### Positive

- Application and personal controls retain one UI implementation without sharing the wrong data source.
- Every field has deterministic precedence and visible provenance.
- A false boolean, per-field reset, whole-scope reset, login, logout, and cross-tab update have testable semantics.
- Existing storage and public-profile security boundaries remain reusable.

### Cost

- Backend validation, transactions, reset routes, permission policies, audit behavior, and generated clients must
  change together.
- Frontend bootstrap must move from mutable initial settings to one runtime resolver.
- Production acceptance needs real roles, two users, two tabs, both themes, mobile widths, and supported databases.

## Completion definition

The decision is implemented only when every required acceptance item in
`.mss/features/admin-theme-settings-precedence.yaml` has evidence, local and production frontend builds agree,
`go run ./cmd/mss verify --changed` succeeds, and the supported-database upgrade matrix passes.
