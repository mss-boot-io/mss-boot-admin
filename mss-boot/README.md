# mss-boot

[![CI](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/codeql.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/codeql.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](./LICENSE)

English | [简体中文](./README.Zh-cn.md)

`mss-boot` is the reusable Go framework module inside the
[`mss-boot-admin`](https://github.com/mss-boot-io/mss-boot-admin) monorepo. It
provides managed HTTP, gRPC, and scheduled-task runtimes together with common
configuration, persistence, storage, security, and request-controller adapters.

The module remains independently consumable and is validated with `GOWORK=off`
to prevent the repository workspace from hiding missing dependencies.

## Status

- Module path: `github.com/mss-boot-io/mss-boot-admin/mss-boot`
- Source directory: `mss-boot/`
- Go requirement: Go 1.26 or later
- Stability: pre-v1; exported APIs are compatibility surfaces, but correctness
  fixes may tighten previously ambiguous behavior

For a development build, use `main`; for production, replace it with a released
nested-module tag or an immutable commit:

```bash
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@main
```

## Capabilities

- Context-driven lifecycle management for HTTP, gRPC, and cron components
- HTTP operational endpoints, timeouts, TLS, metrics, and graceful shutdown
- gRPC middleware, OpenTelemetry integration, Prometheus metrics, TLS, and
  bounded graceful shutdown
- GORM, MongoDB/MGM, and Kubernetes controller actions
- Configuration sources including local files, databases, object storage,
  Kubernetes ConfigMaps, Consul, and AWS AppConfig
- Cache, queue, object-storage, migration, security, language, and response
  helpers

Optional integrations must return errors or skip when unconfigured; framework
packages must not terminate the embedding process.

## Runtime contract

A managed component implements:

```go
type Runnable interface {
    fmt.Stringer
    Start(context.Context) error
}
```

`Start` must block until the component stops, its context is cancelled, or an
unrecoverable runtime error occurs. It must release owned resources before
returning. The manager runs components concurrently, cancels peers on the first
unexpected exit, and waits for graceful shutdown up to the configured timeout.

The manager handles `SIGINT` and `SIGTERM` by default for compatibility. An
application that owns process signals should use `server.WithoutSignalHandling()`
and pass a context created by `signal.NotifyContext`.

See [Runtime and request contracts](./docs/architecture/runtime-and-request-contracts.md)
for lifecycle, binding, authentication, query, and compatibility details.

## Quick start

The complete example is compiled as part of the framework test suite:
[`examples/basic-http/main.go`](./examples/basic-http/main.go).

```go
package main

import (
    "context"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"

    "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server"
    "github.com/mss-boot-io/mss-boot-admin/mss-boot/core/server/listener"
)

func main() {
    ctx, stop := signal.NotifyContext(
        context.Background(),
        os.Interrupt,
        syscall.SIGTERM,
    )
    defer stop()

    mux := http.NewServeMux()
    mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte("ok\n"))
    })

    manager := server.New(server.WithoutSignalHandling())
    manager.Add(listener.New(
        listener.WithAddr(":8080"),
        listener.WithHandler(mux),
    ))

    if err := manager.Start(ctx); err != nil {
        slog.Error("server stopped unexpectedly", "error", err)
        os.Exit(1)
    }
}
```

## Request and controller guarantees

- URI and query/form values are bound before at most one media-type-matched
  request body; the completed DTO is validated once.
- `GET`, `HEAD`, and `OPTIONS` requests never consume JSON, XML, or YAML bodies.
- `WithAuth(true)` is fail-closed for GORM, MGM, Kubernetes, and custom actions.
  If the global compatibility handler is missing, the request is rejected as a
  server configuration error rather than becoming anonymous.
- GORM search always applies request conditions and pagination. Count queries
  use the same filters without pagination.
- Search reflection ignores unsupported DTO fields instead of panicking.

## Validation

From the repository root:

```bash
make deps-framework
make test-framework
```

For an independent, race-tested framework check:

```bash
cd mss-boot
GOWORK=off go test -race -shuffle=on -count=1 ./...
GOWORK=off go vet ./...
GOWORK=off go mod tidy
git diff --exit-code -- go.mod go.sum
```

Tests that require optional databases, object storage, Kubernetes clusters, or
cloud credentials must skip when those integrations are not configured. No
coverage percentage is claimed until CI enforces the same threshold.

## Compatibility and releases

The module is pre-v1. Changes that alter observable behavior must include tests,
documentation, security impact, and migration or rollback notes. Nested-module
releases should use tags in the form `mss-boot/vX.Y.Z`.

See [CHANGELOG.md](./CHANGELOG.md), [CONTRIBUTING.md](./CONTRIBUTING.md), and
[SECURITY.md](./SECURITY.md).

## License

[MIT](./LICENSE)
