# admin 与 mss-boot 易用性/完整度推进记忆

时间：2026-06-10 09:12 CST
更新：2026-06-10 09:35 CST
追加更新：2026-06-10 11:00 CST

## 背景

用户要求继续推进 `admin` 和 `mss-boot` 的整体完整度，并优化易用性。当前目标偏向开源项目第一体验、GitHub AI/CI 可维护性、以及本地开发路径一致性。

## 本轮处理范围

### mss-boot

- PR：https://github.com/mss-boot-io/mss-boot/pull/386
- 分支：`codex/open-source-security-baseline-20260607`
- 最终提交：`9e82bd8 fix: align version ldflags`
- Squash merge 提交：`e62d7da9678588f3113a964b4e9c9221bf4581ea`
- 校对 GitHub Release 后确认最新版本为 `v0.7.3`，README 原先仍写 `v0.7.2`，安装命令仍写 `v0.7.1`。
- 更新 `README.md` / `README.Zh-cn.md`：
  - 最新版本统一为 `v0.7.3`。
  - 快速安装命令统一为 `go get github.com/mss-boot-io/mss-boot@v0.7.3`。
  - 增加 Go 1.26+ 要求与 `make tidy/test/coverage/lint` 本地检查入口。
  - 补充 v0.7.x 亮点：GORM 查询缓存失效、Mongo ObjectID 安全校验、结构化 issue forms 等。
- 更新 `CHANGELOG.md`，补齐 `v0.7.3` 条目。
- 修正 `Makefile` 中旧组织路径 `github.com/matchstalk/mss-boot` 为 `github.com/mss-boot-io/mss-boot`。
- 给 `Makefile` 增加 `test`、`coverage`、`tidy` 目标。
- 按 Copilot Review 意见修复 `Makefile` 版本注入路径：`LDFLAGS` 现在写入 `github.com/mss-boot-io/mss-boot/pkg/version.buildDate`、`gitCommit`、`gitVersion`，并使用 UTC ISO8601 构建时间。

### mss-boot-admin

- PR：https://github.com/mss-boot-io/mss-boot-admin/pull/390
- 分支：`codex/fix-language-public-beta`
- 最终提交：`1e0bea8 Merge remote-tracking branch 'origin/main' into codex/fix-language-public-beta`
- Squash merge 提交：`c8f35515cf59e5fa4e348a67ab82b3c44e0588c5`
- 更新 `README.md` / `README.zh-CN.md`：
  - Go 版本统一为 1.26+。
  - 本地快速开始改为 SQLite 默认路径，避免继续误导贡献者以为 `DB_DSN` 环境变量在 local 配置下必然生效。
  - MySQL/Redis 改为可选集成依赖。
  - 前端启动命令改为 `corepack enable`、`pnpm install`、`pnpm start:dev`。
- 更新 `.github/copilot-instructions.md`：
  - 说明本地默认读取 `config/application.yml`，当前基线为 SQLite。
  - `DB_DSN` / `DB_DRIVER` 改为仅在 GORM/远程配置源或自定义数据库配置时需要。
  - 故障排查从优先检查 `DB_DSN` 改为优先检查 `config/application.yml` 的 database 配置。
- 修复 `Makefile`：把 `test`、`deps`、`generate`、`lint`、`fix-lint` 加入 `.PHONY`。本轮发现原先 `make test` 会因为本地存在 `test` 目标而输出 `make: 'test' is up to date.`，没有真正跑测试。
- 注意：`testdata/test.tar.gz` 与 `testdata/test.zip` 在本轮开始前已经是脏文件，本轮未修改、未提交。

### mss-boot-admin-antd

