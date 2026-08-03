# docs 安全依赖收敛记忆 2026-06-11

## 背景

- 仓库：`mss-boot-io/mss-boot-docs`
- 触发来源：社区运营 heartbeat 巡检发现 docs 仓库仍有 15 个 open Dependabot alerts。
- 主要风险集中在 dumi -> Umi 4 工具链：
  - `vite@4.5.2`
  - `react-router@6.3.0`
  - `@babel/runtime@7.23.6`
  - `elliptic@6.6.1`

## 本轮处理

- 新建分支：`codex/fix-docs-vite-router-alerts-20260611`
- 修改 `package.json` 的 `pnpm.overrides`：
  - `@babel/runtime` -> `7.29.7`
  - `@vitejs/plugin-react@4.0.0` -> `4.7.0`
  - `react-router@6.3.0` -> `6.30.4`
  - `react-router-dom@6.3.0` -> `6.30.4`
  - `vite@4.5.2` -> `6.4.3`
- 更新 `pnpm-lock.yaml`，保持 lockfileVersion `9.0`，与 CI 的 pnpm 9 兼容。

## 验证结果

- `CI=true corepack pnpm@9.15.9 install --frozen-lockfile` 通过。
- `CI=true corepack pnpm@9.15.9 build` 通过。
- `corepack pnpm@9.15.9 audit --audit-level=moderate` 通过，输出只剩 1 个 low vulnerability。
- `pnpm why` 确认：
  - `vite` 解析到 `6.4.3`
  - `react-router` 解析到 `6.30.4`
  - `react-router-dom` 解析到 `6.30.4`
  - `@vitejs/plugin-react` 解析到 `4.7.0`
  - `@babel/runtime` 只剩 `7.29.7`

## 剩余问题

- `elliptic@6.6.1` 仍有 low alert，GitHub Dependabot advisory 显示 `first_patched_version=null`。
- 该依赖来自 `crypto-browserify` / `node-libs-browser` / Umi bundler webpack 兼容链路，不能通过普通 patch version 解决。
- 后续如需清零，需要评估 docs 工具链是否可以移除相关 webpack/node polyfill 兼容链，或等待上游替代方案。

## 开源治理口径

- 本轮是安全依赖收敛 PR，不改变公开文档内容和发布策略。
- docs deploy 只能在 GitHub checks 通过后由 main workflow 正常触发。
- 对外说明应强调：大部分 docs 安全告警已收敛，剩余 low alert 无上游 patched version，需要单独迁移评估。
