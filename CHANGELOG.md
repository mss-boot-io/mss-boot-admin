# Changelog

All notable, verifiable changes to the consolidated `mss-boot-admin` foundation
are documented here. The project uses semantic versioning and component-scoped
tag namespaces.

## [Unreleased]

Target: **v1.1.0 development train**. The former v1.0.x and prerelease slices are
internal, untagged checkpoints. Complete qualification starts only after feature
freeze; no public v1.0.x or v1.1.0 prerelease is planned by the current policy.

### Changed

- Switched the next train to development-first v1.1.0: internal waves remain
  untagged, publication is disabled by checked-in policy, acceptance evidence is
  phase-scoped, and complete release qualification starts only after feature freeze.
- Reconciled the root and nested-framework changelogs, release FeatureSpec, and
  release documentation with the public `v1.0.0` evidence.
- Reclassified the aggregate cache/lock/queue adapter capability from stable to
  legacy and added provider-specific evidence guidance.
- Reduced the Admin Storage AppConfig and UI surface to the upload-admission
  `storage:maxSize` and `storage:allowedTypes` fields. Provider selection and
  SecretRef-backed credentials are reserved for the immutable startup profile.
- Replaced the object-storage provider map with an exact Local-or-S3 startup
  configuration, immutable normalized profiles, explicit credential modes, and
  typed environment SecretRefs. This is an intentional breaking configuration
  contract and does not promote either provider.
- Kept the framework `AdapterQueue` surface source-compatible and added the
  additive `ManagedAdapterQueue` lifecycle contract. Kafka configuration and
  registration now use caller contexts and return errors; Admin owns the managed
  adapter and registers its blocking `Start` with the server `Runnable` manager.
- Made migration registration lossless and fail-fast: full decimal identifiers are
  ordered without integer truncation, duplicates fail before any schema access,
  the Admin migrator propagates context/errors, and explicit v1.0.0 aliases prevent
  historical 10/13-digit marker rows from rerunning. The generated migration
  template now registers the complete filename identifier.

### Fixed

- Moved the legacy Kafka adapter's Sarama offset mark after JSON decoding and
  synchronous handler success, propagated the consumer-session context into the
  message, and stopped canceled or closed consumer loops without marking unfinished
  work. D1 additionally removes registration/configuration Exit/Fatal paths, owns
  one producer and one consumer group per unique topic/group, observes consumer
  errors, rejects new work during close, and provides cancellable start plus
  idempotent, deadline-bounded, retryable close. The adapter remains
  Legacy/Blocked: hermetic evidence does not prove broker commit, manual commit,
  retry/backoff, dead-letter, rebalance, idempotency, outage, or real-broker behavior.
- Removed the Admin ghost storage initializer and per-upload S3 constructor.
  One composition-root owner now installs a leased framework Handle and pinned
  filesystem before registering Application delivery, then closes them with bounded,
  idempotent drain semantics. S3 config
  bootstrap owns a separate client and closes response bodies on every read path. Only a
  missing stage object is optional; read/malformed-overlay failures fail closed, and HTTP
  requires the explicit `s3_tls_allow_insecure_http=true` opt-in.

### Security

- Bounded both authenticated upload routes before multipart parsing with a
  100 MiB configuration ceiling, a fixed 64 KiB multipart envelope budget,
  max-plus-one stream validation, stable 413/422 responses, and deterministic
  multipart spill cleanup. Local and S3 keys are now opaque UUIDs; Local writes
  use an `os.Root`-confined create-only path and remove canceled or partial
  files. Local and S3 remain Legacy/Blocked until the D4 object metadata,
  provider conformance, authorization, and Delivery gates close.
- Added the provisional D0 Redis challenge state machine for email login,
  registration, and password recovery. It uses cryptographic fixed-width codes,
  purpose/subject HMAC keys, versioned peppered verifiers, delivery
  Begin/Commit/Abort CAS, pending leases, cooldown and rolling quotas, bounded
  caller/global issuance, bounded context-aware SMTP delivery, bounded attempts,
  and exactly-once successful consumption.
- Removed the Admin application's fallback to the legacy email-only
  verification-code adapter, made Redis/secret failure explicit, made issuance
  responses account-independent, enforced email/register switches at issuance
  and consumption, added fresh Redis-plus-SMTP readiness and an explicit
  trusted-proxy policy, removed codes from email headers, and removed the
  misleading phone-code login tab until the separately planned phone challenge
  capability exists.
- Canonicalized bounded ASCII email identities consistently, failed closed on
  ambiguous lookup, and disabled self-service email mutation until the planned
  D2 three-database canonical uniqueness migration is complete.
