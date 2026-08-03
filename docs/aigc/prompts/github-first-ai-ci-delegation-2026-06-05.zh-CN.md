# GitHub-first AI 与 CI 委派记忆

- 日期：2026-06-05
- 决策：开源组织优先使用 GitHub 原生能力承接重复劳动，包括 Actions、Dependabot、CodeQL、govulncheck、Scorecard、PR Guard、Docs Drift，以及可用时的 Copilot code review、Copilot coding agent、Copilot issue drafting、Copilot Autofix。
- 原则：能由 GitHub workflow 稳定完成的检查和低风险自动化，落到 workflow；需要组织策略、Copilot 订阅、Actions minutes 或安全设置的能力，先记录为外部配置项，等待维护者确认。
- 边界：AI 和流水线可以做检查、总结、草拟、建议、低风险实现；维护者保留架构、安全、发布、合并和对外沟通的最终判断。
- 当前落地：本轮已将多个仓库的 Actions runtime 升级到 Node 24-compatible major，并通过 PR/main checks 验证；前端 Cloudflare alpha/beta 发布仍保持手动。
- 后续待确认：是否开启/配置 Copilot automatic code review、Copilot coding agent issue assignment、CodeQL Copilot Autofix、private vulnerability reporting、GitHub Discussions。

