# Changelog

All notable, verifiable changes to the `mss-boot` framework are documented in
this file. The format follows [Keep a Changelog](https://keepachangelog.com/),
and the project uses semantic versioning for nested-module releases.

## [Unreleased]

Target: **mss-boot/v1.1.0 development train**. Intermediate patch and public
prerelease tags are disabled; current Challenge, Kafka, and object-provider work
remains internal and does not promote provider maturity.

### Changed

- Replaced legacy object-storage provider globals with an exact Local-or-S3
  startup configuration, immutable `StorageProfile`, explicit default/static
  credential modes, typed environment `SecretRef` resolution, and one reusable
  `StorageHandle` per profile. This is an intentional breaking configuration/API
  contract; Local and S3-compatible remain Legacy/Blocked.
- Added leased Local/S3 use and bounded, retryable, idempotent close semantics so
  one owner rejects new work during shutdown, drains in-flight operations, and
  closes its private HTTP transport exactly once.
- Preserved `AdapterQueue` for source compatibility and added the additive
  `ManagedAdapterQueue` contract with error-returning context-aware registration,
  blocking start, observed errors, and context-bounded close. `Queue.InitContext`
  is the authoritative construction path; the historical wrapper remains
  non-terminating for compatibility.
- Replaced truncated integer migration registration with lossless decimal
  `MigrationID` values, deterministic numeric ordering, duplicate preflight before
  database access, context-aware error propagation, and explicit aliases for all
  published v1.0.0 marker forms. The historical no-return `Migrate` method remains
  as a logging-only source-compatibility bridge; correctness-sensitive callers use
  `MigrateContext`.

### Fixed

- Moved the legacy Kafka consumer's Sarama session mark after successful decode
  and handler completion, passed the session context to handlers, removed raw
  payload logging, and stopped canceled or closed consumer loops. D1 also validates
  the startup profile under the caller context, owns one producer and one consumer
  group per unique topic/group, returns registration/factory errors, observes
  consumer errors, rejects per-append configuration and unsupported manual commit,
  and supplies cancellable start plus idempotent, deadline-bounded, retryable close.
  Kafka remains Legacy/Blocked until retry/backoff, dead-letter, rebalance, outage,
  idempotency, manual-commit policy, and real-broker conformance gates pass.
- Made the S3 configuration source require a caller-owned context and client,
  close object bodies on success and read failure, and return an explicit
  unsupported error for Watch. Bootstrap now owns and closes a profile Handle
  independently from the Admin application object-storage client. A missing stage
  object is the only optional overlay; read/malformed-overlay failures fail closed,
  and HTTP requires explicit `s3_tls_allow_insecure_http=true`.

### Security

- Added a provisional purpose-scoped Redis challenge implementation with
  cryptographic codes, versioned HMAC verifiers, same-slot Lua transitions,
  delivery compensation, pending-lease recovery, subject/caller/global quotas,
  attempt limits, pepper rotation, and exactly-once successful verification.
- Restricted the development-checkpoint provider claim to standalone Redis: generated
  per-subject keys are preflighted for one hash tag, while concrete Cluster and
  Ring clients fail closed until real multi-node conformance exists.
- Permanently disabled the unsafe legacy verification-code behavior while
  retaining its construction symbols for an explicit migration failure.

### Documentation

- Reconciled the changelog with the published nested-module Release and added
  the internal storage-safety and Storage Runtime v2 planning contracts.
- Recorded the D1 object-provider checkpoint and its deferred S3 Put, Delivery,
  and RustFS conformance boundary. The provider catalog remains unchanged at
  Legacy/Blocked.
- Recorded the additive managed Kafka lifecycle, exact owner/configuration evidence,
  and Admin Runnable boundary. D1 is complete and development proceeds to D2;
  lifecycle completion does not promote Kafka beyond Legacy/Blocked.

## [mss-boot/v1.0.0] - 2026-08-09

Status: **published / stable**. The `mss-boot/v1.0.0` nested-module Release was
published before root `v1.0.0`, resolves externally with `GOWORK=off`, and points
to `ee800262c035c5f4242aca1841d077554481d2c4`. The exact-commit evidence is
recorded in repository issue `#471`.

This tag is the reusable framework's first stable 1.0 release from the
consolidated repository. Any package, checksum, proxy lookup, or test result
created for the unpublished v0.8.0 candidate was excluded; the accepted evidence
was regenerated from the exact v1.0.0 release commit.

The compatibility and rollout requirements are part of the consolidated
[v1.0.0 release contract](../docs/docs/releases/v1-0-0.md).

### Added

- Blocking lifecycle regression tests for cancellation, peer-error propagation,
  unexpected exits, graceful-shutdown deadlines, duplicate starts, shutdown
  error collection, and signal ownership.
- HTTP and gRPC lifecycle tests for listener errors, TLS configuration,
  operational hooks, metrics registration, and bounded shutdown.
- Race-oriented tests for task storage and shutdown, validation translator
  initialization, request binding plans, provider authentication, search
  reflection, database ownership, and per-connection IAM token refresh.
- A compiled basic HTTP example and architecture contracts for runtime/request
  behavior and database ownership/RDS IAM.
- `gormdb.Handle`, `Database.Open`, `InstallDefault`, `DefaultHandle`, and
  `ClearDefault` APIs for explicit database resource ownership and gradual
  migration away from process globals.

### Changed

- The public module path moves from the predecessor standalone module
  `github.com/mss-boot-io/mss-boot` to the consolidated nested module
  `github.com/mss-boot-io/mss-boot-admin/mss-boot`. Downstream `require`,
  `replace`, and import paths must be updated together.
- `Runnable.Start(context.Context) error` now has an explicit blocking lifecycle
  contract. The manager concurrently runs registered components, cancels peers
  on the first unexpected exit, joins component shutdown errors, and waits for
  bounded graceful shutdown.
- The manager's default outer shutdown deadline is 30 seconds so built-in
  adapters can complete their shorter deadlines and return specific errors.
- HTTP servers now configure read-header, read, write, idle, and shutdown
  timeouts and return listen/serve errors to the manager.
- gRPC servers no longer terminate the process during TLS initialization,
  mutate global tracing state, or panic on duplicate default metrics
  registration. Reflection, metrics registry, connection timeout, and shutdown
  timeout are configurable.
- Task servers block until cancellation and wait for in-flight cron jobs up to a
  configurable shutdown timeout. The default task storage is concurrency-safe
  and returns deterministic key order.
- Request binding uses a deterministic cached plan: URI and query/form values
  are applied before one media-type-matched body, followed by one final
  validation pass.
- Validation translators are registered once before concurrent use.
- GORM search uses one shared filter chain for rows and count, always applies
  pagination, and returns initialization and row-iteration errors explicitly.
- GORM, MGM, Kubernetes, and custom actions share the same per-action
  authentication and middleware semantics.
- Database initialization can return an owned Handle and errors without
  mutating configuration or package globals. The historical `Init` method is a
  logging-only compatibility wrapper rather than a process-terminating API.
- RDS IAM generates credentials before every new MySQL or PostgreSQL physical
  connection and caps pool connection lifetime at 14 minutes.

### Fixed

- Fixed the manager deadlock/error-loss lifecycle in which long-running
  components could not be cancelled, background serve errors could be lost, or
  real flush/close errors returned during cancellation could be discarded.
- Fixed `grpc.WithTimeout` writing the keepalive field instead of the connection
  timeout field.
- Fixed task shutdown waiting forever for a blocked cron job; it now returns a
  deadline error when its configured shutdown period expires.
- Fixed GORM Search silently skipping request conditions and pagination when no
  custom scope was configured.
- Fixed `GET`, `HEAD`, and `OPTIONS` binding from attempting to consume JSON,
  XML, or YAML request bodies.
- Fixed Mongo/MGM controllers ignoring `WithAuth(true)`.
- Fixed search reflection recursing into scalar pagination fields and panicking;
  nil nested values, malformed `between` values, and `isnull` fields are now
  handled safely.
- Replaced library-level process exit on nil response context with a recoverable
  programming-error panic.
- Removed unused synchronization-bearing table-test fields that caused `go vet`
  `copylocks` failures.
- Replaced GORM database initialization exits with returned errors and ensured
  post-open failures close the owned primary connection pool.
- Fixed PostgreSQL RDS IAM storing one startup-time token in the DSN.
- Fixed MySQL RDS IAM forcing `tls=skip-verify` and mutating the global dialector
  registry.

### Security

- Authentication-enabled controllers now fail closed when the compatibility
  `response.AuthHandler` is missing, rather than registering an anonymous route.
- HTTP servers have non-zero defensive I/O timeouts by default.
- Invalid TLS configuration is returned to the embedding application as an
  error instead of terminating the process.
- RDS IAM verifies certificate chains and server names by default, supports an
  explicit root CA file, and prevents connection params from downgrading TLS.
- `insecureSkipVerify` remains only as an explicit compatibility escape hatch
  and defaults to false.

### Compatibility

- Publish `mss-boot/v1.0.0` before any root Admin release that requires
  `github.com/mss-boot-io/mss-boot-admin/mss-boot v1.0.0`. Verify resolution from
  a clean external temporary module without the repository `go.work` file.
- A source checkout using `go.work` proves workspace compatibility only; it
  does not prove that the nested module tag is available to downstream users.
- Custom `Runnable` implementations that previously launched a goroutine and
  returned immediately must now block until their work stops. Returning early
  is treated as an unexpected component exit.
- HTTP and gRPC `Start` methods now block and report runtime errors.
- Task `Start` can now return `context.DeadlineExceeded` when a running job
  exceeds its shutdown timeout.
- GORM and MGM route naming retains the historical `mgm.CollName` behavior;
  Kubernetes controllers without a database model use their resource type.
- Correctness changes make previously ignored pagination, filters, and
  authentication settings effective. Roll back this change set as a unit if an
  application depended on the old unsafe behavior.
- `gormdb.DB`, `gormdb.Enforcer`, and `Database.Init` remain available while
  applications migrate to owned Handles. `Database.Init` no longer exits on
  failure; callers that require startup guarantees must use `Open` or
  `InitContext` and handle the returned error.
- RDS IAM combined with dbresolver registrations now returns an explicit error
  until every source and replica pool has an ownership-aware dynamic credential
  adapter.

### Documentation

- Replaced stale repository links, nonexistent quick-start APIs, unsupported
  version instructions, and unenforced coverage claims.
- Removed historical free-form migration examples that referenced APIs not
  present in the current module. Released source tags remain the authority for
  older behavior.
- Added database Handle ownership, RDS IAM TLS, migration, and rollback
  guidance.

## v0.7.3 - 2026-06-07 (pre-consolidation history)

This entry records predecessor framework history. It is not evidence that a
`mss-boot/v0.7.3` tag exists in the consolidated repository.

### Added

- Structured GitHub issue forms and refreshed open-source contribution entry
  points.

### Fixed

- Completed GORM query-cache tag invalidation for create, update, and delete
  paths.
- Validated MongoDB ObjectID input before delete operations.
