# 社区 issue 与 PR 治理轮次记忆

## 背景

- 时间：2026-06-09
- 范围：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`
- 目标：围绕社区 issue、PR Guard、CI、CodeQL、govulncheck、docs-drift、Cloudflare 发布约束，继续提升开源工程完善度。

## 本轮已推进的 PR

### mss-boot

- `#384 docs: document test workflow and action glossary`
  - 覆盖 issue：`#348`、`#349`
  - 内容：补充 `go test ./...`、lint workflow、Action/Controller 术语表和仓库本地 AIGC 记忆。
  - 验证：GitHub PR Guard、docs-drift、test、lint、govulncheck、CodeQL 均通过。
  - 状态：只差人工 review。
- `#385 test: cover config env fallback`
  - 覆盖 issue：`#350`
  - 内容：补充配置模板环境变量 fallback 表驱动测试，覆盖缺失变量、精确命中、大写 fallback、小写 fallback、非 `.Env.*` key 忽略。
  - 本地验证：`go test ./pkg/config`、`go test ./...`、`git diff --check`。
  - GitHub 验证：PR Guard、docs-drift、test、lint、govulncheck、CodeQL 均通过。
  - 状态：只差人工 review。

### mss-boot-admin

- `#388 docs: document admin test prerequisites and rbac glossary`
  - 覆盖 issue：`#344`、`#345`
  - 内容：补充 `make test` 前置条件、CI Redis 7 对齐说明、RBAC 术语表和仓库本地 AIGC 记忆。
  - 本地验证：`make test`、`git diff --check`、README 静态检查。
  - GitHub 验证：CI、PR Guard、docs-drift、govulncheck、CodeQL 均通过。
  - 状态：只差人工 review。
- `#389 test: cover language fallback middleware`
  - 覆盖 issue：`#346`
  - 内容：补充语言中间件 fallback 单测，覆盖空 header、受支持语言、权重后缀、区域归一化、不支持语言、query fallback、`Content-Language` fallback。
  - 本地验证：`go test ./middleware`、`go test ./...`、`git diff --check`。
  - GitHub 验证：CI、PR Guard、docs-drift、govulncheck、CodeQL 均通过。
  - 状态：只差人工 review。

### mss-boot-admin-antd

- `#97 ci: make release workflow idempotent`
  - 处理：同步 `origin/main` 到 PR 分支并重新触发 GitHub checks。
  - 本地验证：`pnpm@9.15.9 install --frozen-lockfile`、`pnpm@9.15.9 lint`；lint 脚本会写 prettier 结果，已恢复工作区噪音。
  - GitHub 验证：CI、PR Guard、docs-drift、CodeQL 均通过。
  - 状态：只差人工 review。
- `#100 chore: converge frontend security dependencies`
  - 状态：GitHub checks 全绿，只差人工依赖/lockfile review。
  - 约束：不触发 Cloudflare 发布。
- `#102 docs: add frontend environment matrix`
  - 覆盖 issue：`#71`、`#72`
  - 状态：GitHub checks 全绿，只差人工 review。
  - 约束：说明本地开发对接 alpha API，Cloudflare beta/prod 发布保持手动和冒烟测试门槛。

### mss-boot-docs

- `#31 chore: converge docs security dependencies`
  - 状态：GitHub checks 全绿，只差人工依赖/lockfile review。
  - 后续：剩余 dumi/Umi 链路安全项进入 `#32` RFC 迁移轨道。

## 社区互动闭环

- 已在 `mss-boot#348`、`#349`、`#350` 回复对应 PR 与验证范围。
- 已在 `mss-boot-admin#344`、`#345`、`#346` 回复对应 PR 与验证范围。
- 已在 `mss-boot-admin-antd#71`、`#72` 回复 `#102` 的环境矩阵与 Cloudflare 发布边界。
- 已在 `mss-boot#352` 说明 `#383` 只阻止新增 lint debt，历史 backlog 需要按 package/linter 小 PR 消化。
- 已在 `mss-boot-admin#374`、`#111`、`#55`、`#53` 回复 RFC 边界和下一步贡献方向。
- 已在 `mss-boot-admin-antd#101`、`mss-boot-docs#32` 回复安全迁移轨道边界。

## 标签治理

- `mss-boot-admin#374`：`needs-rfc`、`queue/discussion`、`breaking-change`
- `mss-boot-admin#111`：`needs-rfc`、`queue/discussion`、`type/feature`
- `mss-boot-admin#55`：`needs-rfc`、`queue/discussion`、`type/feature`
- `mss-boot-admin#53`：`needs-rfc`、`queue/discussion`、`type/feature`、`type/security`
- `mss-boot-admin-antd#101`：`area/frontend`、`queue/discussion`、`type/security`
- `mss-boot-docs#32`：`area/docs`、`queue/discussion`、`type/security`
- `mss-boot#352`：`area/backend`、`queue/discussion`

## 当前人工边界

- 代码与文档 PR 已全部交给 GitHub checks 验证，当前阻塞主要是 `REVIEW_REQUIRED`。
- 依赖收敛类 PR 需要人工确认 lockfile、生态兼容性和发布节奏。
- 前端仍不应高频发布 Cloudflare；只有在本地验证完整、beta 可公开展示时，才进入手动 Cloudflare 发布和冒烟测试。
- 高影响功能保留 RFC-first：SQL migration/rollback、手机号登录、通知多 provider、统计能力、Umi/Vite/router 与 dumi/Umi 迁移。

## 后续推荐

1. 人工 review 并按风险顺序合并绿灯 PR：文档/测试 PR 优先，依赖 PR 其次，release workflow PR 可与依赖 PR 分开合并。
2. 合并后观察自动关闭 issue：`#344`、`#345`、`#346`、`#348`、`#349`、`#350`、`#71`、`#72`。
3. `mss-boot#352` 继续按小切片消化 lint backlog，避免大 PR。
4. 用 GitHub Discussions 或 issue comment 邀请社区参与 RFC review，把实现前置到可讨论设计。
5. 继续依赖 GitHub PR Guard、Docs Drift、CodeQL、govulncheck、Scorecard、Dependabot 做低成本社区质量护栏。
