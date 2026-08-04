# Runtime and request contracts

Status: accepted for the pre-v1 runtime-hardening line.

This document defines observable framework behavior. Tests are the executable
specification; this document explains the intended architecture and migration
impact.

## Goals

The runtime must be predictable under cancellation and failure, adapters must
return errors instead of terminating the process, request decoding must be
deterministic, and authentication settings must behave consistently across
providers.

The hardening work deliberately preserves public constructor and option shapes
where possible. It fixes correctness before attempting a physical multi-module
split.

## 1. Component lifecycle

A managed component implements:

```go
type Runnable interface {
    fmt.Stringer
    Start(context.Context) error
}
```

`Start` is a blocking operation. It returns only when one of these events
occurs:

1. the supplied context is cancelled and owned resources have stopped;
2. the component encounters an unrecoverable runtime error;
3. the component exits normally because its underlying service was explicitly
   stopped.

A component that spawns background work and immediately returns violates the
contract. The manager treats a successful early return as
`ErrRunnableStopped`, cancels every peer, and reports the unexpected exit.

### Manager behavior

- Registrations have deterministic insertion order. Replacing a component with
  the same name preserves its original position.
- Components run concurrently.
- The first unexpected error or early exit cancels the shared context.
- Caller cancellation and configured process signals are normal shutdown paths.
- The manager waits for components to return and joins real errors returned
  during shutdown. A matching `context.Canceled` result is treated as normal;
  flush, close, and component deadline errors remain visible to the caller.
- If the configured outer grace period expires, the manager returns
  `ErrGracefulShutdownTimeout`.
- The default outer grace period is 30 seconds. It is deliberately longer than
  built-in component shutdown deadlines. Custom configurations should preserve
  this hierarchy so component-specific errors can be returned first.
- A manager instance can be started once. Additions after start are ignored for
  compatibility with the existing `Add` signature, which cannot return an
  error.

The application entry point should normally own operating-system signals:

```go
ctx, stop := signal.NotifyContext(
    context.Background(),
    os.Interrupt,
    syscall.SIGTERM,
)
defer stop()

manager := server.New(server.WithoutSignalHandling())
if err := manager.Start(ctx); err != nil {
    return err
}
```

Default manager signal handling remains available for existing applications.

## 2. HTTP lifecycle and safety defaults

The HTTP adapter creates the listening socket before reporting that it has
started. Listen and serve failures are returned to the manager.

Default defensive timeouts are non-zero:

- read-header timeout;
- full-request read timeout;
- response write timeout;
- keep-alive idle timeout;
- graceful-shutdown timeout.

Granular duration options are preferred. The legacy integer `WithTimeout`
option remains supported and applies the supplied seconds to all timeout
categories.

On cancellation, the adapter calls `http.Server.Shutdown` with a fresh shutdown
context. If the deadline expires it calls `Close`, waits for the serve loop to
return, and joins relevant errors. A configured end hook runs before `Start`
returns.

Certificate and private-key paths are an all-or-nothing pair. Partial TLS
configuration is rejected before binding a socket.

## 3. gRPC lifecycle and process ownership

The gRPC adapter follows the same blocking contract. It returns listen and
serve errors, performs `GracefulStop`, and calls `Stop` if the independent
shutdown deadline expires.

Framework construction must not call `os.Exit` or `log.Fatal`. TLS-loading
errors are retained by the compatibility constructor and returned by `Start`.
Duplicate registration of the shared Prometheus collector is accepted without
panic; callers can provide an isolated registry or disable registration.

Server reflection remains enabled by default for compatibility and can be
disabled with `WithReflection(false)` in production environments.

## 4. Scheduled tasks

The task server loads stored schedules, starts cron, waits for cancellation,
and then waits for running jobs to complete. The default in-memory storage uses
a read/write lock and sorted key enumeration so concurrent access and startup
order are deterministic.

Task shutdown has an independent configurable timeout. The default is five
seconds. When a running cron job does not finish before that deadline, `Start`
returns `context.DeadlineExceeded`; the outer manager then reports the component
shutdown failure instead of waiting forever.

