# 2026-06-05 前端 CI 失败复盘

## 现象

`mss-boot-admin-antd` 的 Dependabot PR 大量触发 `(alpha)Deploy To Cloudflare`，但 Dependabot 无法读取仓库 secret，导致 Cloudflare 发布失败。`beta` workflow 也配置为 main push 自动发布 Cloudflare，与“前端不高频发布、可对外展示后再发布”的约束冲突。CI、CodeQL 等基础检查本身不是主要问题。

## 处理

- 将 alpha 和 beta Cloudflare workflow 改为 `workflow_dispatch`，发布由维护者确认后手动触发。
- Scorecard 权限从 workflow 顶层收窄到 job 级别。

## 验证

- `go run github.com/rhysd/actionlint/cmd/actionlint@latest`
- `git diff --check`

## 后续

前端仍遵循低频 Cloudflare 发布策略：本地开发和 CI 先验证，只有达到可对外展示状态后才手动发布到 alpha/beta/prod。