- PR：https://github.com/mss-boot-io/mss-boot-admin-antd/pull/105
- 分支：`codex/open-source-issue-templates-20260607`
- 最终提交：`0cdbbfc docs: align frontend startup commands`
- Squash merge 提交：`b06da3efcba3693f56ff8909f3a5ad3ed8a8ac14`
- 更新 `package.json`：
  - `engines.node` 从 `>=12.0.0` 更新为 `>=22.0.0`。
  - 增加 `engines.pnpm >=9.0.0`。
  - 将内部脚本链路从 `npm run` 改为 `pnpm`。
  - 修正文案中的 Ant Design / Umi 描述，并弱化已降级的虚拟模型方向。
- 增加 `.nvmrc`，内容为 `22`。
- 更新 `README.md` / `README.zh-CN.md`：
  - 前端 Node.js 22+、pnpm 9+。
  - 后端本地默认 SQLite。
  - 启动命令统一为 `corepack enable`、`pnpm install`、`pnpm start:dev`。
- 更新 `.github/copilot-instructions.md`：
  - 增加运行时基线：Node 22+、pnpm 9+、不要新增 npm/yarn lockfile。
- 优化 `src/requestErrorConfig.ts`：
  - 拆出纯函数 `src/util/requestError.ts`。
  - HTTP 错误消息现在可安全处理 `msg`、`errorMessage`、`message`、`error`、`detail`、`reason`、字符串 body、空 body。
  - 移除响应拦截器中的泛化 `请求失败！`，避免和错误处理器重复弹框。
- 增加 `src/util/requestError.test.ts` 覆盖错误消息提取。
- 更新 `src/typings.d.ts`，补齐 `REACT_APP_ENV` 的 `local`、`alpha`、`beta`、`prod` 类型。
- 合并 `origin/main` 后保留主线依赖升级和 `pnpm.overrides`，并按 Copilot Review 意见将 README 启动命令统一为当前真实脚本：`pnpm dev` / `pnpm start:no-mock`。

#### 依赖治理追加

- PR：https://github.com/mss-boot-io/mss-boot-admin-antd/pull/106
- 标题：`chore: remove obsolete frontend tooling`
- Squash merge 提交：`461956dc4096ea4a77915b2e1c5697b4e37af2f4`
- 移除未使用且带来安全告警链路的 `react-dev-inspector`、`@ant-design/pro-cli` 和 `i18n-remove` 脚本，清理 `tar`、`shell-quote`、`yeoman-environment` 相关告警入口。

- PR：https://github.com/mss-boot-io/mss-boot-admin-antd/pull/107
- 标题：`chore(deps): bump lodash from 4.17.21 to 4.18.1`
- Squash merge 提交：`d32b8ce5f20365205c5bee49f8dd80f5952d89a3`
- 评估 Dependabot PR 后确认方向合理，浏览器验证检查通过后 squash merge。

- PR：https://github.com/mss-boot-io/mss-boot-admin-antd/pull/108
- 标题：`chore: pin vulnerable transitive frontend deps`
- Squash merge 提交：`a2e5638aae499765fdf9bfd56db285d9059127c7`
- 使用 `pnpm.overrides` 收敛 `@babel/plugin-transform-modules-systemjs`、`@tootallnate/once`、`immutable`、`flatted`、`picomatch`、`postcss`、`yaml` 等传递依赖告警。

- PR：https://github.com/mss-boot-io/mss-boot-admin-antd/pull/109
- 标题：`chore: pin remaining vulnerable frontend deps`
- Squash merge 提交：`d017f2f7c15676267d4928dcc7a7020bc26b28b6`
- 追加 `webpack@5.107.2` 直接 devDependency，并用 `pnpm.overrides` 收敛 `@remix-run/router`、`js-yaml`、`minimatch`、`webpack` 传递依赖告警。

- PR：https://github.com/mss-boot-io/mss-boot-admin-antd/pull/110
- 标题：`chore: pin axios and dom transitive deps`
- Squash merge 提交：`b83872c8902e18eb029956cc11dacc18bdee6a37`
- 追加 `form-data@4.0.5`、`min-document@2.19.2` overrides，补齐 #109 合并后 GitHub Dependabot dynamic runs 暴露的剩余传递依赖告警。

## 验证结果

