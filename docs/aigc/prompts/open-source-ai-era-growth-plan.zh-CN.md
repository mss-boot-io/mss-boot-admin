# mss-boot-io 开源组织评分与 AI 时代增长方案

## 评估时间与数据来源

- 评估时间：2026-06-04
- 工作区：本地多仓库 workspace，根目录形态为 `<workspace>/mss-boot-io`
- GitHub 组织：`https://github.com/mss-boot-io`
- 数据来源：
  - `gh api orgs/mss-boot-io`
  - `gh api orgs/mss-boot-io/repos --paginate`
  - GitHub GraphQL repositories query
  - `gh issue list` / `gh pr list` / `gh release list` / `gh run list`
  - 核心仓库本地文件与 `aigc` 记忆
- 外部参考：
  - GitHub Actions 对公开仓库标准 runner 免费：https://docs.github.com/en/billing/concepts/product-billing/github-actions
  - Cloudflare Pages 免费计划每月构建次数限制：https://developers.cloudflare.com/pages/platform/limits/
  - OpenSSF Scorecard：https://openssf.org/scorecard/
  - GitHub Sponsors 费用说明：https://docs.github.com/en/sponsors/getting-started-with-github-sponsors/about-github-sponsors

## GitHub 当前可见数据

截至本次拉取，GitHub API 返回：

- 可见仓库：38 个，其中非 fork 33 个，fork 5 个。
- 组织 profile 显示公开仓库数：34 个；实际 API 返回数和 profile 数存在差异，后续以 repo API 拉取结果作为评估明细。
- 组织 followers：16。
- 总 stars：128。
- 总 forks：39。
- Open items：47 个，GitHub REST 的 `open_issues_count` 包含 issue 和 PR。
- 2026 年有 push 的仓库：7 个。
- 2025 年有 push 的仓库：8 个。
- 2025 年以前最后 push 的仓库：23 个。
- 有 license 的仓库：21 个。

核心仓库信号：

| 仓库 | stars | forks | 最新 release | 最近 push | 社区健康分 |
| --- | ---: | ---: | --- | --- | ---: |
| `mss-boot-admin` | 69 | 13 | `v0.6.1` | 2026-06-04 | 75 |
| `mss-boot` | 37 | 6 | `v0.7.1` | 2026-06-03 | 62 |
| `mss-boot-admin-antd` | 5 | 7 | `v0.5.1` | 2026-04-06 | 62 |
| `mss-boot-docs` | 0 | 3 | 无 release | 2026-04-06 | 37 |

### 2026-06-05 GitHub 公网复核

本轮复核使用 `gh repo list mss-boot-io` 与 `gh api repos/mss-boot-io/<repo>/community/profile`。需要注意：以下是公网 main 分支已发布状态，不包含本地尚未提交/推送的治理改动。

- 可见仓库：38 个，其中公开 34 个、私有 4 个、归档 0 个。
- 总 stars：128；总 forks：39。
- 当前 GitHub 社区健康分：`.github` 12，`mss-boot` 62，`mss-boot-admin` 75，`mss-boot-admin-antd` 62，`mss-boot-docs` 37。
- 公开仓库中仅 `mss-boot` 当前显示启用 Security policy；`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs` 与组织 `.github` 仍需在变更合并后通过 GitHub 重新识别。
- 公开/可见仓库中有 17 个仓库缺 license 元数据，包含 `.github`、`mss-boot-docs` 之外的若干历史/示例仓库；主评分优先聚焦四核心仓库，历史仓库后续可按“归档、补 license、补描述、转移到示例集”分批治理。
- 当前 top 仓库：`mss-boot-admin` 69 stars / 13 forks，`mss-boot` 37 stars / 6 forks，`mss-boot-admin-antd` 5 stars / 7 forks，`workflow-tools-go` 5 stars / 1 fork。

复核结论：公网当前健康度仍停留在 6.4 左右，本地已完成的源码侧治理改动属于 9.2+ 准备态；真实公开评分需要完成提交、推送、合并，并配置分支保护、required checks、private vulnerability reporting 等外部设置后再复核。

贡献者集中度：

| 仓库 | 主要贡献者 | 外部贡献信号 |
| --- | --- | --- |
| `mss-boot` | `lwnmengjing` 154 commits | `WangDe7`、`snakelu` 有少量贡献，Dependabot 活跃 |
| `mss-boot-admin` | `lwnmengjing` 161 commits | `wxip`、`WangDe7`、`mss-boot` 有少量贡献 |
| `mss-boot-admin-antd` | `lwnmengjing` 68 commits | `chenfei4396`、`mss-boot` 有少量贡献 |
| `mss-boot-docs` | `lwnmengjing` 38 commits | 暂未形成外部贡献 |