- Stopped projecting historical storage provider or credential AppConfig rows and
  reject every removed storage key with a stable 422 before any mutation. Secret
  read/write capabilities cannot restore this retired configuration surface.
- Made absent, invalid, unresolved, closing, or unavailable object storage return
  a fixed 503 from both upload routes with zero implicit Local write. Local is
  installed only for an explicit development static mapping; S3 upload stops before
  Put until the D4 ObjectStore and Delivery contracts pass.

### Documentation

- Added machine-validated contracts for the internal safety wave, Storage Runtime v2,
  the v1.1 Generator/Blueprint golden slice, and the development-first v1.1.0 release
  train, plus the owned-resource architecture decision and provider maturity matrix.
- Added the Challenge checkpoint operator note covering SecretRef setup, failure
  semantics, focused evidence, rollback, and the remaining real-Cluster gate.
- Added the Kafka Mark-after-success checkpoint note and corrected the queue
  tutorial so legacy Kafka/NSQ adapters are not presented as production-ready.
- Added the Upload admission checkpoint note with byte-unit configuration,
  handler-level admission plus route-registration evidence, rollback guidance,
  and the remaining provider and delivery blockers.
- Added the D1 Object Provider/Owner checkpoint with exact profile, owner,
  AppConfig, 503, development Local delivery, and shutdown evidence. S3 Put,
  Delivery, and pinned RustFS conformance remain deferred to D4.
- Extended the Kafka checkpoint with exact managed-interface, owner, configuration,
  Casbin registration, Admin Runnable, error-observation, and bounded-close evidence.
  D1 is complete and the next development wave is D2 contract substrate; Kafka
  remains Legacy/Blocked pending its dedicated real-broker and delivery suites.

## [v1.0.0] - 2026-08-09

Status: **published / stable**. Root `v1.0.0` and the prerequisite
`mss-boot/v1.0.0` Release both resolve to
`ee800262c035c5f4242aca1841d077554481d2c4`. The public artifacts and exact-main
approval are recorded in GitHub issue `#471`. The optional standalone
`web/antd/v1.0.0` tag was not published.

This is the consolidated foundation's first stable 1.0 boundary. It superseded
the unpublished v0.8.0 release candidate; no v0.8.0 archive, checksum, image
digest, workflow run, or smoke result was accepted as release evidence for
v1.0.0. Every required artifact and proof was regenerated from the exact
v1.0.0 commit.

Release, upgrade, rollback, and compatibility contracts are maintained under
[`docs/docs/releases/`](docs/docs/releases/).

### Added

- Consolidated the Admin application, nested `mss-boot` framework module,
  React/Ant Design frontend, documentation, machine-readable `.mss` contracts,
  deterministic generators, Skills, MCP adapter, and evaluations into one
  foundation repository.
- Added the Agent-facing `mss` CLI for context, environment setup, verification,
  specification validation, deterministic module/application generation, and
  three-way downstream upgrade planning.
- Added backend-owned CPU and memory sampling with bounded recent history. The
  Admin task server runs monitoring and session cleanup as immutable system
  jobs, separate from user-managed Task records.
- Added database-backed configuration revisions, strong ETags, conditional
  writes, revision-bound public profile caches, and owner-isolated personal
  configuration snapshots.
- Added layered theme settings with the precedence `code defaults < application
  settings < personal settings`. This capability remains **preview** until its
  external MySQL/PostgreSQL and browser acceptance gates are complete.
- Added positive and negative authorization coverage for Admin routes, static
  frontend routes, dynamic menus, application secrets, uploads, and role
  authorization.

### Changed

- Split the former root Admin Go module into:
  - `github.com/mss-boot-io/mss-boot-admin/admin` for the deployable reference
    application;
  - `github.com/mss-boot-io/mss-boot-admin/mss-boot` for the reusable framework;
  - `github.com/mss-boot-io/mss-boot-admin` for Agent/foundation tooling.
- The Admin module requires `github.com/mss-boot-io/mss-boot-admin/mss-boot
  v1.0.0`. The nested module must therefore be published and externally
  resolved before the root `v1.0.0` tag is created.
- Authentication now resolves the current user, role, enabled state, and root
  state from authoritative storage. Role/root snapshots embedded in older JWTs
  are not trusted.
- The historical root behavior remains: an enabled root identity bypasses
  ordinary Casbin policy checks. Root/default roles and root users are protected
  from destructive generic CRUD operations.
- Role authorization is a versioned whole-resource update. Reads return a
  revision and strong ETag; bundled clients send `If-Match`; stale writes return
  `412 Precondition Failed` without partial persistence.
- Personal access tokens are owner-scoped, stored only as versioned digests,
  shown in raw form once, and atomically rotated or revoked. PATs cannot invoke
  interactive account-security operations.
