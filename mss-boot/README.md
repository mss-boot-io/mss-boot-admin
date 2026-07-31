# mss-boot

---
<img align="right" width="320" src="https://docs.mss-boot-io.top/favicon.ico"  alt="https://github.com/mss-boot-io/mss-boot"/>


[![ci](https://github.com/mss-boot-io/mss-boot/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mss-boot-io/mss-boot/actions/workflows/codeql.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://github.com/mss-boot-io/mss-boot/actions/workflows/scorecard.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot/actions/workflows/scorecard.yml)
[![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot/releases)
[![License](https://img.shields.io/github/license/mss-boot-io/mss-boot.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot/blob/main/LICENSE)

English | [简体中文](https://github.com/mss-boot-io/mss-boot/blob/main/README.Zh-cn.md)

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/mss-boot-io/mss-boot)

An enterprise-level language heterogeneous microservice solution that supports grpc and http protocols. The single-service code framework adheres to the principle of minimalism, while providing complete devops process support (gitops).

## 📦 Latest Version: v0.7.3

[documentation](https://docs.mss-boot-io.top)

[contributing](./CONTRIBUTING.md) · [security](./SECURITY.md) · [good first issues](https://github.com/mss-boot-io/mss-boot/issues?q=is%3Aissue%20is%3Aopen%20label%3A%22good%20first%20issue%22)

[http service template](https://github.com/mss-boot-io/service-http)

[grpc service template](https://github.com/mss-boot-io/service-grpc)

## ✨ Features
> - Follow RESTful API Design Specifications
> - Login support idp(dex)
> - Support for Swagger documentation (based on swaggo)
> - Code generation tool
> - Perfect cicd package
> - Comprehensive core libraries (log, cache, queue, search, config)
> - Multi-database support (MySQL, PostgreSQL, SQLite, MongoDB)
> - Multi-storage provider support (local, S3, embed)
> - Advanced error handling with standardized error codes
> - Action scope management for context-aware operations
> - Extensive middleware ecosystem

## 🚀 v0.7.x Highlights

### Core Improvements
- **Enhanced Error Handling**: Standardized error codes and improved error propagation
- **Action Scope Management**: Better context management for complex operations
- **Query Cache Invalidation**: More complete GORM query-cache tag invalidation for create, update, and delete paths
- **Mongo Safety**: ObjectID validation before Mongo delete operations
- **Dependency Updates**: Comprehensive dependency refresh across all modules
- **Performance Optimizations**: Improved memory usage and response times

### Testing & Quality
- **Test Coverage**: Comprehensive test suite with 80%+ coverage requirement
- **Integration Testing**: Robust integration tests for all core components
- **CI/CD Pipeline**: Enhanced GitHub Actions workflow with quality gates
- **Open Source Intake**: Structured issue forms and refreshed contributor entry points

### Documentation
- **Comprehensive Guides**: Updated documentation for all core features
- **API Reference**: Complete Swagger documentation for all endpoints
- **Migration Guides**: Clear upgrade paths from previous versions

## 🧭 Action/Controller Glossary

The request flow uses a small set of repeated terms:

- **Controller**: A `pkg/response.Controller` implementation that owns a route path, route-level Gin handlers, and the mapping from an action name to a concrete `Action`. `pkg/response/controller.Simple` chooses provider-specific actions for GORM, MGM, or Kubernetes models.
- **Action**: A named request operation such as `get`, `search`, `control`, or `delete`. An action implements `response.Action`, returns a Gin handler chain, and contains the provider-specific logic for handling that operation.
- **Hook**: A callback attached to a lifecycle point. Common examples are `BeforeCreate`, `AfterUpdate`, `BeforeSearch`, config source `PrefixHook` / `PostHook`, and server start or shutdown hooks.
- **Scope**: A per-request query filter, usually supplied through `WithScope`, that turns the current Gin context and model table into a GORM scope. Use scopes for tenant filters, ownership filters, or other contextual constraints.
- **Provider**: The selected backend implementation for a model, config source, or storage target. For controllers this is usually `ModelProviderGorm`, `ModelProviderMgm`, or `ModelProviderK8S`; config and storage packages have their own provider enums.

## 📋 Todo List
> - [x] Support dynamodb
> - [x] Support config provider  
> - [x] Support istio traces
> - [x] Out-of-the-box support
> - [x] Enhanced error handling (v0.7.x)
> - [x] Action scope management (v0.7.x)
> - [x] Comprehensive testing infrastructure (v0.7.x)

## 🧪 Testing

The project follows strict testing requirements:

### Test Types
- **Unit Tests**: `*_test.go` files alongside source code
- **Integration Tests**: Database and API integration validation  
- **E2E Tests**: Full stack testing with real dependencies

### Coverage Requirements
- **Minimum Coverage**: 80%
- **Critical Components**: 85%+
- **New Code**: Must meet or exceed existing coverage

### Running Tests
```bash
# Required local test gate
go test ./...

# Coverage report
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

# Integration tests (requires configured external services)
go test -tags=integration ./...
```

Integration-style tests should skip when optional databases, object storage, Kubernetes clusters, cloud credentials, or other external services are not configured.

### Running Lint
```bash
# Install the tools used by the CI lint job when they are not already available
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Local lint sweep
golangci-lint run ./...
```

The GitHub Actions lint job runs `golangci-lint` from the repository root with the latest action-provided version. If a local binary was built with an older Go version than `go.mod` targets, reinstall it before running the sweep. The lint job is currently advisory while the existing lint backlog is reduced; `go test ./...` remains the required CI gate.

## 🔧 Quick Start

### Requirements
- Go 1.26+

### Using Go Modules
```bash
go get github.com/mss-boot-io/mss-boot@v0.7.3
```

### Basic Usage
```go
package main

import (
    "github.com/mss-boot-io/mss-boot/core/server"
    "github.com/mss-boot-io/mss-boot/pkg/log"
)

func main() {
    s := server.New()
    if err := s.Run(); err != nil {
        log.Fatal("server run failed", log.Err(err))
    }
}
```

### Local Checks
```bash
make tidy
make test
make coverage
make lint
```

## 📝 CHANGELOG

For detailed release notes and migration guides, see [CHANGELOG.md](./CHANGELOG.md).

## Buy me a coffee
<a href="https://www.buymeacoffee.com/lwnmengjing" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>

## JetBrains open source certificate support

The `mss-boot-io` project has always been developed in the GoLand integrated development environment under JetBrains, based on the **free JetBrains Open Source license(s)** genuine free license. I would like to express my gratitude.

<a href="https://www.jetbrains.com/?from=kubeadm-ha" target="_blank"><img src="https://raw.githubusercontent.com/panjf2000/illustrations/master/jetbrains/jetbrains-variant-4.png" width="250" align="middle"/></a>

## 🔑 License

[MIT](https://raw.githubusercontent.com/mss-boot-io/mss-boot/main/LICENSE)

Copyright (c) 2022 mss-boot-io
