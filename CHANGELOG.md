# Changelog

All notable, verifiable changes to the consolidated `mss-boot-admin` foundation
are documented here. The project uses semantic versioning and component-scoped
tag namespaces.

## [Unreleased]

Target: **v1.2.2** for the root foundation, `mss-boot/v1.2.2` for the reusable
Framework, and `web/antd-v6/v1.2.2` for the sole Admin frontend. Publication
requires one exact clean commit already merged into `origin/main`; local or
topic-branch evidence is preliminary only.

The immutable v1.2.1 Framework and frontend releases completed, and the root tag
and runtime image exist, but the root Release workflow retained a second raw V6
directory upload and stopped on a colon-bearing route filename before assembling
or publishing platform archives. v1.2.2 repairs the complete product boundary;
no v1.2.0 or v1.2.1 public ref is moved, deleted, reused, or supplemented.

### Changed

- Normalize Umi's literal dynamic-route HTML placeholders out of release output,
  fail closed on unexpected placeholder content, and set deterministic static
  directory/file modes to 755/644 so restrictive build umasks cannot cause Nginx 403.
- Added one cross-platform path validator for directories, ZIPs, and TARs. It
  rejects forbidden/control characters, traversal, trailing spaces or periods,
  Windows device names, case-insensitive collisions, unsafe links, and overlong
  components before artifact or Release publication.
- Made both frontend workflows transport only a checksummed `dist-v6.tar.gz`,
  and made root assembly verify/extract it before validating all six final ZIPs.
  Workflow tests inventory every remaining raw directory upload and require a
  preceding portability guard.
- Extended production delivery smoke to a concrete dynamic deep link after the
  placeholder removal, in addition to hashed-asset, cache, and missing-file checks.

## [v1.2.1] - 2026-08-19

Status: **component-partial / immutable**. Framework and the sole V6 frontend were
published from `80d2d20f1b44105e18706cfa0deb7f8512966f92`. The root tag and runtime
image were created, but root package assembly and GitHub Release publication did
not complete because the root workflow still uploaded raw V6 output. v1.2.2 is
the only active forward-repair target.

### Changed

- Made V6 publication upload only the portable `dist-v6.tar.gz`, build identity,
  and checksum manifest instead of traversing the raw `dist` tree. A workflow
  contract test prevents the non-portable directory upload from returning.
- Made the V6 code-layer theme default dark while retaining application and
  personal overrides with their existing precedence.
- Made `web/antd-v6` the sole Admin browser application and removed the retired
  frontend source, generator projection, dependency automation, CI/release/deploy
  paths, active documentation, and rollback image. Root distribution and rollback
  now use only qualified V6 frontend/backend pairs.
- Made `antd-v6` the only module-generation target. The Supplier golden produces
  V6 routes, locale catalogs, strict response contracts, React Query CRUD, and an
  HttpOnly/CSRF-aware Playwright flow; CI checks the one generated projection for drift.
- Removed token-returning browser login/refresh, bearer OAuth callback mode,
  query-token WebSocket authentication, the historical `jwt` cookie, unversioned
  theme projections, and missing-revision mutation fallbacks. Standard REST Bearer
  and PAT authentication remain supported for documented non-browser automation.
- Made the V6 browser session and durable server-side session check mandatory.
  The backend no longer exposes switches that can restore stateless browser JWT
  behavior; production still configures Secure/SameSite cookies, trusted origins,
  strong keys, shared session state, and one-time WebSocket ticket lifetime.
- Added forward-only Language, Option, account-reauthentication, example-Supplier
  retirement, and retired-V5 configuration migrations for fresh and upgraded
  V6 deployments.
- Made the one-shot `STAGE=local go run . server -a` route synchronization an
  explicit setup and upgrade contract. A healthy process with an empty API registry
  is not ready for menu API binding.
- Preserved the source-compatible Framework hardening in the synchronized v1.2.1
  forward repair for transactional
  delete controls, fixed public write-error mapping, constant-time password
  verification, and non-reversible SecretRef fingerprints.
- Added a repository-wide LF checkout contract and explicit pnpm pins for the
  pnpm 10.34.5 V6 application and pnpm 9.15.9 documentation site so WSL release
  checks do not depend on a Windows Git or global package-manager setting.
