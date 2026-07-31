# GitHub Copilot Setup Steps

日期：2026-06-05

## 背景

前端不能高频发布到 Cloudflare，但仍可以把本地开发、PR 检查、类型问题和
小范围 UI 修复更多交给 GitHub Copilot coding agent 准备。

## 已落地

- 新增 `.github/workflows/copilot-setup-steps.yml`。
- 工作流只在手动触发、或该 workflow 文件变更时运行。
- 代理环境步骤包括 checkout、`pnpm/action-setup@v6`、
  `actions/setup-node@v6` 和 `pnpm install --frozen-lockfile`。
- 将 `.github/workflows/prod.yml` 调整为仅保留人工触发，避免 tag push
  自动触发 Cloudflare prod 发布。
- 在 `.github/workflows/cf.yml` 增加按环境分组的部署并发锁，避免同一
  Cloudflare 环境并发发布。
- 为 Dependabot 增加 npm minor/patch 分组和 GitHub Actions 分组，降低依赖
  PR 数量和 CI 噪音。

## 约束

- 该 workflow 只做环境预热，不触发 Cloudflare 发布。
- alpha/beta/prod Cloudflare 发布必须保持人工 `workflow_dispatch`。
- 真实对外环境发布前仍需本地开发验证、CI 通过和冒烟测试。
