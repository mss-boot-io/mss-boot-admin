# docs 依赖安全收敛记忆

- 日期：2026-06-09
- 仓库：mss-boot-docs
- 背景：docs 仓库 Dependabot alerts 主要来自 dumi/Umi/Vite 文档站工具链。可低风险处理的传递依赖应先收敛，Vite/React Router 等主链路迁移单独评估。

## 本轮处理

- `dumi` 从 `^2.4.28` 升级到 `^2.4.30`，带动 Umi patch 到 4.6.61。
- 增加 `pnpm.overrides`：
  - `esbuild@0.25.11`
  - `nth-check@2.1.1`
  - `path-to-regexp@1.9.0`
  - `send@0.19.2`
  - `uuid@11.1.1`

## 验证

- `CI=true npx -y pnpm@9.15.9 install --frozen-lockfile`
- `npx -y pnpm@9.15.9 build`
- `npx -y pnpm@9.15.9 exec prettier --check ...`
- `npx -y pnpm@9.15.9 audit --json`

## 剩余风险与边界

- `vite` 仍由 `dumi -> umi -> @umijs/bundler-vite` 固定在 4.x，需要 dumi/Umi bundler 专项迁移或上游升级，不能在普通补丁 PR 中强行 override 到 Vite 6/7/8。
- `react-router` 仍由 Umi 4 链路固定在 6.3.x，需要上游 Umi routing 迁移。
- `elliptic` 当前 advisory 无 patched version，来自 webpack/node-libs-browser 兼容链路，需等待上游或替换构建链路。
- `@babel/runtime` advisory 也来自 Umi 主链路，需等待上游 patch 或专项迁移。