- Made feature-freeze release readiness install the repository-pinned Playwright
  Chromium and operating-system dependencies before executing V6 browser evidence,
  so clean GitHub runners no longer fail before E2E execution begins.

## [v1.1.0] - 2026-08-11

Status: **published / historical**. This section is retained as immutable release
history; all active release preparation now targets v1.2.2.

### Changed

- Enabled the protected v1.1.0 publication path after the scoped readiness runner, exact-run
  attestation, required-reviewer `release` environment, and immutable release-tag rulesets were
  installed and verified. Publication still requires exact-SHA pre-framework and pre-root authority.
- Added the D2 canonical-email development checkpoint for the bundled Admin:
  a full-ID forward migration preflights existing active identities without
  disclosure, performs compare-and-swap canonical backfill, and installs an
  active/non-empty unique identity using SQLite/PostgreSQL partial expression
  indexes or a MySQL nullable stored `VARBINARY` key. Real MySQL/PostgreSQL
  integration tests exist, but must be rerun with both DSNs and zero skip from
  the selected feature-freeze SHA; current development runs are not release evidence.
  Migration completion and Admin startup now share an exact redacted schema verifier,
  and the server mounts business routes only after that readiness gate passes.
- Added the D2 downstream-snapshot identity consumer checkpoint at `151a91c`:
  CLI upgrade status, MCP, and doctor now read one strict SnapshotStatus carrying the
  independent Foundation, Blueprint, generator, and downstream identities plus atomic
  lock/manifest digests. The source checkout is recognized only by its exact legacy
  development sentinel; malformed, orphaned, or source-to-generated transitional state
  cannot fall back to a false source classification, and upgrade planning no longer
  treats the nested Admin module or project generation baseline as a runtime identity.
  The checked-in compatibility workflow has a static contract test, but no real GitHub
  Actions run was performed for this development checkpoint. Feature freeze still
  requires an exact-SHA run that proves all four identities and digests, a real Blueprint
  0.1-to-0.2 customized upgrade, and an empty second upgrade.
- Added the D2 strict runtime-configuration checkpoint: exact-key YAML/JSON
  decoding, explicit Redis deployment modes, typed SecretRefs, immutable
  snapshots, and side-effect-free plans. Provider construction, health, and
  real Redis deployment conformance remain feature-freeze work.
- Added the D3 domain-neutral Runtime v2 resource graph at `d90b4c7`, with its
  deterministic close-generation evidence repaired at `c830b5f` and its public
  provider error tree repaired at `c57ffc8`, and
  deterministic side-effect-free graph preflight, topological startup, required
  readiness before dependent start, reverse rollback/close, graph-owned Run
  cancellation, concurrent idempotent close with retry after a bounded failure,
  and redacted lifecycle errors that preserve `errors.Is` classification without
  exposing provider objects or text through recursive unwrap/`errors.As`. Hermetic checkpoint
  evidence covers the state machine and owned handles; real provider health,
  Admin readiness-before-listen composition, and goroutine/file-descriptor leak
  bounds remain feature-freeze gates.
- Added the D3 additive named Redis resource at `86c0e8a`. One normalized Redis
  profile owns exactly one delayed standalone, Sentinel, or cluster go-redis client;
  stable resource-and-scope-prefixed capabilities lend structured caller-bounded
  leases without exposing the client or `Close`. Start/Ready/Health and commands
  honor caller deadlines, missing keys use a provider-neutral sentinel, and one
  tracked close generation invokes the context-free provider close exactly once.
  Twenty-two fully anchored top-level tests pass twenty uncached race-detected runs,
  including standalone miniredis and stalled-socket deadlines. The aggregate remains
  Planned: Sentinel control-plane ACL is anonymous, cluster multi-key operations are
  non-atomic with partial counts, and real Sentinel/cluster/TLS, Admin composition,
  Challenge injection, and leak conformance remain open.
