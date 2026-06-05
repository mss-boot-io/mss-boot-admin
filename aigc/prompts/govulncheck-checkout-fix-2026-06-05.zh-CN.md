# 2026-06-05 govulncheck checkout 修复记忆

## 现象

Dependabot 创建 `actions/checkout` 升级 PR 后，`govulncheck` 失败在 `golang/govulncheck-action@v1` 内部 checkout 阶段，日志出现 `Duplicate header: "Authorization"`，不是漏洞扫描结果失败。

## 处理

- workflow 已经先执行 `actions/checkout`。
- 给 root `golang/govulncheck-action@v1` 增加 `repo-checkout: false`，避免 action 内部重复 checkout。
- submodule govulncheck 仍使用已 checkout 的本地工作区执行。

## 验证

- `go run github.com/rhysd/actionlint/cmd/actionlint@latest`

## 后续

如果同类问题在其他 Go 仓库出现，优先采用外部 checkout + `repo-checkout: false` 的模式。
