# Actions runtime 升级记忆

- 日期：2026-06-05
- 背景：后台后端仓库主干流水线已恢复稳定，但 GitHub Actions 继续提示 checkout / CodeQL 相关 action 的 Node 20 runtime 与 CodeQL v3 弃用风险。
- 处理：将 `actions/checkout` 与 `actions/setup-go` 升级到 `v6`，将 `github/codeql-action/init|analyze|upload-sarif` 升级到 `v4`。
- 约束：只处理 CI 基础设施，不修改后端业务、部署配置或 beta/prod 环境。
- 验收：PR checks 全绿后再合并，合并后继续观察 main 的 CI、CodeQL、govulncheck、Scorecard、Mirror、Swagger。