- Added the D3 Framework Challenge checkpoint at `1faa9ef`. The public additive
  `runtime/challenge` package accepts one named Redis Scope, exposes explicit
  Begin/Commit/Abort/Verify outcomes without a raw client or `Close`, and uses an
  internal opaque same-slot bridge limited to fixed repository scripts. Rate-operation
  replay is idempotent at the limit boundary, and every syntactically valid Verify path
  performs one fixed read plus one fixed completion script. The deprecated D0 exported
  surface remains source-compatible and receives the same replay and redaction repairs.
  Twenty-two newly introduced top-level tests passed fully anchored, uncached `count=1`
  race evidence with `GOWORK=off`. Admin composition and real Redis Cluster/failover
  remain pending, so no capability is promoted to Stable.
- Composed the D3 Challenge runtime into Admin at `3e9ca94`. Startup now builds the
  named Redis resource `main` and Scope `challenge.email`, completes Start/Ready before
  publication and later route/listener assembly, and gives Config sole bounded-close
  ownership. Optional invalid or unavailable configuration keeps the application up but
  makes Challenge issuance and consumption return fixed 503 without falling back to the
  legacy global Redis path; required failure blocks setup. FakeCaptcha uses
  BeginIssue, SMTP delivery, and Commit/Abort, while login, registration, and password
  reset consume VerifyOutcome. Thirteen selected Admin top-level tests passed fully
  anchored, uncached `count=1` race evidence with `GOWORK=off`. Runtime configuration
  remains a startup snapshot, so changes require restart; browser and frozen-SHA
  provider/lifecycle gates remain pending and no capability is promoted to Stable.
- Added the D5 scoped runtime-cache checkpoint at `88f40c3`. The additive
  `mss-boot/runtime/cache` package declares database authority, Scope namespace,
  TTL, payload bound, provider-bypass, and loader reconstruction; provides
  singleflight plus generation invalidation; preserves not-found and RowsAffected;
  and bypasses shared state for active GORM transactions. QueryCache is an explicit
  opt-in loader adapter rather than a transparent plugin: callers own payload codecs
  and stable non-sensitive query identities, while cross-process outage recovery
  remains coupled to EventBus/database-revision reconciliation. Eight exact new tests
  passed uncached `count=1` race evidence with `GOWORK=off`; Planned/Beta status and
  the feature-freeze rerun remain unchanged.
- Added the D5 revision EventBus and Admin authorization-reconciliation checkpoint.
  Commit `04e8e0c` introduces an additive typed `mss-boot/runtime/eventbus` with
  process-local current-subscriber Memory fan-out, Redis Scope polling of the latest
  revision, panic isolation, degraded health, caller-bounded lifecycle, and a
  domain-neutral authoritative reconciler. Commit `160e2df` makes canonical Casbin
  mutation and its global `ConfigRevision` one transaction, publishes only after the
  commit, reloads authoritative policy in the subscriber, and registers a Memory
  runtime plus periodic reconciliation with the Admin server manager. Publication
  failure never rolls back an already committed policy, while missed, duplicate,
  out-of-order, panicking, disconnected, and commit-before-publish cases remain
  repairable without WorkQueue acknowledgement semantics. Exact uncached `count=1`
  race evidence covers seven Framework and eight Admin top-level tests; Framework
  runs with `GOWORK=off`, while Admin uses the current workspace until the unpublished
  v1.1.0 Framework dependency can be tagged and updated. EventBus is Beta and the
  aggregate Runtime v2 capability remains Planned pending the frozen-SHA rerun, real
  Redis multi-replica/failover evidence, and remaining runtime gates.
- Added the D5 provider-evidence validation checkpoint at `668dfe3`. The root CLI now
  strictly loads a repository-confined `ProviderMaturityReport`, validates pinned
  version/commit/fixture identities and internally consistent result counts, emits a
  deterministic normalized report, and makes required zero-run, skip, failure, partial,
  cached-only, or empty selections fail. Optional rows remain visible and non-blocking.
  The command only validates a supplied artifact: it starts no provider, creates no real
  provider report, and does not promote ObjectStore, RustFS, or any other provider.
