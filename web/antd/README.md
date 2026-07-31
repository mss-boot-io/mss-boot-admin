# mss-boot-admin-antd

[![CI](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/ci.yml) [![CodeQL](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/codeql.yml/badge.svg?branch=main)](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/codeql.yml) [![OpenSSF Scorecard](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/scorecard.yml/badge.svg?branch=main)](https://github.com/mss-boot-io/mss-boot-admin-antd/actions/workflows/scorecard.yml) [![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot-admin-antd.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin-antd/releases) [![License](https://img.shields.io/github/license/mss-boot-io/mss-boot-admin-antd.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin-antd/blob/main/LICENSE)

English | [简体中文](./README.zh-CN.md)

## Introduction

> `mss-boot-admin-antd` is the frontend of the mss-boot admin platform. The product is being repositioned around governance, operations, access control, configuration, and AI-annotation-assisted engineering collaboration, rather than dynamic-model or generator-led workflows.

## Recent Updates

The frontend has undergone comprehensive polish rounds focusing on:

- **Stability**: Fixed navigation redirect paths, password reset logic, polling cleanup
- **Code Quality**: Eliminated all TypeScript errors (0 errors), fixed import paths, removed duplicate locale keys
- **Abstraction**: Created AuthShell component for auth pages, useMonitorData hook for monitoring, unified page structures
- **Documentation**: Added CHANGELOG documenting all frontend changes

## Tutorial

[Online documentation](https://docs.mss-boot-io.top) [Video tutorial](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)

## Community

[Contributing](./CONTRIBUTING.md) [Security](./SECURITY.md) [Support](./SUPPORT.md) [Good first issues](https://github.com/mss-boot-io/mss-boot-admin-antd/issues?q=is%3Aissue%20is%3Aopen%20label%3A%22good%20first%20issue%22) [AI memory](./aigc/prompts/readme.zh-CN.md)

## Project address

[Backend project](https://github.com/mss-boot-io/mss-boot-admin) [Front-end project](https://github.com/mss-boot-io/mss-boot-admin-antd)

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

## 📦 Preparation

- Backend: Go 1.26+
- Optional backend integration dependencies: MySQL 8.0+ and Redis 7+
- Frontend: Node.js 22+ and pnpm 9.x

## 📦 Quick start

### 1. Download the project

```shell
# Download the backend project
git clone https://github.com/mss-boot-io/mss-boot-admin.git
# Download the front-end project
git clone https://github.com/mss-boot-io/mss-boot-admin-antd.git
```

### 2. Migrate the database

```shell
# Enter the backend project
cd mss-boot-admin
# The default local backend config uses SQLite: mss-boot-admin-local.db
go run . migrate
```

To use MySQL locally, start `compose/mysql/docker-compose.yml` in the backend repository and update `config/application.yml` before running migrations.

### 3. Generate API interface information

```shell
# Generate API interface information
go run . server -a
```

### 4. Start the backend service

```shell
# Start the backend service
go run . server
```

### 5. Start the front-end service

```shell
# Enter the front-end project
cd mss-boot-admin-antd
# Install dependencies
corepack enable
pnpm install
# Start the front-end service
pnpm dev
```

## Frontend Environment Matrix

The frontend uses `UMI_ENV` and `REACT_APP_ENV` to choose the environment-specific config. `API_URL` is defined in the matching `config/config.prod.*.ts` file or by the dev proxy.

| Context | Command | API target | Usage |
| --- | --- | --- | --- |
| Local development | `pnpm dev` or `pnpm start:no-mock` | Dev proxy to `http://localhost:8080` for `/admin/` and `/public/` | Use with a local `mss-boot-admin` backend. |
| Local build | `pnpm build:local` | `http://localhost:8080` | Validate a production-style bundle against a local backend. |
| Alpha | `pnpm start:alpha` / `pnpm build:alpha` | `https://admin-api-alpha.mss-boot-io.top` | Development backend environment for integration checks. |
| Beta | `pnpm start:beta` / `pnpm build:beta` | `https://admin-api-beta.mss-boot-io.top` | Public beta target after local and CI verification. |
| Production | `pnpm start:prod` / `pnpm build:prod` | `https://admin-api.mss-boot-io.top` | Production release build target. |

CI and Cloudflare workflows use Node.js 22 and pnpm 9. Cloudflare alpha, beta, and production workflows are manual `workflow_dispatch` deployments; PRs, Dependabot branches, and normal `codex/**` review branches should not publish frontend changes.

## 📨 Interaction

<table>
   <tr>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/wechat-mp.jpg" width="180px"></td>
    <td><img src="https://mss-boot-io.github.io/.github/images/qq-group.jpg" width="200px"></td>
    <td><a href="https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026&ctype=0">mss-boot-io</a></td>
  </tr>
  <tr>
    <td>WeChat</td>
    <td>WeChat MP🔥🔥🔥</td>
    <td><a target="_blank" href="https://shang.qq.com/wpa/qunwpa?idkey=0f2bf59f5f2edec6a4550c364242c0641f870aa328e468c4ee4b7dbfb392627b"><img border="0" src="https://pub.idqqimg.com/wpa/images/group.png" alt="mss-boot技术交流群" title="mss-boot技术交流群"></a></td>
    <td>bilibili🔥🔥🔥</td>
  </tr>
</table>

## 💎 Contributors

<span style="margin: 0 5px;" ><a href="https://github.com/lwnmengjing" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/12806223?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span> <span style="margin: 0 5px;" ><a href="https://github.com/wangde7" ><img src="https://images.weserv.nl/?url=avatars.githubusercontent.com/u/56955959?s=64&v=4&w=60&fit=cover&mask=circle&maxage=7d" /></a></span>

## JetBrains open source certificate support

The `mss-boot-io` project has always been developed in the GoLand integrated development environment under JetBrains, based on the **free JetBrains Open Source license(s)** genuine free license. I would like to express my gratitude.

<a href="https://www.jetbrains.com/?from=kubeadm-ha" target="_blank"><img src="https://raw.githubusercontent.com/panjf2000/illustrations/master/jetbrains/jetbrains-variant-4.png" width="250" align="middle"/></a>

## 🤝 Special thanks

1. [ant-design](https://github.com/ant-design/ant-design)
2. [ant-design-pro](https://github.com/ant-design/ant-design-pro)
3. [gin](https://github.com/gin-gonic/gin)
4. [casbin](https://github.com/casbin/casbin)
5. [gorm](https://github.com/jinzhu/gorm)
6. [gin-swagger](https://github.com/swaggo/gin-swagger)
7. [jwt-go](https://github.com/dgrijalva/jwt-go)
8. [oauth2](https://pkg.go.dev/golang.org/x/oauth2)

## Testing

The project follows strict testing requirements with comprehensive test coverage for both desktop and mobile platforms.

### Test Types

#### 1. Unit Tests

- **Location**: `__tests__/*.test.ts` or `*.test.tsx`
- **Minimum coverage**: **80%**
- **Run command**:
  ```bash
  pnpm test --coverage
  ```

#### 2. Integration Tests

- **Focus**: Component interactions with API mocks
- **Tools**: React Testing Library + Mock Service Worker (MSW)
- **Run command**:
  ```bash
  pnpm test
  ```

#### 3. End-to-End (E2E) Tests

- **Full Stack Testing**: Uses Playwright for browser automation
- **Critical user flows**: login, CRUD operations, permissions
- **Mobile testing**: Comprehensive iPhone 12 Pro viewport tests
- **Run command**:
  ```bash
  pnpm run test:e2e
  ```

### Coverage Requirements

| Component  | Unit Tests  | Integration Tests | E2E Tests | Min Coverage |
| ---------- | ----------- | ----------------- | --------- | ------------ |
| Hooks      | ✅ Required | Optional          | N/A       | 80%          |
| Components | ✅ Required | Optional          | Optional  | 75%          |
| Utils      | ✅ Required | Optional          | N/A       | 90%          |

Detailed testing instructions are available in [TESTING.md](./TESTING.md).

## Mobile H5 Adaptation

The application features comprehensive mobile H5 adaptation with responsive design and mobile-specific components.

### Key Features

- **Responsive Detection**: Automatic detection using Ant Design breakpoints via `useResponsive` hook
- **Mobile Components**: Dedicated mobile components in `/src/pages/*/Mobile/` directories
- **Mobile Navigation**: Bottom tab bar navigation optimized for touch interfaces
- **Viewport Handling**: Optimized layouts for iPhone 12 Pro (390x844) viewport
- **Touch Optimization**: Touch-friendly UI elements and gestures

### Mobile Architecture

- **Desktop vs Mobile**: Components automatically switch between desktop and mobile layouts based on screen size
- **Component Structure**: Each major page has both desktop (`/src/pages/User/List.tsx`) and mobile (`/src/pages/User/Mobile/List.tsx`) versions
- **Styling**: Mobile-specific CSS in `src/styles/mobile.less`
- **Performance**: Optimized asset loading and rendering for mobile devices

### Development Guidelines

- **Testing Mobile**: Use `pnpm run test:e2e -- --project='iPhone 12 Pro'` to run mobile-specific tests
- **Responsive Breakpoints**: Mobile layout activates below 768px width
- **Touch Targets**: Ensure all interactive elements are at least 44px for touch accessibility

For detailed mobile development documentation, see the full documentation site.

## 🔑 License

[MIT](https://github.com/mss-boot-io/mss-boot-admin-antd/blob/main/LICENSE)

Copyright (c) 2024 mss-boot-io
