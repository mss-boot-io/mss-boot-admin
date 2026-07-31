# GitHub Copilot Setup Steps

日期：2026-06-05

## 背景

`mss-boot-docs` 是组织级记忆中心。为了让 GitHub Copilot coding agent 在
处理文档、记忆、开源治理说明时更快进入可验证状态，需要预热文档项目依赖。

## 已落地

- 新增 `.github/workflows/copilot-setup-steps.yml`。
- 工作流只在手动触发、或该 workflow 文件变更时运行。
- 代理环境步骤包括 checkout、`pnpm/action-setup@v6`、
  `actions/setup-node@v6` 和 `pnpm install --frozen-lockfile`。
- docs 发布 workflow 增加 `workflow_dispatch`、文档相关 `push.paths` 过滤和
  `cloudflare-docs-prod` 并发锁，降低无关 main push 触发 Cloudflare 发布的
  概率。
- 为 Dependabot 增加 npm minor/patch 分组和 GitHub Actions 分组，降低依赖
  PR 数量和 CI 噪音。

## 约束

- 该 workflow 只做环境预热，不替代正式 docs CI。
- 文档站部署继续使用现有 docs workflow；部署触发应限于文档站相关变更或
  人工触发。
- 组织级流程、发布策略、社区运营和 AI 记忆必须继续落盘在 docs。