- Added the cumulative D3 Supplier generator checkpoint. Commit `5a60ad6` projects the
  canonical AdminModule into an explicit lossless-ID SQLite/MySQL/PostgreSQL forward
  migration, model, validated DTOs, CRUD/query/export service, typed post-commit events,
  authorized HTTP operations, OpenAPI annotations, and exact generated tests under
  `admin/modules/supplier`. Commit `d92458c` adds the independent authorization migration
  `20260811120000`, persists the parent/menu plus hidden permission COMPONENT/API metadata,
  seeds exact admin/procurement/finance Casbin policy and role/global revisions atomically,
  and binds the AdminAuthorizer to canonical identity, HTTP method, full Gin path, declared
  root bypass, and ownership mode `none`. Route composition still fails closed without an
  explicit authorizer. The current dry-run honestly reports `phase=backend-checkpoint`,
  template `1.1.0-backend.3`, `complete=false`, 19 unchanged managed files, and 19 deferred
  frontend/documentation/E2E projections. Real MySQL 8.4 and PostgreSQL 17 migration tests
  passed at the earlier development checkpoint with zero skip, but must rerun on the selected
  feature-freeze SHA. Typed client, frontend/UI/browser E2E, generated module docs, and the
  customization-preserving upgrade rehearsal remain D5 work.
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
  typed environment SecretRefs. Admin startup configuration must migrate to this
  strict contract and neither provider is promoted. The nested framework retains
  a deprecated v1.0 source bridge for the removed storage symbols, but it rejects
  implicit credential fallback and is not the authoritative runtime path.
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

- Stopped the Account Settings profile form from resubmitting the immutable
  authentication email on every unrelated profile update.
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
  ambiguous lookup, and made registration plus first-time GitHub/Lark OAuth
  provisioning create new users atomically without provider-email account merge.
  Bounded opaque usernames protect the legacy `varchar(20)` field, while only
  the named email constraint becomes a fixed identity conflict. Admin create/update
  now return redacted fixed 422/409 responses and unrelated database errors retain
  the generic safe fallback. Self-service email mutation remains disabled. The
  fail-closed schema-readiness development checkpoint is complete, but the
  capability stays Planned until both readiness and real-database suites pass
  again on the exact feature-freeze SHA.
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
- Added the D2 Canonical Email Identity checkpoint note with the dialect-specific
  migration, privacy and API boundaries, exact SQLite/model/Controller evidence,
  schemahealth/migrate/server composition evidence, forward-only recovery guidance,
  and the still-open exact-freeze readiness plus MySQL/PostgreSQL zero-skip reruns.
- Added the D2 Downstream Snapshot Identity checkpoint note with the shared
  SnapshotStatus consumer contract, strict source/generated classification, fully
  anchored local evidence commands, and the still-open exact-SHA GitHub Actions,
  Blueprint 0.1-to-0.2, second-empty-upgrade, and release-built external artifact gates.
- Added the D3 Resource Lifecycle checkpoint note and a fully anchored evidence
  command requiring every top-level `runtime/resource` test for twenty uncached
  race-detected runs, without promoting the aggregate Storage Runtime capability.
- Added the D3 Named Redis Resource checkpoint note and a fully anchored evidence
  command requiring all twenty-two `runtime/redisresource` top-level tests, while
  keeping the capability Planned and listing every deferred Provider/composition gate.
- Added the D3 Challenge Runtime checkpoint note with fully anchored `count=1` race
  evidence for only the newly introduced public API, opaque bridge, Redis Scope adapter,
  replay-safe rate script, equal valid-Verify I/O, and legacy compatibility tests.
- Added the D5 Provider Evidence Validator checkpoint note and machine acceptance for
  exactly the six new validator tests and three new CLI tests introduced at `668dfe3`.
  The feature-freeze provider report remains a separate, not-yet-generated artifact.
- Added the D5 Scoped Runtime Cache checkpoint note with its explicit policy and
  caller-owned codec/QueryIdentity boundary, all eight fully anchored development
  tests, transaction isolation, and the still-open EventBus/revision plus frozen-SHA gates.
- Added the D3 Supplier Backend checkpoint note with its exact generated surface,
  machine-readable `complete=false` boundary, anchored generator/spec/Admin evidence,
  three-dialect development matrix, forward-only recovery, and explicit D4/D5 deferrals.

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