工程与社区配置：

- 核心仓库均有 MIT License。
- `mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd` 有 PR 模板。
- `mss-boot-admin` 有 issue templates 和 `CONTRIBUTING.md`。
- `mss-boot-docs` 社区健康文件明显不足。
- `mss-boot` 已有 CodeQL、Dependabot、Release workflow。
- `mss-boot-admin` 有 CI、Swagger、Release、Dependabot。
- `mss-boot-admin-antd` 已有 alpha/beta/prod Cloudflare workflow。
- `mss-boot-docs` 有 docs deploy workflow。

## 总评分

当前开源组织综合评分：6.4 / 10。

这个分数不是项目价值低，而是开源组织化程度还没完全展开。核心代码和产品底子已经够用，短板主要在“社区入口、贡献者转化、发布可信、组织级治理、安全披露、AI 自动化运营”。

| 维度 | 权重 | 当前分 | 主要证据 | 到 10 分需要补齐 |
| --- | ---: | ---: | --- | --- |
| 产品定位与叙事 | 12% | 7.2 | 已从低代码/多租户叙事收敛到治理型 admin，但 README、copilot 指令和历史仓库仍可能有旧叙事 | 统一成 AI-governed / agent-readable Go microservice admin framework |
| 工程可信度 | 14% | 7.0 | CI、release、CodeQL、Dependabot 已具备，但强门禁和覆盖率证据还需组织化 | CI 必过、覆盖率门禁、OpenSSF Scorecard、供应链证明 |
| 文档与上手 | 14% | 7.0 | docs 项目存在，README 有 quickstart；但 docs 社区健康分低，文档站还没完全成为单一事实源 | 5 分钟上手、FAQ、故障排查、版本矩阵、示例自动验证 |
| 社区健康 | 12% | 5.4 | 缺组织级 Code of Conduct、Support、统一 issue 模板、Discussions 体系 | 组织级 `.github` 社区健康文件和标签/SLA |
| 贡献者体验 | 12% | 5.2 | 外部贡献少，good first issue 机制未形成 | 30 分钟可贡献路径、每月稳定 good first issue、快速首评 |
| 发布质量 | 12% | 6.2 | release 存在，前后端/Cloudflare 流程已定义；但 tag、SBOM、回滚、版本矩阵还不完整 | 可复现发布、SBOM、checksum、attestation、回滚手册 |
| 安全与供应链 | 12% | 5.5 | CodeQL 有基础，Security policy 和漏洞响应流程不足 | SECURITY、govulncheck、secret scan、Scorecard、SLSA/attestation |
| AI 时代差异化 | 12% | 7.0 | `aigc`、多 agent 流程、组织记忆已有雏形 | context pack、contract manifest、OpenAPI `x-mss-*`、policy explain |

## 核心判断

mss-boot-io 不应该去卷“更快生成 CRUD”。更好的定位是：

中文：面向 AI 协作时代的治理型 Go 微服务后台框架。

英文：AI-governed Go microservice admin framework.

差异化不是“低代码”或“动态模型”，而是：

- Agent-readable：AI 能读懂项目边界、接口、权限、配置、测试和发布环境。
- Governance-first：权限、审计、配置、组织、任务、通知和发布流程可追溯。
- Production-maintainable：不是 demo，而是可部署、可回滚、可测试、可交接。
- Docs-as-memory：`mss-boot-docs` 是组织级 AI 记忆中心，不只是文档站。

## 10 分目标定义

10 分不是 stars 很多，而是个人开发者仍能低成本维护一个可信开源组织：

- 新用户 5 分钟知道项目解决什么问题。
- 新贡献者 30 分钟能跑通并提交一个小 PR。
- 每个核心 PR 都有测试、文档、安全、发布影响说明。
- 每次 release 都能追踪 commit、镜像、前端版本、配置差异和回滚方式。
- 社区问题 48 小时内有 AI 辅助首响，维护者只处理需要判断的部分。
- AI agent 能通过上下文包理解仓库、接口、权限、测试、发布环境。
- 文档、README、OpenAPI、前端 service、部署配置之间不漂移。

## 0 到 30 天：把评分从 6.4 推到 7.5

