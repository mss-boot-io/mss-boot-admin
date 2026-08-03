# Actions runtime 升级记忆

- 日期：2026-06-05
- 背景：后端主干已恢复全绿，但 CodeQL / checkout 仍有 Node.js 20 runtime 与 CodeQL Action v3 的弃用提示。
- 处理：将后端仓库内的 `actions/checkout` 与 `actions/setup-go` 升级到 `v6`，将 `github/codeql-action/init|analyze|upload-sarif` 升级到 `v4`。
- 约束：这是 CI 体验与未来兼容性修复，不涉及业务代码、镜像发布或 beta 环境部署。
- 验证：通过 GitHub API 确认 `checkout@v6` 与 `codeql-action@v4` 标签存在，后续依赖 PR checks 和 main checks 验收。
