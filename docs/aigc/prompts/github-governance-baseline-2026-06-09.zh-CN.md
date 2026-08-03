# GitHub 治理基线与自动化记忆

- 日期：2026-06-09
- 范围：mss-boot-io 4 个核心开源仓库（mss-boot、mss-boot-admin、mss-boot-admin-antd、mss-boot-docs）
- 目标：把个人维护的开源组织尽量交给 GitHub 原生能力与 AI/流水线兜底，降低人工运营成本，同时保证社区贡献进入可 review、可测试、可追踪的链路。

## 已落地动作

- 仓库元数据：
  - 4 个仓库更新 description 和 topics，突出 governance-first、AI-ready、CI/CD、open-source/community 等定位。
  - mss-boot-admin-antd 与 mss-boot-docs 开启 Discussions，GitHub Discussions 作为社区运营主阵地。
- CODEOWNERS：
  - 4 个仓库补充 `.github/CODEOWNERS`。
  - 默认 owner 使用 `@mss-boot-io/admin`。
  - 前端仓库额外覆盖 `.github/**` 为 `@mss-boot-io/admin @mss-boot-io/frontend`。
- Branch protection：
  - main 分支要求 PR review、code owner review、1 个 approval、dismiss stale reviews、conversation resolution。
  - required checks 仅使用 PR 可触发的状态，避免把 push-only 任务放进 PR merge gate。
  - mss-boot：guard、docs-drift、test、lint、govulncheck、CodeQL(go/actions)。
  - mss-boot-admin：guard、build、govulncheck、CodeQL(go/actions)。
  - mss-boot-admin-antd：guard、docs-drift、build、CodeQL(javascript-typescript/actions)。
  - mss-boot-docs：guard、docs-drift、build、CodeQL(javascript-typescript/actions)。
- Labels：
  - 4 个仓库补充 `queue/review`、`queue/discussion`、`queue/feedback`，用于 AI/maintainer 分拣队列。
- Weekly Digest：
  - `.github` 组织级 weekly digest workflow 降低 GitHub API payload，避免 Node `ENOBUFS` 导致周报失败。
  - 失败仓库已 rerun 并恢复成功。
- Release workflow：
  - mss-boot 与 mss-boot-admin-antd 打开 release workflow 幂等化 PR。
  - 目标是避免 tag/release 重跑时因 release 已存在而失败。

## 仍需留意

- 3 个仓库的 branch protection API 回读仍显示 `allow_force_pushes.enabled=true`：
  - mss-boot
  - mss-boot-admin
  - mss-boot-admin-antd
- 已尝试使用 GitHub REST protection payload 关闭 force push，但回读未生效；后续可在 GitHub Web UI 或 Rulesets 再复核。
- mss-boot/mss-boot-admin-antd 的 release workflow 幂等化 PR 仍需要 code owner review 后合并，保持开源项目 review 语义，不建议默默绕过。

## 后续执行原则

- 后端 beta 环境可以高频发布验证；前端 Cloudflare beta/prod 有发布次数成本，必须本地开发与 alpha 后端充分验证后再公开发布。
- 外部社区反馈优先沉淀到 GitHub Discussions/Issues；国内外平台发帖用于引流，不作为需求与决策的唯一来源。
- AI agent 处理社区事务时优先做：
  - 标签分拣
  - 初步回复
  - docs/FAQ/RFC 落盘
  - PR Guard、Docs Drift、CodeQL、CI 失败修复
  - 可复现问题的 dev 环境验证
- 涉及账号、付费平台、对外发布频次或需要 maintainer 价值判断的动作，先写入 mss-boot-docs/aigc，等待人工决定。
