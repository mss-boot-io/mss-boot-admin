# Changelog

All notable, verifiable changes to the `mss-boot` framework are documented in
this file. The format follows [Keep a Changelog](https://keepachangelog.com/),
and the project uses semantic versioning for nested-module releases.

## Unreleased

### Added

- Blocking lifecycle regression tests for cancellation, peer-error propagation,
  unexpected exits, graceful-shutdown deadlines, duplicate starts, and signal
  ownership.
- HTTP and gRPC lifecycle tests for listener errors, TLS configuration,
  operational hooks, metrics registration, and bounded shutdown.
- Race-oriented tests for task storage, validation translator initialization,
  request binding plans, provider authentication, and search reflection.
- A compiled basic HTTP example and an architecture contract document.

### Changed

- `Runnable.Start(context.Context) error` now has an explicit blocking lifecycle
  contract. The manager concurrently runs registered components, cancels peers
  on the first unexpected exit, and waits for bounded graceful shutdown.
- HTTP servers now configure read-header, read, write, idle, and shutdown
  timeouts and return listen/serve errors to the manager.
- gRPC servers no longer terminate the process during TLS initialization,
  mutate global tracing state, or panic on duplicate default metrics
  registration. Reflection, metrics registry, connection timeout, and shutdown
  timeout are configurable.
- Task servers block until cancellation and wait for in-flight cron jobs. The
  default task storage is concurrency-safe and returns deterministic key order.
- Request binding uses a deterministic cached plan: URI and query/form values
  are applied before one media-type-matched body, followed by one final
  validation pass.
- Validation translators are registered once before concurrent use.
- GORM search uses one shared filter chain for rows and count, always applies
  pagination, and returns initialization and row-iteration errors explicitly.
- GORM, MGM, Kubernetes, and custom actions share the same per-action
  authentication and middleware semantics.

### Fixed

- Fixed the manager deadlock/error-loss lifecycle in which long-running
  components could not be cancelled or background serve errors could be lost.
- Fixed `grpc.WithTimeout` writing the keepalive field instead of the connection
  timeout field.
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

### Security

- Authentication-enabled controllers now fail closed when the compatibility
  `response.AuthHandler` is missing, rather than registering an anonymous route.
- HTTP servers have non-zero defensive I/O timeouts by default.
- Invalid TLS configuration is returned to the embedding application as an
  error instead of terminating the process.

### Compatibility

- Custom `Runnable` implementations that previously launched a goroutine and
  returned immediately must now block until their work stops. Returning early
  is treated as an unexpected component exit.
- HTTP and gRPC `Start` methods now block and report runtime errors.
- GORM and MGM route naming retains the historical `mgm.CollName` behavior;
  Kubernetes controllers without a database model use their resource type.
- Correctness changes make previously ignored pagination, filters, and
  authentication settings effective. Roll back this change set as a unit if an
  application depended on the old unsafe behavior.

### Documentation

- Replaced stale repository links, nonexistent quick-start APIs, unsupported
  version instructions, and unenforced coverage claims.
- Removed historical free-form migration examples that referenced APIs not
  present in the current module. Released source tags remain the authority for
  older behavior.

## v0.7.3 - 2026-06-07

### Added

- Structured GitHub issue forms and refreshed open-source contribution entry
  points.

### Fixed

- Completed GORM query-cache tag invalidation for create, update, and delete
  paths.
- Validated MongoDB ObjectID input before delete operations.
