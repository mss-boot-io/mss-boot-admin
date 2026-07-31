# Actions runtime 升级记忆

- 日期：2026-06-05
- 背景：前端主干合并 README 社区信号后全绿，但 CI / CodeQL 中仍有 Node.js 20 runtime 和 CodeQL Action v3 的弃用提示。
- 处理：将 `actions/checkout`、`actions/setup-node`、`pnpm/action-setup` 升级到 `v6`，将 CodeQL 相关 action 升级到 `v4`。
- 发布约束：Cloudflare alpha/beta workflow 仍保持 `workflow_dispatch`，本次只是工作流 action 版本升级，不触发高频前端发布。
- 验收：PR checks 全绿后合并，合并后确认 main 只跑 CI/CodeQL/Mirror/Scorecard，不触发 Cloudflare alpha/beta。

