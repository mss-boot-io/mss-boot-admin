# GitHub Copilot Setup Steps

日期：2026-06-05

## 背景

`mss-boot-admin` 后端可以较频繁发布到 beta 环境，适合把低风险、可验证的
修复任务逐步交给 GitHub Copilot coding agent 处理。

## 已落地

- 新增 `.github/workflows/copilot-setup-steps.yml`。
- 工作流只在手动触发、或该 workflow 文件变更时运行。
- 代理环境步骤包括 checkout、`actions/setup-go@v6` 和 `go mod download`。

## 约束

- 该 workflow 只做环境预热，不自动发布镜像。
- 后端镜像发布仍由现有 CI/release 流程控制。
- 数据库、Redis、beta/prod 环境变更必须通过已有部署记忆和发布流程确认。