- `mss-boot`: `make test` 通过。
- `mss-boot`: 手动验证 `go test -ldflags ... ./pkg/version` 通过，确认 Makefile 版本注入路径有效。
- `mss-boot-admin`: `make test` 通过，确认 Makefile `.PHONY` 修复后真实执行了测试。
- `mss-boot-admin`: `make build` 通过；构建生成的本地 `admin` 二进制已清理。
- `mss-boot-admin`: GitHub PR 检查全部通过，包括 CI build、CodeQL、govulncheck、Docs Drift、PR Guard、Copilot Setup、Dependabot 配置检查。
- `mss-boot-admin-antd`: `pnpm test -- --runInBand src/util/requestError.test.ts` 通过。
- `mss-boot-admin-antd`: `pnpm tsc` 通过。
- `mss-boot-admin-antd`: `pnpm lint:js` 通过。
- `mss-boot-admin-antd`: `pnpm build:local` 通过。
- `mss-boot`: GitHub PR 检查全部通过，包括 ci test/lint、govulncheck、CodeQL Advanced、Docs Drift、PR Guard。
- `mss-boot-admin-antd`: GitHub PR 检查全部通过，包括 CI build、CodeQL、Docs Drift、PR Guard。
- `mss-boot`: PR #386 已浏览器复核，Copilot Review 无阻塞评论，已 squash merge。main 后续 GitHub Actions Mirror、OpenSSF Scorecard、govulncheck、ci、CodeQL Advanced 均通过。
- `mss-boot-admin`: PR #390 已浏览器复核，已 squash merge。main 后续 GitHub Actions 均通过。
- `mss-boot-admin-antd`: PR #105/#106/#107/#108/#109/#110 均已浏览器复核并 squash merge。#110 页面显示 `10 checks passed`，Copilot Review 生成 0 条评论；main 最新提交 `b83872c` 的 GitHub Actions Mirror、OpenSSF Scorecard、CodeQL、CI 均通过。

## 当前状态

- 本轮涉及的可合并 PR 已全部 squash merge：
  - `mss-boot` #386。
  - `mss-boot-admin` #390。
  - `mss-boot-admin-antd` #105/#106/#107/#108/#109/#110。
- `mss-boot-admin-antd` 当前无 open PR。
- GitHub Actions 历史记录中仍可看到旧提交上的 Dependabot dynamic failure，但最新 main 提交 `b83872c` 的核心流水线均为 success；截至 2026-06-10 11:00 CST，未观察到针对 `b83872c` 的新 dynamic failure。
- 本轮未发布镜像、未发布 Cloudflare。
- `mss-boot-admin` 工作区仍保留用户原有脏文件：`testdata/test.tar.gz`、`testdata/test.zip`，不要在后续无关任务中提交或回滚它们。
- `mss-boot-admin-antd` 仍保留后续治理 issue：
  - #101：RFC，迁移 Umi/Vite/router 依赖链以继续降低安全告警和维护成本。
  - #103：文档盘点前端安全迁移影响。
  - #104：补充前端迁移 smoke/rollback checklist。

## 后续建议

- `mss-boot-admin` 后续应补一个更轻量的本地依赖路径：单节点 Redis compose 或明确说明当前 Redis compose 是主从哨兵拓扑，避免新人误以为必须启动全套。
- `mss-boot-admin` 可以继续把 `config/application.yml` 的 SQLite 默认体验做成更明确的 `make dev` / `make migrate` / `make run`。
- `mss-boot-admin-antd` 后续可处理 Browserslist 数据库陈旧问题，但应评估是否会带来 lockfile 变化。
- `mss-boot-admin-antd` 当前通过 overrides 收敛了一批传递依赖风险，但长期 9.2+ 到 10 分方向仍应落在前端技术栈升级：减少 Umi 3/旧 webpack/旧富文本编辑器链路对传递依赖安全治理的拖累。
- 前端 Cloudflare 发布仍遵守低频原则：本地和 CI 验证充分后再发布 beta/prod。
