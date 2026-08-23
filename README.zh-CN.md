# mss-boot-admin

[![CI](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot-admin/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/releases)
[![License](https://img.shields.io/github/license/mss-boot-io/mss-boot-admin.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot-admin/blob/main/LICENSE)

[English](./README.md) | 简体中文

`mss-boot-admin` 是一套 Agent 原生的管理系统基础设施。它在同一个源码仓库中
整合了面向生产的 Go Admin、React 19 + Ant Design 6 应用、机器可读项目契约、
确定性全栈生成、变更感知验证和可持续升级的 Thin Host Blueprint。

当前稳定版是 **v1.3.2**，已从精确 merged-main 提交
`635fbb03a82976941e527d8ac1000fec0624abac` 完整发布并完成公开对账。已公开的
`mss-boot/v1.3.1` 与失败的 `admin/v1.3.1`
资格验证属于永久不可变的部分发布历史；它们不能替代或移动为完整 v1.3.2 制品。

## 完整 Admin 发行包

v1.3.2 统一版本在同一源码提交上协调以下可独立发布的组件：

| 组件 | 稳定版标识 |
| --- | --- |
| Foundation 工具与根交付物 | `v1.3.2` |
| 可复用 Go Framework | `mss-boot/v1.3.2` |
| 可导入 Admin Go Module | `admin/v1.3.2` |
| 完整 Admin Web 包 | `web/antd-v6/v1.3.2` / `@mss-boot-io/admin-web@1.3.2` |
| 文档 | 使用 `docs/vX.Y.Z` 独立发布 |

身份、浏览器 Session、RBAC、菜单、布局、国际化和公共运行时仍然属于同一套 Admin
产品。下游应用是 Thin Host：它锁定完整后端和前端，只增加自有业务模块与组合胶水，
最终仍然只产生一个后端二进制、一个前端 `dist`、一套 Session 边界和一套权限/菜单模型。

运行时动态模型、虚拟 CRUD 和浏览器代码生成已经移除。新的业务能力通过 Feature 与
AdminModule 契约描述，并在开发期确定性生成。

```text
业务意图
  -> Feature 与 Acceptance 契约
  -> AdminModule 契约
  -> 确定性生成后端、迁移、权限、菜单、前端与测试
  -> 实现自有业务规则
  -> 变更感知验证与 Agent Evals
  -> 可审查 PR 与可持续升级的 Thin Host
```

## 引用 v1.3.2

稳定版标签已经公开。Go 使用方应锁定完全一致的版本；Admin Module 不包含本地
`replace`：

```shell
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.2
go get github.com/mss-boot-io/mss-boot-admin/admin@v1.3.2
```

完整前端公开发布到 npmjs，Thin Host 默认安装不需要 Registry Token：

```shell
corepack pnpm@10.34.5 add --save-exact @mss-boot-io/admin-web@1.3.2
```

GitHub Packages 保留完全相同的不可变 tarball 作为兼容镜像。只有明确选择该镜像的
使用方才需要在安装进程中临时注入 `read:packages` Token，绝不能把 Token 写入仓库。

稳定包使用 `latest` 分发标签，预发布包使用 `next`。生成的 Thin Host 仍会锁定精确
版本，而不会依赖移动标签。官方 npm 发布是最后一次制品发布，必须等所有协调组件和
Docs Release 都解析到同一个 merged-main 提交后才执行。v1.3.2 的 npm `latest` 已指向
`1.3.2`；后续发布通过绑定 `mss-boot-io/mss-boot-admin`、`npm-release.yml` 与
`release-v6` 的 npm Trusted Publishing 完成，不再保留 bootstrap npm Token 或 GitHub
`NPM_TOKEN` Secret。

## 创建与升级 Thin Host

从干净的 Foundation checkout 创建宿主。随后初始化并校验纳入版本控制的模块
规格，再生成第一个垂直业务模块：

```shell
go run ./cmd/mss new app orders-admin \
  --module github.com/acme/orders-admin \
  --repository acme/orders-admin \
  --destination ../orders-admin \
  --write \
  --format json

go run ./cmd/mss --root ../orders-admin spec init supplier \
  --kind module \
  --output .mss/modules/supplier.yaml \
  --write

go run ./cmd/mss --root ../orders-admin spec validate \
  .mss/modules/supplier.yaml \
  --format json

go run ./cmd/mss --root ../orders-admin module generate \
  .mss/modules/supplier.yaml \
  --write \
  --frontend-target antd-v6 \
  --format json
```

应用 Admin 发行包升级之前，必须先生成只读计划：

```shell
go run ./cmd/mss --root ../orders-admin upgrade admin v1.3.2 \
  --foundation . \
  --format json

go run ./cmd/mss --root ../orders-admin upgrade admin v1.3.2 \
  --foundation . \
  --apply --yes \
  --format json
```

三方升级引擎只自动修改 Blueprint 管理的宿主文件；未知文件和业务自有文件会被保留，
有冲突的计划必须人工审查，不能自动应用。

## 本地开发

环境要求：

- Go 1.26.6 或同一 1.26 版本线的更高版本；
- Node.js 24；
- 通过 Corepack 使用 pnpm 10.34.5；
- MySQL、PostgreSQL、Redis 只在验证对应可选集成时需要。

```shell
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin

go run ./cmd/mss doctor
go run ./cmd/mss setup
(cd admin && STAGE=local go run . server -a)
go run ./cmd/mss dev --detach
go run ./cmd/mss dev status --format json
```

Admin Web 开发服务监听 `http://localhost:8001`，并把 `/admin/` 代理到 Go 后端。
一次性的 `server -a` 会把已挂载路由同步进 API 注册表；如果跳过它，即使前后端都健康，
菜单的“绑定 API”列表也可能为空。

## 产品能力

- 身份、HttpOnly 浏览器 Session、OAuth 账号绑定、PAT 生命周期与在线 Session 撤销；
- 基于 Casbin 的 RBAC、组织与数据范围、菜单/API 绑定，以及后端默认拒绝的鉴权边界；
- 用户、角色、菜单、API、部门、岗位、选项、语言、配置、通知、任务、审计、存储、
  监控与统计模块；
- React Query 服务端状态、同步的中英文、响应式明暗主题，以及权限、加载、空、冲突、
  错误等完整页面状态；
- 确定性全栈模块生成、可升级迁移、仓库级 Skills、Agent Evals 与外部消费者验证。

浏览器 JavaScript 不接触 Admin JWT 或第三方凭证。浏览器使用 HttpOnly
`mss_admin_session` Cookie、签名且绑定 Session 的 CSRF Token 和一次性 WebSocket
票据。标准 Bearer 与 PAT 仍用于有明确文档的非浏览器 API 自动化。

## 仓库结构

| 路径 | 职责 |
| --- | --- |
| `/`、`cmd/mss/`、`internal/mss/` | Agent CLI、编排、生成、验证与升级 |
| `.mss/` | 项目、能力、发布、模块与评测的机器可读契约 |
| `admin/` | 完整、可复用且可部署的 Admin Go 应用 |
| `mss-boot/` | 领域无关的可复用 Go Framework |
| `web/antd-v6/` | 官方 Admin Web 应用与完整 npm 包 |
| `templates/` | 确定性应用和模块模板 |
| `docs/` | 产品、架构、运维与贡献者文档 |

## 验证

优先使用仓库契约，而不是工作站特有命令：

```shell
go run ./cmd/mss context
go run ./cmd/mss verify --changed
go run ./cmd/mss verify --all
```

也可按组件执行真实的聚焦验证：

```shell
make test-framework
make test-all

corepack pnpm@10.34.5 --dir web/antd-v6 run deps:check
corepack pnpm@10.34.5 --dir web/antd-v6 run lint
corepack pnpm@10.34.5 --dir web/antd-v6 run test:ci
corepack pnpm@10.34.5 --dir web/antd-v6 run build:release
corepack pnpm@10.34.5 --dir web/antd-v6 run test:e2e

make docs-build
```

Playwright 仅在浏览器契约变化或正式资格验证时运行。测试不需要生产凭证；只有当 CI
执行同一阈值时，项目才会声明覆盖率百分比。

## 文档与社区

- [在线文档](https://docs.mss-boot-io.top)
- [OpenAPI 文档](https://mss-boot-io.github.io/mss-boot-admin/swagger.json)
- [版本发布](https://github.com/mss-boot-io/mss-boot-admin/releases)
- [安全策略](./SECURITY.md)
- [贡献指南](./CONTRIBUTING.md)
- [视频教程](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)

## License

[MIT](./LICENSE)

Copyright (c) 2024-2026 mss-boot-io
