# Community PR Validation and Merge Sweep

日期：2026-06-07

## 范围

本轮只处理 `mss-boot-io` 的 4 个核心开源仓库：

- `mss-boot`
- `mss-boot-admin`
- `mss-boot-admin-antd`
- `mss-boot-docs`

目标是按开源社区标准验证外部贡献 PR，合理则公开回复、approve、squash merge；
同时继续推进仓库治理和 AI/CI 能力，使组织开源完善度稳定进入 9 分以上区间。

## 已合并的外部社区 PR

- `mss-boot-admin-antd#95` by `titanniya542-spec`
  - 内容：修正文档中的测试命令，使 README / TESTING 与 `package.json` 脚本一致。
  - 验证：脚本名核对、`git diff --check`。
  - 处理：英文 approve 回复，squash merge。
- `mss-boot-admin#376` by `12ain`
  - 内容：在线会话、服务端 session、强制下线、菜单/API 权限 seed、审计日志和回归测试。
  - 维护者补丁：新增 session 菜单 migration 的 `default` role 查询改为 GORM
    `clause.Eq`，避免 PostgreSQL/TimescaleDB 反引号风险。
  - 验证：`go test ./cmd/migrate/migration/system`、
    `go test ./pkg/sessioncache ./service ./apis ./middleware`、`go test ./...`。
  - 处理：英文 approve 回复，squash merge。
- `mss-boot-admin-antd#94` by `12ain`
  - 内容：在线会话前端页面、详情抽屉、强制下线交互、退出时撤销 sid、路由和国际化。
  - 验证：`pnpm install --frozen-lockfile`、`pnpm tsc`、`pnpm lint:js`、
    `pnpm build:local`。
  - 处理：后端 `#376` 合并后英文 approve 回复，squash merge。

## 已合并的维护者治理 PR

- `mss-boot-admin#375`：PostgreSQL/TimescaleDB migration 兼容修复，先于 `#376`
  合并，避免新增菜单 migration 触发旧 `Menu.AfterCreate` 方言问题。
- `mss-boot#379`：补齐 issue forms、README 开源入口，并加强 Mongo delete
  ObjectID 校验。
- `mss-boot-admin#370`：删除 audit middleware 未使用的 response writer wrapper。
- `mss-boot-admin-antd#93`：补齐前端 issue forms，修复 README license 链接。

这些维护者 PR 无法由同一账号 approve。处理原则是先公开留下验证说明，再在 checks
和本地测试通过后使用 maintainer/admin 权限 squash merge。

## Release 处理

- `mss-boot#380` 已合并，核心 query cache tag invalidation 修复进入 main。
- 本轮已发布 `mss-boot v0.7.3`，覆盖：
  - query cache create/update/delete tag invalidation；
  - issue forms 与 README 开源入口；
  - Mongo ObjectID delete 校验；
  - 最近一批依赖维护更新。
- 发布后 `mss-boot-admin#371` 已从 pseudo-version 切换到正式
  `github.com/mss-boot-io/mss-boot v0.7.3`。

## 后续复核与合并

- `mss-boot-admin#371`
  - 已按 `12ain` 二轮 review 修复 P0/P1：
    - query cache invalidation 失败会记录 error；
    - `queryCache=true` 但 `queryCacheDuration<=0` 会启动期 warning；
    - 依赖改为 `mss-boot v0.7.3` 正式 tag。
  - 本地验证：`go test ./config`、`go test ./...`。
  - GitHub checks 已全绿。
  - 2026-06-08 复核结论：该 PR 修复的是仍在使用的 query cache 运行期问题，
    与虚拟模型降级范围无关；已公开说明并使用 maintainer/admin 权限
    squash merge。
- `mss-boot-admin#372`
  - 已按 `12ain` 二轮 review 修复 P0/P1：
    - cleanup 失败后恢复已 soft-delete 的 model；
    - `LoadPolicy()` 失败后恢复 model、menu、Casbin policy，再返回错误；
    - policy 删除前收集可恢复数据并记录数量，避免不可观测的授权漂移。
  - 本地验证：`go test ./apis -run TestDeleteGeneratedModelMenus`、`go test ./...`。
  - GitHub checks 已全绿。
  - 2026-06-08 复核结论：虚拟模型已是降级/废弃路径功能，不应继续为该功能
    扩展 P2/P3 复杂边界。本 PR 仅作为存量安装的有限安全兜底合并，后续除非
    活跃用户在移除前报告明确回归，否则不继续深投 i18n 清理、并发竞争、递归
    CTE 深度保护等增强。

## 社区治理准则

- 外部贡献者 PR：必须先回复，语境友好、具体、尊重贡献；通过验证后 approve；
  合并使用 squash merge。
- 如果外部 reviewer 提出 P0/P1，即使 PR 来自维护者/Codex 分支，也先修复并公开
  说明，不直接绕过。
- 对已降级或废弃路径功能，社区说明必须明确范围：只接受低投入的存量安全兜底，
  不把 review 扩展成新能力建设。
- 维护者自己的 PR 不能用同账号 approve；应公开留下验证说明，再由分支策略或
  maintainer/admin 权限处理。
- GitHub AI 能力优先使用：Copilot PR review、CodeQL、PR Guard、Docs Drift、
  govulncheck、CI build、coverage comment 都应作为公开验证材料。
- 前端 Cloudflare 发布仍遵守低频发布策略；本轮只合并代码，不触发 Cloudflare
  beta/prod 发布。

## 验证注意

- `mss-boot-admin` 全量 `go test ./...` 会改动 `testdata/test.tar.gz` 和
  `testdata/test.zip`，这是测试副作用。除非任务明确涉及这些文件，否则恢复它们，
  不纳入 PR。
- 涉及 PostgreSQL/TimescaleDB 的 migration 不能使用 MySQL 反引号，应使用
  GORM 方言感知写法，例如 `clause.Eq{Column: clause.Column{Name: "default"}}`。
