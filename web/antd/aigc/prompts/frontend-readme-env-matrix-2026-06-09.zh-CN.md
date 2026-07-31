# 前端 README 环境矩阵记忆

- 日期：2026-06-09
- 仓库：mss-boot-admin-antd
- 对应 issue：#71、#72

## 本轮处理

- README / README.zh-CN 的 workflow badges 增加 `branch=main`，明确展示本前端仓库主分支的 CI、CodeQL、OpenSSF Scorecard 状态。
- 准备工作从旧的 Node 18 / npm 描述调整为 Node.js 22 + pnpm 9，与当前 GitHub Actions、Cloudflare workflow 保持一致。
- 新增前端环境矩阵，覆盖 local、alpha、beta、prod 的命令、API 目标和使用边界。
- 明确 Cloudflare alpha/beta/prod 发布均为手动 `workflow_dispatch`，普通 PR、Dependabot、`codex/**` 分支不应触发前端发布。

## 注意

- `package.json` 的 Node/pnpm 元数据由安全依赖收敛 PR 单独处理；本轮只修 README 和本地记忆。
- 未包含 Cloudflare account、zone、token 等外部账号或私密信息。