- OAuth authorization and callback state are server generated, single use, and
  bound to provider, intent, browser, and—when applicable—user/session identity.
  Provider access and refresh tokens are not serialized to browser storage or
  persisted in the user binding model.
- OAuth-created or historically OAuth-bound accounts are fail-closed for local
  password login until an explicit password reset restores local credentials.
- Application and personal configuration reads use the database as authority.
  Cache keys include all query dimensions and database revisions; Redis failure
  degrades to authoritative database reads rather than stale acceptance.
- System configuration remains an opaque, root-only resource. Application
  credential fields require separate `app-config:secret-read` and
  `app-config:secret-write` capabilities for non-root users.
- Generic storage upload now requires explicit `storage:upload` authorization
  for non-root users and PATs.
- Dynamic menus remain supported and are refreshed with current identity and
  permission state; frontend visibility continues to be advisory to backend
  authorization.

### Breaking API and behavior changes

- `GET /admin/api/user-auth-token/generate` no longer creates a token and
  returns `405 Method Not Allowed`. Use `POST /admin/api/user-auth-tokens`.
- JWT refresh is state-changing and uses `POST
  /admin/api/user/refresh-token`; legacy GET refresh is not supported.
- OAuth callback completion uses `POST /admin/api/user/:provider/callback`
  with `code` and `state` in the JSON body. The legacy GET callback and browser
  token binding endpoint return `405`.
- Retired `/admin/api/template/*`, runtime model/field, and virtual CRUD routes
  are no longer registered.
- Older PATs without the minimum signed identity and persisted digest contract
  may stop authenticating and must be reissued.
- Public registration and first-time OAuth account creation are disabled unless
  `security:registerEnabled` is explicitly enabled and exactly one enabled,
  non-root default role exists.
- The least-privilege default-role migration intentionally stops on ambiguous
  historical root/default-role data instead of silently retaining a privilege
  escalation path.

### Removed

- Removed Admin runtime dynamic models, model fields, virtual CRUD, browser
  template/code generation, their routes, menu entries, policies, and reusable
  runtime framework packages.
- Preserved inert historical metadata and user-created data tables during
  automatic upgrade. Their removal or export is an explicit operator action,
  not part of the release migration.
- Removed OAuth `integration` intent and the short-lived provider credential
  handle formerly used by the browser generator flow.

### Security

- Production startup rejects the public development authentication secret and
  requires a unique random `auth.key` of at least 32 bytes.
- User/role disablement and role changes are re-evaluated from authoritative
  storage instead of trusting stale claims. Session termination and password
  changes revoke active server-side sessions, while PAT revocation and rotation
  invalidate their bearer immediately. Production deployments must enable
  `auth.sessionEnabled`; without it, an already-issued browser JWT can remain
  valid until expiry after a password change.
- Historical built-in OAuth credentials are sanitized only when their exact
  fingerprint matches. Provider-side rotation/revocation and repository secret
  scanning remain mandatory release gates.
- Audit logging redacts passwords, PATs, OAuth credentials, theme values,
  multipart file contents, and case-insensitive token query parameters.
- Audit and alert-history resources are read-only; generic mutation routes are
  not exposed.

### Migration

- A database backup, restore rehearsal, configuration backup, active-writer
  drain, and the preflight checks in the
  [v1.0.0 upgrade guide](docs/docs/releases/v1-0-0-upgrade.md) are required.
- Run the Admin migration command before starting v1.0.0 application writers.
  The release adds or advances session/menu metadata, PAT digests, OAuth local
  password state and identity keys, permission metadata, retired-tool cleanup,
  configuration revisions, and least-privilege role data.
- Migrations are forward and idempotent, but not all effects are reversible.
  In particular, cleared PAT plaintext and sanitized credentials are not
  reconstructed by a code rollback.
- Use forward-fix by default. Restore the complete pre-upgrade database and
  configuration snapshot only when a proven compatible previous runtime is
  required; never partially edit migration version rows.

### Compatibility and release order

1. Publish `mss-boot/v1.0.0` from the reviewed release commit.
2. From outside this repository with `GOWORK=off`, resolve and test
   `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.0.0`.
3. Re-run the root release gates on the exact commit.
4. Publish root `v1.0.0`; publish a standalone `web/antd/v1.0.0` only after its
   independent production/local artifact contract passes.

`planned`, `preview`, a release branch, or an `Unreleased` changelog entry must
never be presented as a stable tag.

## [v0.7.0] - 2026-06-05

`v0.7.0` is the preceding root release baseline. Historical details and
artifacts remain available from the GitHub Releases page. Older untagged
development snapshots formerly described as `v1.0.0` were not consolidated
repository releases and provide no tag, artifact, or validation evidence for
the stable v1.0.0 release documented above.
