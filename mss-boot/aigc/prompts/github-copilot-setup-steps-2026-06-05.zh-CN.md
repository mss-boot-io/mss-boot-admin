# GitHub Copilot Setup Steps

日期：2026-06-05

## 背景

为了把低风险、边界清晰的实现任务更多交给 GitHub Copilot coding agent，
需要让 GitHub 代理在进入任务前自动准备 Go 环境和依赖缓存。

## 已落地

- 新增 `.github/workflows/copilot-setup-steps.yml`。
- 工作流只在手动触发、或该 workflow 文件变更时运行。
- 代理环境步骤包括 checkout、`actions/setup-go@v6` 和 `go mod download`。

## 约束

- 该 workflow 只做环境预热，不替代正式 CI。
- 正式质量门禁仍以 `ci`、CodeQL、govulncheck、Scorecard、PR Guard 和
  Docs Drift 为准。
