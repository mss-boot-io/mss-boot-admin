# Actions runtime 升级记忆

- 日期：2026-06-05
- 背景：组织主干流水线恢复全绿后，GitHub Actions 仍显示 Node.js 20 runtime 与 CodeQL Action v3 弃用提示。文档仓库还承担公开文档部署，因此需要提前消除未来流水线风险。
- 处理：将 `actions/checkout`、`actions/setup-node`、`actions/setup-go`、`pnpm/action-setup` 升级到 `v6`，将 `github/codeql-action/init|analyze|upload-sarif` 升级到 `v4`。
- 约束：文档部署保持原触发规则；本次不改 Cloudflare 凭据、不改发布域名。
- 验收：本地 `pnpm build` 与 PR checks / main checks 双重确认，记忆落盘在 docs 仓库，避免多 git workspace 中丢失上下文。
