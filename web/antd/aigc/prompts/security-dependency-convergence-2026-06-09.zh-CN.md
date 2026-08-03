# 前端安全依赖收敛记忆

- 日期：2026-06-09
- 仓库：mss-boot-admin-antd
- 背景：Dependabot security update jobs 在 `axios`、`js-cookie`、`qs`、`react-router`、`vite` 上返回 `security_update_not_possible`。这些依赖大多来自 Umi/Alita/DVA/Pro CLI 传递链路，不适合一次性硬改到最新 major。

## 本轮处理

- `uuid` 直接依赖已由 Dependabot PR #99 升级到 `14.0.0` 并合并。
- `ahooks` 升级到 `^3.9.7`，使 `js-cookie` 进入 3.x 线。
- `@umijs/max` 与 `@umijs/lint` 升级到 `^4.6.61`，跟进 Umi 4 最新 patch。
- `@ant-design/pro-cli` 升级到 `^3.4.0`，降低旧工具链依赖面。
- `express` 升级到 `^4.22.2`，避免直接跳到 Express 5 造成 mock 类型和中间件语义漂移。
- `swagger-ui-dist` 升级到 `^5.32.6`。
- 通过 `pnpm.overrides` 收敛以下传递依赖：
  - `axios@0.32.0`
  - `js-cookie@3.0.8`
  - `qs@6.15.2`

## 明确暂缓

- `vite` 仍由 `@umijs/bundler-vite@4.6.x` 固定在 4.x 线，Dependabot 要求最低非漏洞线为 6.x。直接 override 到 Vite 6/8 风险高，应作为 Umi bundler 专项迁移。
- `react-router` 仍有 DVA 老链路引入 4.x，以及 Umi 4 引入 6.3.x。要彻底清零需要 DVA/Alita/Umi router 迁移，不应混在普通依赖补丁 PR。

## 验证

本轮在本地完成：

- `pnpm@9.15.9 install --frozen-lockfile`
- `pnpm@9.15.9 lint:js`
- `pnpm@9.15.9 tsc`
- `pnpm@9.15.9 exec jest --runInBand`
- `pnpm@9.15.9 build:local`
- `pnpm@9.15.9 build:beta`

在合入 package metadata 与 workflow 调整前，同一依赖收敛变更还完成过 `build:alpha`、`build:prod` 验证。

Cloudflare alpha/beta/prod workflow 仍是 `workflow_dispatch`，本轮不会自动发布前端。
