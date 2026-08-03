# package metadata / Node 22 基线记忆

- 日期：2026-06-09
- 仓库：mss-boot-admin-antd
- 背景：前端 CI、Cloudflare 发布流水线、Copilot setup 均已使用 Node 22 与 pnpm 9，`package.json` 仍声明 `node >=12.0.0`，且项目描述继续突出已经降级、即将废弃的“虚拟模型”能力。
- 决策：
  - `packageManager` 固定为 `pnpm@9.15.9`，与 lockfile v9 和流水线一致。
  - `engines.node` 调整为 `>=22.0.0`，`engines.pnpm` 调整为 `>=9.0.0`。
  - package 描述改为开源治理控制台视角，突出 RBAC、组织/菜单/API 治理、配置、国际化、可观测性和 public beta 工作流。
- CI 修正：
  - `pnpm/action-setup@v6` 会同时读取 workflow `version` 与 package.json `packageManager`。
  - 两处同时声明会触发 `Multiple versions of pnpm specified`，因此 CI、Cloudflare、Release、Copilot setup 均移除显式 `version: 9`，统一从 `packageManager` 解析 `pnpm@9.15.9`。
- 约束：
  - 虚拟模型是 legacy/degraded 功能，不再作为前端对外主卖点。
  - 前端不能高频发布到 Cloudflare；本地开发先对接 alpha 后端，完整验证后再发布 beta/prod。
  - 多 git workspace 的组织级记忆仍必须同步落盘到 mss-boot-docs/aigc，避免线程压缩或仓库切换后丢失。
