# mss-boot-io good first issue 任务工厂

## 记录时间

- 记录时间：2026-06-05
- 目标：为个人维护者准备低成本、可验证、适合 AI 辅助生成 issue 的社区任务池。
- 使用方式：Maintainer 从本文件挑选任务，确认范围后创建 GitHub issue，并打 `good first issue` 或 `candidate/good-first-issue`。

## 任务创建模板

每个 issue 建议包含：

- 背景：为什么需要这个任务。
- 修改范围：允许改哪些文件，避免新人误改架构。
- 非目标：明确不做的内容。
- 验证命令：本地命令或文档构建命令。
- 验收标准：review 时如何判断完成。

## 候选任务

2026-06-05 已从下表中直接创建 12 个首批公开 `good first issue`：

- `mss-boot-docs`：#4、#5、#6
- `mss-boot`：#348、#349、#350
- `mss-boot-admin`：#344、#345、#346
- `mss-boot-admin-antd`：#71、#72、#73

后续继续创建 issue 时，应先检查同名 issue 是否已存在，避免重复。

| # | 仓库 | 标题 | 修改范围 | 验证命令 | 验收标准 |
| ---: | --- | --- | --- | --- | --- |
| 1 | `mss-boot-docs` | `docs: add alpha/beta/prod environment quick reference` | `docs/devops/**` | `pnpm build` | 页面清楚说明三个环境、域名、发布门禁，不含凭据。 |
| 2 | `mss-boot-docs` | `docs: add SECURITY policy FAQ` | `docs/devops/**` 或 `docs/admin/**` | `pnpm build` | 用户知道漏洞不要发 public issue，知道如何提供复现。 |
| 3 | `mss-boot-docs` | `docs: add first contribution guide` | `docs/coding/**` | `pnpm build` | 新贡献者 30 分钟内能找到 fork、安装、测试、PR 流程。 |
| 4 | `mss-boot-docs` | `docs: add release checklist page` | `docs/devops/**` | `pnpm build` | 包含后端镜像、前端 Cloudflare、回滚、冒烟清单。 |
| 5 | `mss-boot-docs` | `docs: add troubleshooting page for login failures` | `docs/admin/**` | `pnpm build` | 覆盖 401、token 过期、菜单为空、API 域名错误。 |
| 6 | `mss-boot` | `docs: document go test and lint workflow` | `README.md`、`README.Zh-cn.md` | `go test ./...` | README 中有清晰本地验证命令。 |
| 7 | `mss-boot` | `test: add small test for config default behavior` | `pkg/config/**` | `go test ./...` | 增加低风险单元测试，不改变配置语义。 |
| 8 | `mss-boot` | `docs: add action/controller glossary` | `README.md` 或 docs 链接 | `go test ./...` | 新人能理解 Controller、Action、Hook、Scope 基本含义。 |
| 9 | `mss-boot` | `ci: add workflow badge section to README` | `README.md`、`README.Zh-cn.md` | 不需要构建 | badge 指向 CI、CodeQL、Scorecard。 |
| 10 | `mss-boot-admin` | `docs: document make test prerequisites` | `README.md`、`README.zh-CN.md` | `make test` | 说明 Redis、数据库或 mock 需求，避免新人跑不通。 |
| 11 | `mss-boot-admin` | `docs: add API smoke test commands` | `README.md`、`README.zh-CN.md` | `make test` | 给出健康检查、登录、配置接口的 curl 示例，不含真实密码。 |
| 12 | `mss-boot-admin` | `test: add unit test for language public API edge case` | `apis/**`、`service/**` 测试文件 | `make test` | 覆盖空 header/默认语言场景。 |
| 13 | `mss-boot-admin` | `docs: add RBAC glossary` | `README.zh-CN.md` 或 docs | `make test` | 解释用户、角色、菜单、API、数据范围。 |
| 14 | `mss-boot-admin` | `ci: document image tag strategy` | `README.md`、`README.zh-CN.md` | 不需要构建 | 说明 branch tag、commit SHA tag、release tag 的区别。 |
| 15 | `mss-boot-admin-antd` | `docs: add frontend local env matrix` | `README.md`、`README.zh-CN.md` | `pnpm tsc` | 说明 dev/alpha/beta/prod 对应 API 域名和命令。 |
| 16 | `mss-boot-admin-antd` | `test: add smoke test for app config page route` | `tests/**` 或 Playwright 测试 | `pnpm test` 或 `pnpm test:e2e` | 能验证页面渲染和关键接口错误提示。 |
| 17 | `mss-boot-admin-antd` | `docs: add Cloudflare release checklist` | `README.md`、`README.zh-CN.md` | `pnpm build:local` | 明确不能高频发布，beta 必须冒烟通过。 |
| 18 | `mss-boot-admin-antd` | `ci: add README badges for CI and CodeQL` | `README.md`、`README.zh-CN.md` | 不需要构建 | badge 指向 CI、CodeQL、Scorecard。 |
| 19 | `.github` | `docs: add label taxonomy usage guide` | `.github/README.md` 或 docs | 不需要构建 | 说明 `area/*`、`type/*`、`needs-*` 标签含义。 |
| 20 | `.github` | `ci: add label sync workflow proposal` | `.github/workflows/**` 或 backlog | 不需要构建 | 只生成提案或手动 workflow，不自动覆盖未知标签。 |

## 不适合作为 good first issue 的任务

- 权限模型重构、去多租户重构、认证流程调整。
- 数据库迁移、生产发布、Cloudflare 发布。
- Kubernetes Secret、Cloudflare Token、GitHub org settings。
- 大范围格式化或无明确验收标准的重构。

## AI 辅助规则

- AI 可以把本文件中的候选任务扩展成 issue 草稿。
- AI 不应自动创建大量 public issues；先由 Maintainer 挑选和确认。
- AI 可以在 Weekly Digest 中推荐 `candidate/good-first-issue`，但正式 `good first issue` 标签应由 Maintainer 确认。