目标：补齐开源信任底座，最低成本提升专业度。

1. 组织级 `.github` 基建
   - 新建或完善组织 `.github` 仓库。
   - 添加默认 `CODE_OF_CONDUCT.md`、`SECURITY.md`、`SUPPORT.md`、`CONTRIBUTING.md`。
   - 添加统一 issue templates、PR template、Funding 配置。
   - 建立标签规范：`bug`、`docs`、`good first issue`、`help wanted`、`security`、`needs reproduction`、`ai-ready`、`release-blocker`。

2. README 和定位统一
   - 四个核心仓库首屏统一定位。
   - 删除或弱化“低代码”“动态模型主线”“多租户 SaaS 主线”旧表达。
   - 明确当前主线：治理型 admin、生产运维、AI 协作可读。
   - 修正 Go/Node/pnpm 版本、体验环境、alpha/beta/prod 域名说明。

3. CI 门禁
   - `mss-boot`、`mss-boot-admin`：`go test ./...` 必过。
   - 前端：`pnpm install --frozen-lockfile`、`pnpm tsc`、`pnpm test`、`pnpm build`。
   - 文档：`pnpm build`。
   - 取消 lint 的 `continue-on-error`，或拆成非阻塞 warning job 和必过基础 job。

4. AI 运营最小闭环
   - Weekly Digest：每周自动汇总 stars/forks/issues/PR/releases/CI 状态。
   - Issue Triage：缺复现则自动评论提醒，按标题/模板自动打标签。
   - PR Guard：检查测试命令、文档影响、安全影响、发布影响是否填写。

30 天验收：

- 核心仓库社区健康分全部达到 75+，`mss-boot-docs` 达到 70+。
- 每个核心仓库 README 首屏统一。
- 至少 10 个 `good first issue`。
- 近 30 天 issue/PR 首响时间小于 48 小时。

## 31 到 90 天：把评分推到 8.5

目标：建立贡献飞轮和 AI-ready 差异化。

1. Agent Context Pack
   - 增加 `mss context export --format markdown/json` 或先用脚本输出。
   - 内容包括仓库地图、模块责任、路由、OpenAPI 摘要、权限、配置、模型、测试命令、发布环境。
   - 先从 `mss-boot-admin` 做样板，再扩展到 `mss-boot`。

2. MSS Contract Manifest
   - 对用户、角色、菜单、API、任务、通知 6 个模块建立 `mss.contract.json` 样板。
   - 描述 resource、routes、auth、permissions、dataScope、audit、events、configs、tests。
   - CI 检查 contract、OpenAPI、前端 service 是否漂移。

3. OpenAPI `x-mss-*` 扩展
   - 增加 `x-mss-auth`、`x-mss-permission`、`x-mss-data-scope`、`x-mss-audit`、`x-mss-config`、`x-mss-i18n`。
   - 让前端、文档、测试和 agent 都围绕同一份契约工作。

4. 社区内容节奏
   - 每周 1 篇 5 分钟上手或真实问题修复。
   - 每月 1 篇架构取舍或 release 解读。
   - GitHub Discussions 开 6 类：Q&A、Show and Tell、RFC、Roadmap、Help Wanted、中文交流。

5. 外部贡献路径
   - 准备 20 个小任务：文档、测试、i18n、FAQ、示例。
   - 每个 good first issue 写清背景、修改范围、测试命令、验收结果。
   - 首个外部 PR 优先 24 小时内 review，合并后在 release note 或月报感谢。

90 天验收：

- 至少 3 个外部 PR。
- 至少 5 个有效 Discussions。
- 至少 20 个有效 issue，其中 10 个沉淀为 FAQ 或文档改进。
- 形成 1 套 AI 辅助运营流水线。
- 核心仓库 OpenSSF Scorecard 有可见基线并持续上升。

## 91 到 180 天：把评分推到 9.2

目标：从“好项目”变成“可信开源组织”。

