# mss-boot

---
<img align="right" width="32" src="https://docs.mss-boot-io.top/favicon.ico"  alt="https://github.com/mss-boot-io/mss-boot"/>


[![ci](https://github.com/mss-boot-io/mss-boot/actions/workflows/ci.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot/actions/workflows/ci.yml)
[![CodeQL](https://github.com/mss-boot-io/mss-boot/actions/workflows/codeql.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://github.com/mss-boot-io/mss-boot/actions/workflows/scorecard.yml/badge.svg)](https://github.com/mss-boot-io/mss-boot/actions/workflows/scorecard.yml)
[![Release](https://img.shields.io/github/v/release/mss-boot-io/mss-boot.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot/releases)
[![License](https://img.shields.io/github/license/mss-boot-io/mss-boot.svg?style=flat-square)](https://github.com/mss-boot-io/mss-boot/blob/main/LICENSE)

[English](https://github.com/mss-boot-io/mss-boot/blob/main/README.md) | 简体中文

支持grpc、http协议的企业级语言异构微服务解决方案，单服务代码框架坚持极简的原则，同时提供完善的devops流程支撑(gitops)

## 📦 当前版本: v0.7.3

[在线文档](https://docs.mss-boot-io.top)

[贡献指南](./CONTRIBUTING.md) · [安全策略](./SECURITY.md) · [新手任务](https://github.com/mss-boot-io/mss-boot/issues?q=is%3Aissue%20is%3Aopen%20label%3A%22good%20first%20issue%22)

[微服务集合](https://github.com/mss-boot-io/mss-boot-monorepo)

## ✨ 特性
> - 遵循 RESTful API 设计规范
> - 登录支持idp(dex)
> - 支持 Swagger 文档(基于swaggo)
> - AI 可读契约、发布治理与可观测能力
> - 标准化错误处理与 Action Scope 上下文治理
> - GORM 查询缓存标签失效、Mongo ObjectID 安全校验等稳定性增强
> - 完善的cicd配套

## todo list
> - [ ] 支持租户
> - [ ] 支持dynamodb
> - [x] 支持config provider
> - [ ] 支持istio链路追踪
> - [ ] 开箱即用支持

## 🧭 Action/Controller 术语

mss-boot 的请求流程会反复出现以下概念：

- **Controller**：实现 `pkg/response.Controller` 的控制器，负责路由路径、路由级 Gin handlers，以及按 action 名称取得具体 `Action`。`pkg/response/controller.Simple` 会根据 GORM、MGM 或 Kubernetes 模型 provider 选择对应 action。
- **Action**：`get`、`search`、`control`、`delete` 等具名请求动作。Action 实现 `response.Action`，返回 Gin handler chain，并封装该动作在具体 provider 下的处理逻辑。
- **Hook**：挂在生命周期节点上的回调，例如 `BeforeCreate`、`AfterUpdate`、`BeforeSearch`，配置源的 `PrefixHook` / `PostHook`，以及服务启动或关闭回调。
- **Scope**：通常通过 `WithScope` 注入的按请求查询过滤器，会把当前 Gin context 和模型表转换成 GORM scope。常用于租户、归属关系或其他上下文约束。
- **Provider**：用于选择后端实现的枚举或 `Stringer`。控制器常见的是 `ModelProviderGorm`、`ModelProviderMgm`、`ModelProviderK8S`；配置源与存储包也有各自的 provider。

## 🧪 本地验证

提交 PR 前建议先在仓库根目录运行：

```bash
# 必须通过的本地测试入口
go test ./...

# 覆盖率报告
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

# 集成测试，需要先配置对应外部服务
go test -tags=integration ./...
```

依赖数据库、对象存储、Kubernetes、云凭证或其他可选外部服务的集成类测试，在配置缺失时应主动 skip，而不是让普通本地验证失败。

Lint 流程与 CI 保持一致：

```bash
# 未安装工具时先安装
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# 本地 lint 扫描
golangci-lint run ./...
```

GitHub Actions 会在仓库根目录运行 action 提供的 latest `golangci-lint`。如果本地二进制由低于 `go.mod` 目标版本的 Go 构建，请先重新安装再扫描。当前 lint job 用于提示历史 lint backlog，`go test ./...` 仍是必须通过的 CI 检查。

## 🔧 快速开始

### 环境要求
- Go 1.26+

### 使用 Go Modules
```bash
go get github.com/mss-boot-io/mss-boot@v0.7.3
```

### 本地检查
```bash
make tidy
make test
make coverage
make lint
```

## 请我喝杯咖啡
<a href="https://www.buymeacoffee.com/lwnmengjing" target="_blank"><img src="https://cdn.buymeacoffee.com/buttons/v2/default-yellow.png" alt="Buy Me A Coffee" style="height: 60px !important;width: 217px !important;" ></a>


## JetBrains 开源证书支持

`mss-boot-io` 项目一直以来都是在 JetBrains 公司旗下的 GoLand 集成开发环境中进行开发，基于 **free JetBrains Open Source license(s)** 正版免费授权，在此表达我的谢意。

<a href="https://www.jetbrains.com/?from=kubeadm-ha" target="_blank"><img src="https://raw.githubusercontent.com/panjf2000/illustrations/master/jetbrains/jetbrains-variant-4.png" width="250" align="middle"/></a>

## 🔑 License

[MIT](https://raw.githubusercontent.com/mss-boot-io/mss-boot/main/LICENSE)

Copyright (c) 2022 mss-boot-io
