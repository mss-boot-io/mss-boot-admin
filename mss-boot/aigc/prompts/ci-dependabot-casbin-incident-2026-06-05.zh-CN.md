# 2026-06-05 mss-boot CI 失败复盘

## 现象

Dependabot 合并 `github.com/casbin/gorm-adapter/v3` 后，`main` 的 CI 和 govulncheck 编译失败。错误表现为 adapter 实现了 `casbin/v3/model.Model` 接口，但项目当前仍使用 `casbin/v2/persist.Adapter`。

## 处理

- 将 `github.com/casbin/gorm-adapter/v3` 固定回兼容 Casbin v2 的 `v3.38.0`。
- 在 Dependabot 配置里忽略该依赖的自动 minor/patch 升级，避免再次隐式引入 Casbin v3 接口。
- Scorecard 权限从 workflow 顶层收窄到 job 级别。
- mirror 模板 checkout 改为 `fetch-depth: 0`。

## 验证

- `go test ./...`
- `GOTOOLCHAIN=go1.26.4 go run golang.org/x/vuln/cmd/govulncheck@latest ./...`
- `go run github.com/rhysd/actionlint/cmd/actionlint@latest`
- `git diff --check`

## 后续

长期应评估是否整体迁移到 Casbin v3；迁移前不要让 adapter 自动升级到会改变接口 major 的版本。