The current process-wide task singleton is retained for compatibility. Replacing
it with instance-scoped schedulers is a later architectural milestone.

## 5. Request binding

The binding pipeline is deterministic:

1. URI parameters;
2. query values and declared form fields;
3. at most one body binder selected from the request media type;
4. one validation pass over the completed DTO.

`GET`, `HEAD`, and `OPTIONS` never consume JSON, XML, or YAML bodies. A POST,
PUT, or PATCH request can combine query values with a JSON/XML/YAML body. Form
and multipart media types use Gin's form binder rather than falling through to
JSON.

Binding plans are cached by reflected type, returned as defensive copies, and
safe for concurrent use. Validation translations are registered once and
translated error fields are emitted in stable order.

## 6. Controller authentication

Authentication is action-specific because `WithNoAuthAction` can exempt one
operation while protecting others. Controller-level middleware is therefore
attached once to each action, not both to the router group and the action.

The following providers share the same rules:

- GORM;
- MongoDB/MGM;
- Kubernetes;
- custom actions registered through `WithAction`.

When `WithAuth(true)` applies, authentication middleware runs before the
provider handler. If the global `response.AuthHandler` compatibility hook is
nil, the request is rejected with an internal configuration error. It is never
silently registered as an anonymous endpoint.

Long term, authentication should become an instance dependency instead of a
global variable. The fail-closed compatibility bridge prevents unsafe behavior
until that migration is complete.

## 7. GORM search semantics

Search creates a base query from:

- the selected table/resolver;
- request-derived conditions;
- an optional request-aware custom scope.

The row query adds pagination and optional tree preloads. The count query
rebuilds the same base filters without pagination. This ensures total and rows
represent the same data set.

Search validates required configuration, reports an uninitialized database,
checks row iteration and close errors, and safely copies result models.

The reflection query resolver descends only into nested structs. Scalar
pagination fields, nil pointers, invalid values, and short `between` ranges are
ignored rather than panicking. Search-tagged string operations are applied only
to string values, and `isnull` emits valid `IS NULL` SQL.

## 8. Compatibility and rollback

These changes preserve exported function names and most option signatures but
tighten observable behavior:

- custom runnables must block;
- HTTP and gRPC starts no longer return immediately;
- configured filters and pagination are now effective;
- authentication-enabled MGM actions are now protected;
- GET-family methods no longer read structured bodies;
- task shutdown is bounded and can return a deadline error;
- invalid TLS and framework configuration return errors instead of exiting.

Applications should run API and integration tests before release. If an
application depended on ignored authentication, filters, or lifecycle errors,
roll back the complete hardening change set rather than selectively reverting
one adapter.

## 9. Verification gates

The independent module gate is:

```bash
cd mss-boot
GOWORK=off go test -race -shuffle=on -count=1 ./...
GOWORK=off go vet ./...
GOWORK=off go mod tidy
git diff --exit-code -- go.mod go.sum
```

The monorepo gate additionally runs application tests and a backend build.
External-service integration tests must explicitly skip when credentials or
services are absent.

## 10. Roadmap toward a stable v1

The current hardening removes several blocking correctness and security issues,
but it is not the final architecture. The highest-value remaining work is:

1. replace process-wide database, authentication, response, cache, and task
   globals with explicit instance dependencies;
2. make database opening return an owned handle and errors, remove library-level
   exits, verify RDS TLS identities, and refresh IAM tokens per physical
   connection;
3. invert configuration-source dependencies through a registered `Source`
   interface with context-aware load, watch, and close behavior;
4. separate provider-specific controllers and repository adapters from the HTTP
   contract instead of extending a central provider switch;
5. introduce typed public errors that separate client-safe messages from
   internal causes;
6. enforce measured coverage thresholds and real integration matrices before
   claiming them;
7. split adapter modules only after dependency direction is clean and stable.

These milestones address the remaining architecture-boundary and global-state
risk without destabilizing the correctness fixes in this line.
