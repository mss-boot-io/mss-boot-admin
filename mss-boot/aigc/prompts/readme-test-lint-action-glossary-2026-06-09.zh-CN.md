# README 本地验证与 Action/Controller 术语记忆

- 日期：2026-06-09
- 关联 issue：#348、#349
- 变更范围：仅更新 `README.md`、`README.Zh-cn.md` 与 `aigc/` 记忆文件；不改 Go 业务代码、不改 CI 行为。

## 本地验证口径

- README 需要明确 `go test ./...` 是本地与 CI 的必过测试入口。
- Lint 文档对齐 `.github/workflows/ci.yml`：CI 使用 `golangci/golangci-lint-action@v9`、`version: latest`，在仓库根目录运行 `golangci-lint`；本地可用 `golangci-lint run ./...` 复现，安装命令使用 v2 模块路径 `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`。
- CI lint job 仍是 advisory，用于提示历史 lint backlog；不要把文档写成 lint 必须全绿才可合并。
- 依赖数据库、对象存储、Kubernetes、云凭证等可选外部服务的集成类测试，在配置缺失时应 skip，避免给普通本地验证增加新服务要求。

## 术语表口径

- `Controller`：`pkg/response.Controller`，负责 route path、Gin handlers、按名称返回 `Action`，并暴露 `GetProvider()`。
- `Action`：`response.Action`，包含 `String()` 和 `Handler() gin.HandlersChain`；常见名称来自 `pkg/response/action.go`，如 `get`、`search`、`control`、`delete`。
- `Hook`：生命周期回调，包括 GORM/K8S actions 的 before/after hooks、配置源 `PrefixHook` / `PostHook`、server start/end hooks。
- `Scope`：请求上下文相关的查询过滤器，主要是 `WithScope` 提供的 GORM scope，也包括 virtual model 中的 table/tenant/URI/search/pagination scopes。
- `Provider`：选择后端实现的枚举或 `Stringer`，如 `ModelProviderGorm`、`ModelProviderMgm`、`ModelProviderK8S`，以及配置源/存储包里的 provider。

## 后续维护提醒

- 如果 CI 从 advisory lint 改成 required，README 中的 lint 状态说明需要同步更新。
- 如果新增 `create` / `update` 等默认 controller action，术语表中的示例 action 名称可以补齐。
