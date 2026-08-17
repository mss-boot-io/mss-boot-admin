# mss-boot-admin

[![CI](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/releases)
[![License](https://img.shields.io/github/license/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/blob/main/LICENSE)

English | [简体中文](./README.zh-CN.md)

## Introduction

> `mss-boot-admin` is an Agent-native management-system development foundation. It combines a production-oriented Gin + React + Ant Design reference application with machine-readable project contracts, Feature and AdminModule specifications, deterministic full-stack generation, repository Skills, a project MCP server, reproducible setup, change-aware verification, Agent Evals, versioned application Blueprints, and conflict-aware downstream upgrades.

> The runtime admin platform still provides identity, RBAC, organization, configuration, audit, notification, task, internationalization, storage, WebSocket, and observability capabilities. Runtime dynamic models, virtual CRUD, and browser-facing code generation have been removed; new business modules use development-time specifications, the offline deterministic `cmd/mss` generator, and compiled vertical modules.

## Agent-native workflow

```text
business intent
  → Feature and Acceptance contract
  → AdminModule contract
  → deterministic generation
  → Agent implements non-template business rules
  → change-aware verification and Evals
  → reviewable PR and upgradeable downstream application
```

```shell
./mss context --format json
./mss doctor --strict --format json
./mss setup
./mss dev --detach
./mss verify --changed
./mss eval run --all
```

## Recent Updates

The project has undergone comprehensive polish rounds focusing on:

- **Stability (P0)**: Fixed nil dereference risks, boundary checks, panic prevention, auth coverage, and polling cleanup
- **Code Quality**: Eliminated all TypeScript errors, unified page structures, removed duplicate keys
- **Abstraction**: Created reusable components (AuthShell, useMonitorData hook), unified API response format
- **Testing**: Documented 70+ test cases, executed core scenarios successfully
- **Documentation**: Added CHANGELOG, CONTRIBUTING guide, and comprehensive configuration tutorial

[Beta Environment](https://admin-beta.mss-boot-io.top)

[Swagger](https://mss-boot-io.github.io/mss-boot-admin/swagger.json)

## Tutorial
[Online documentation](https://docs.mss-boot-io.top)
[Video tutorial](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)

## Repository layout

| Path | Component |
| --- | --- |
| `/` | Go admin backend |
| `mss-boot/` | Reusable Go framework module |
| `web/antd-v6/` | React 19 + Ant Design 6 frontend, independently released |
| `docs/` | Dumi documentation site |

All active development now happens in this repository. The former standalone repositories are retained only as migration history and compatibility references.

## 🎬 Experience environment
[Experience address](https://admin-beta.mss-boot-io.top)
> Account: admin Password: 123456

## ✨ Features
- Support internationalization
- Standard Restful API development specifications
- RBAC permission management based on Casbin
- Database storage based on Gorm
- Middleware development based on Gin
- Swagger document generation based on Gin
- Support oauth2.0 third-party login
- Support swagger document generation
- Support multiple configuration sources (local files, embed, object storage s3, etc., databases supported by gorm, mongodb)
- Support database migration
- Support governance-oriented admin modules such as users, roles, menus, departments, posts, APIs, and configuration
- Support operational modules such as notices, tasks, monitoring, and statistics
- Agent-native contracts, deterministic generation, project MCP tools, change-aware verification, downstream Blueprints, and three-way foundation upgrades

## 📦 Built-in functions
- User management: Users are system operators, and this function mainly completes the configuration of system users.
- Department management: Maintain organization hierarchy and ownership boundaries.
- Post management: Maintain post data for organization and permission collaboration.
- Role management: Role menu permission allocation, set role data range permission division by organization.
- Menu management: Configure system menus, operation permissions, button permission identifiers, etc.
- API management: Maintain API registry information for governance and permission mapping.
- Option management: dynamically configure enumerations.
- System configuration: Manage the configuration of various environments.
- Notice announcement: user notification message.
- Task management: Manage scheduled tasks, including execution logs.
- Internationalization management: Manage internationalization resources.
- Account and token management: Support OAuth2 binding and personal access tokens.
- Monitoring and statistics: Provide runtime visibility and statistics querying capabilities.

## RBAC Glossary

| Term | Meaning in mss-boot-admin |
| --- | --- |
| User | A system operator. Users authenticate, receive roles, and operate within the permissions granted by those roles. |
| Role | A permission group stored in `mss_boot_roles`. Roles are the main subject in Casbin policies and are assigned to users. |
| Menu | A frontend navigation or permission node stored in `mss_boot_menus`. Menu records can represent directories, pages, components, or API permission nodes. |
| API | A backend route record stored in `mss_boot_api`, usually generated from Gin route metadata and used for permission governance. |
| Permission path | The menu/API path written into authorization requests and Casbin rules. Duplicate or blank paths are ignored before rules are built. |
| Casbin rule | A policy row in `mss_boot_casbin_rule`. The common shape is `p, roleID, accessType, path, method`. |
| Access type | The rule scope, such as `MENU`, `API`, or component access. Role authorization can combine menu rules and child API rules. |
| Data scope | The organization/data boundary attached to a role. It controls which department-owned data a role should be able to access. |
| Default role | A role marked as default. New menu access can be granted to it automatically when menu records are created. |

## 📦 Preparation
- Install Go 1.26+
- Optional for backend integration testing: MySQL 8.0+ and Redis 7+
- Primary frontend development: Node.js 24 and pnpm 10.34.5 through Corepack

## 📦 Quick start

```shell
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin

./mss doctor --strict --format json
./mss setup
./mss dev --detach
./mss dev status --format json
```

The single Admin client is served at `http://localhost:8001` from `web/antd-v6`.

Create or validate development contracts before editing repetitive code:

```shell
./mss spec validate .mss/features/example-supplier-onboarding.yaml
./mss feature plan .mss/features/example-supplier-onboarding.yaml
./mss module generate .mss/modules/example-supplier.yaml --format json
./mss verify --changed
```

The manual backend, frontend, migration, Blueprint, upgrade, Skills, MCP, and Eval workflows are documented under `docs/docs/agent/`.

## 📨 Interaction
<table>
   <tr>
    <td><a href="https://t.me/+318z6NULrw81N2E1" target="_blank"><img src="https://th.bing.com/th/id/OIP.lYN2s7Dv1a4pLAVUaXMCVgAAAA?rs=1&pid=ImgDetMain" width="180px"></a></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat-mp.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/qq-group.jpg" width="200px"></td>
    <td><a href="https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026&ctype=0">mss-boot-io</a></td>
  </tr>
  <tr>
    <td>telegram🔥🔥🔥</td>
    <td>WeChat</td>
    <td>WeChat MP🔥🔥🔥</td>
    <td><a target="_blank" href="https://shang.qq.com/wpa/qunwpa?idkey=0f2bf59f5f2edec6a4550c364242c0641f870aa328e468c4ee4b7dbfb392627b"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="mss-boot技术交流群" title="mss-boot技术交流群"></a></td>
    <td>bilibili🔥🔥🔥</td>
  </tr>
</table>

## 💎 Contributors

<span style="margin: 0 5px;" ><a href="https://github.com/lwnmengjing" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/12806223?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/wangde7" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/56955959?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/mss-boot" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/109259065?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>
<span style="margin: 0 5px;" ><a href="https://github.com/wxip" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/25923931?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>

## JetBrains open source certificate support

The `mss-boot-io` project has always been developed in the GoLand integrated development environment under JetBrains, based on the **free JetBrains Open Source license(s)** genuine free license. I would like to express my gratitude.

<a href="https://www.jetbrains.com/?from=kubeadm-ha" target="_blank"><img src="https://raw.githubusercontent.com/panjf2000/illustrations/master/jetbrains/jetbrains-variant-4.png" width="250" align="middle"/></a>

## 🤝 Special thanks

1. [ant-design](https://github.com/ant-design/ant-design)
2. [ant-design-pro](https://github.com/ant-design/ant-design-pro)
3. [umi](https://umijs.org)
4. [gin](https://github.com/gin-gonic/gin)
5. [casbin](https://github.com/casbin/casbin)
6. [gorm](https://github.com/jinzhu/gorm)
7. [gin-swagger](https://github.com/swaggo/gin-swagger)
8. [jwt-go](https://github.com/dgrijalva/jwt-go)
9. [oauth2](https://pkg.go.dev/golang.org/x/oauth2)

## 🤟 Sponsor Us

If you think this project helped you, you can buy a glass of juice for the author to show encouragement 🍹

## Testing

The project follows strict testing requirements to ensure code quality and reliability.

### Local Test Prerequisites

`make test` runs `go test -coverprofile=coverage.out ./...`. Before opening a backend PR, check these local prerequisites:

- Use Go 1.26+, matching `go.mod` and GitHub Actions.
- Run `make deps` once after pulling dependency or `go.sum` changes.
- Redis-backed tests generally use `miniredis`, but a local Redis 7 instance is useful when validating cache/session behavior manually.
- No real production DSN, token, Kubernetes cluster, or private credential is required for `make test`.
- CI starts Redis 7 with `supercharge/redis-github-action`, then runs `make deps`, `make test`, and `make build`.

If a local test fails because an optional external service is unavailable, mention that in the PR verification notes and include the exact command output. Do not paste real credentials or production endpoints.

### Test Types

#### 1. Unit Tests
- **Location**: `*_test.go` alongside source files
- **Minimum coverage**: **80%**
- **Run command**: 
  ```bash
  go test ./... -v -coverprofile=coverage.out
  ```
- **Verify coverage**:
  ```bash
  go tool cover -html=coverage.out
  # Or check terminal summary
  go tool cover -func=coverage.out | grep total
  ```

#### 2. Integration Tests
- **Focus**: API endpoints with database interactions
- **Test databases**: Use test databases (SQLite in-memory for unit tests, real DB for integration)
- **Run command**:
  ```bash
  go test -tags=integration ./...
  ```
- **Verify**: Database migrations, API contracts, and service integrations

#### 3. End-to-End (E2E) Tests
- **Full Stack Testing**: Uses Playwright for browser automation
- **Critical user flows**: login, CRUD operations, permissions
- **Run command**: `pnpm e2e` (executed in frontend project)

### Development Workflow

```text
1. DEVELOPMENT PHASE
   └── Write code
   └── Write tests (TDD preferred)
   └── Ensure compilation

2. TESTING PHASE (MANDATORY)
   ├── Unit Tests: go test ./...
   ├── Integration Tests: go test -tags=integration ./...
   └── E2E Tests: pnpm e2e (for major features)

3. VERIFICATION PHASE
   ├── Check test coverage (≥80%)
   ├── All tests must pass
   └── Document test results
```

### Coverage Requirements by Component

| Component | Unit Tests | Integration Tests | Min Coverage |
|-----------|-----------|-------------------|--------------|
| Models | ✅ Required | Optional | 80% |
| Services | ✅ Required | ✅ Required | 85% |
| APIs | ✅ Required | ✅ Required | 80% |
| Utils | ✅ Required | Optional | 90% |

### Test Structure Example

```go
// service/user_test.go
package service

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestUserService_Create(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    svc := &UserService{}
    
    // Execute
    user, err := svc.Create(ctx, &CreateUserRequest{...})
    
    // Verify
    assert.NoError(t, err)
    assert.NotNil(t, user)
    assert.Equal(t, "expected", user.Field)
}
```

### Pre-commit Hooks
Before committing, run:
```bash
# Run all tests
go test ./... -v -race -coverprofile=coverage.out

# Check coverage meets requirements
go tool cover -func=coverage.out | grep total # Must be ≥ 80.0%
```

<img class="no-margin" src="https://mss-boot-io.github.io/.github/images/sponsor-us.jpg"  height="400px"  alt="Sponsor Us">

## 🔑 License

[MIT](https://github.com/mss-boot-io/mss-boot-admin/blob/main/LICENSE)

Copyright (c) 2024 mss-boot-io