1. 发布可信度
   - 建立版本兼容矩阵：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs`。
   - release 产出 changelog、upgrade note、checksum、SBOM、镜像 digest、回滚说明。
   - 后端 prod 只发 tag 版本；前端 Cloudflare 发布必须关联后端 tag/API 版本。

2. 安全与供应链
   - `SECURITY.md` 写清支持版本、报告入口、响应 SLA、披露流程。
   - 四仓库统一 Dependabot、CodeQL、govulncheck、pnpm audit。
   - 加 OpenSSF Scorecard workflow。
   - 对 release 加 artifact attestation 或 SLSA 生成器。

3. 架构可解释
   - 建 ADR 目录，先写认证、RBAC/Casbin、配置源、存储、国际化、动态模型/代码生成取舍。
   - 增加 Action/Hook Trace：action、resource、permission、scope、hook 耗时、error code、audit id。
   - 增加 Policy Explain / Dry Run：
     - `mss admin policy explain --user u1 --method POST --path /admin/api/users`
     - 输出允许/拒绝原因、角色、菜单/API 权限、数据范围。

4. Admin AI Console
   - 不做泛聊天框。
   - 做明确按钮：
     - 解释用户为什么看不到菜单。
     - 解释配置变更影响。
     - 总结最近 24 小时告警。
     - 生成本次发布说明。
     - 根据失败测试生成复现步骤。

180 天验收：

- OpenAPI、contract、docs、frontend service 有漂移检查。
- 每次 release 可追踪、可回滚、可解释。
- 至少 1 个长期贡献者进入 triage 或 reviewer 角色。
- 社区维护时间下降，重复问题主要由 FAQ、模板、AI 首响处理。

## 9.2 到 10 分：长期能力

10 分阶段不靠堆功能，靠持续可信。

- 形成维护者梯队，不再完全依赖个人。
- 有稳定用户案例和 Show and Tell。
- 有清晰路线图和弃用计划。
- 有赞助入口和成本透明度。
- 有公开运营月报。
- AI agent 能稳定完成 issue triage、PR guard、release note、docs drift、good first issue 生成。
- 用户可以用同一套文档理解本地、alpha、beta、prod 的发布路径。

## 个人开发者低成本社区运营方案

原则：少开渠道，多做复用；AI 做重复劳动，人做判断。

推荐工具：

| 场景 | 低成本方案 |
| --- | --- |
| 代码与社区 | GitHub Issues、PR、Discussions、Projects |
| CI 自动化 | GitHub Actions，公开仓库标准 runner 免费 |
| 文档与展示 | mss-boot-docs + Cloudflare Pages，但控制发布次数 |
| 数据监控 | GitHub API weekly digest + Cloudflare Web Analytics |
| 社区答疑 | GitHub Discussions 为主，微信群只做同步通知 |
| 内容发布 | README/docs 为主，掘金或公众号作为中文分发 |
| 赞助 | GitHub Sponsors + README sponsor 区 |

AI 自动化优先级：

1. Weekly Digest agent：每周生成组织健康报告。
2. Issue Triage agent：自动分类、补问复现信息、推荐文档链接。
3. PR Guard agent：检查测试、文档、安全、发布影响。
4. Release agent：生成 changelog、升级影响、镜像 tag、回滚说明。
5. Docs Drift agent：根据 OpenAPI/config/routes 变化提示文档更新。
6. Growth agent：每周生成 good first issue 候选和内容选题。

成本控制：

- 第一阶段不用大模型全自动，先用 GitHub Actions + 脚本 + 模板完成 60%。
- LLM 只用于摘要、分类、FAQ 草稿、release 文案，不用于自动 merge 或自动发布。
- Cloudflare 前端发布严格门禁，避免消耗次数。
- 不同时维护太多社交渠道，先把 GitHub Discussions 和 docs 做扎实。

## 最高杠杆的 12 个任务

1. 组织级 `.github` 社区健康文件。
2. 四核心仓库 README 首屏统一定位。
3. `mss-boot-docs` 社区健康补齐。
4. CI 必过和分支保护。
5. SECURITY policy 和漏洞响应 SLA。
6. OpenSSF Scorecard workflow。
7. 每周组织健康 digest。
8. Issue/PR AI guard。
9. 10 到 20 个高质量 good first issue。
10. Agent Context Pack。
11. MSS Contract Manifest。
12. Policy Explain / Action Trace。

执行顺序建议：

- 第 1 周：1、2、3、4。
- 第 2 到 4 周：5、6、7、8、9。
- 第 2 到 3 个月：10、11。
- 第 4 到 6 个月：12。

## 最终建议

mss-boot-io 当前最强的资产不是 stars，而是已经有完整的框架、后台、前端、文档和 AI 记忆雏形。作为个人开发者，要把分数推到 10，不应该靠加班维护社区，而应该把组织设计成“AI 能帮你持续维护”的形态。

一句话路线：

把 mss-boot-io 打造成 agent-readable、governance-first、production-maintainable 的 Go 微服务后台开源组织。

## 2026-06-05 直接实施状态

本轮目标：在不依赖外部账号决策、不开启高成本社区渠道、不过度发布 Cloudflare 的前提下，先把可直接通过源码落地的开源组织评分项推到 9.2+ 准备态。

已直接实施：

1. 新增/完善组织级 `.github` 仓库：
   - 组织 profile 叙事改为 governance-first、agent-readable、production-maintainable。
   - 补齐 `CODE_OF_CONDUCT.md`、`CONTRIBUTING.md`、`SECURITY.md`、`SUPPORT.md`。
   - 补齐默认 issue templates、PR template 和 labels taxonomy。
   - 新增 reusable workflows：PR Guard、Issue Triage、Docs Drift、Weekly Digest、Release Draft。
2. 四个核心仓库补齐社区健康文件：
   - `mss-boot`
   - `mss-boot-admin`
   - `mss-boot-admin-antd`
   - `mss-boot-docs`
3. 四个核心仓库补齐自动化：
   - OpenSSF Scorecard workflow。
   - PR Guard、Issue Triage、Docs Drift、Weekly Digest、Release Draft。
   - Dependabot 覆盖 GitHub Actions；Go/npm 仓库覆盖对应依赖生态。
4. 安全扫描补强：
   - `mss-boot` 与 `mss-boot-admin` 增加 govulncheck。
   - `mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs` 增加 CodeQL。
5. CI 与发布可信度补强：
   - `mss-boot` CI 改为 `go-version-file: go.mod`，增加 `go test ./...`，取消 lint 软失败。
   - `mss-boot-admin-antd` 增加 PR CI：frozen install、ESLint、tsc、unit test、local build。
   - `mss-boot-docs` 增加 PR CI：frozen install 与 docs build。
   - Cloudflare 相关 workflow 改为 `pnpm install --frozen-lockfile`，不增加新的高频发布触发。
   - 旧发布 workflow 升级 `softprops/action-gh-release@v2`，替换废弃 `set-output`。
6. AI 协作叙事修正：
   - `mss-boot-admin/.github/copilot-instructions.md` 将多租户、虚拟模型、代码生成标记为历史/兼容能力，不再作为新功能主线。
   - `mss-boot/README.Zh-cn.md` 将旧的“代码生成工具”todo 替换为 AI 可读契约、发布治理与可观测能力。
7. 社区运营材料落盘：
   - 外部账号/组织设置待办：`open-source-operations-backlog.zh-CN.md`。
   - good first issue 任务工厂：`open-source-good-first-issue-factory.zh-CN.md`。
8. CI 可运行性修复：
   - `mss-boot` 全量 `go test ./...` 已改为无外部凭据/服务时可通过；对象存储、Redis 等集成测试在依赖未配置时跳过，虚拟模型测试改用内存 SQLite。
   - 修复 `mss-boot` Mongo lookup 关系字段解析，优先使用关联 ID 字段的 `bson` 名称。
   - `mss-boot-admin-antd` 统一 workflow pnpm 到 v9，以匹配 lockfile v9；本地 Node 24 会触发 Umi 旧依赖的 `http_parser` 兼容问题，验证使用 Node 18。
   - `mss-boot-admin-antd` 修复类型、lint、Jest 基线：补 `uuid` 声明、清理未使用 import、修正 localStorage test setup、同步登录页测试文案与快照。

实施后评分判断：

- 可通过源码直接实现的开源组织化能力，已从 6.4/10 推进到 9.2+ 准备态。
- 真实 GitHub 公开评分仍取决于外部设置是否完成，例如 branch protection、private vulnerability reporting、Discussions、Sponsors、secret scanning、rulesets、workflow required checks。
- OpenSSF Scorecard 的实际分数需要相关分支合并并在 GitHub Actions 中跑完后才能确认。

下一步最关键的外部动作：

1. 合并/发布 `.github` 组织仓库变更，让默认社区健康文件和 reusable workflows 生效。
2. 为四个核心仓库启用 branch protection / rulesets，并要求 CI、CodeQL、govulncheck、Scorecard 通过。
3. 启用 private vulnerability reporting，并确定公开安全联系入口。
4. 开启 Discussions，先用低维护成本分类：Q&A、RFC、Roadmap、Help Wanted、中文交流。
5. 从 good first issue 工厂中挑选 10 到 20 个任务创建公开 issue。
