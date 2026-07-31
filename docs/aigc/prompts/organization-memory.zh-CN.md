# mss-boot-io 组织级 AI 记忆

## 记忆来源

- 读取时间：2026-06-04
- 更新确认：2026-06-04，`mss-boot-admin` 与 `mss-boot-admin-antd` 已补齐 `aigc` 目录
- 工作区：本地多仓库 workspace，根目录形态为 `<workspace>/mss-boot-io`
- 工作区形态：多 Git 仓库并列的 workspace，`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs` 各自是独立 Git 仓库。
- 关键约束：组织级记忆、跨仓库流程、发布策略和多 agent 协作规则必须落盘到 `mss-boot-docs`，否则在多 Git workspace 中容易随单仓上下文切换而丢失。
- 已发现的 `aigc` 目录：
  - `mss-boot/aigc`
  - `mss-boot-admin/aigc`
  - `mss-boot-admin-antd/aigc`
  - `mss-boot-docs/aigc`
- 组织级文档项目：`mss-boot-docs`

## 组织项目地图

`mss-boot-io` 当前工作区由四个核心仓库组成：

- `mss-boot`：企业级微服务基础框架，主责 HTTP/Gin、gRPC、配置、响应抽象、鉴权、安全、存储、任务、可观测与虚拟资源等框架能力。
- `mss-boot-admin`：基于 `mss-boot` 的后台服务，主责权限治理、组织管理、系统配置、通知任务、国际化、监控统计、OAuth2/Token、上传、事件等业务后台能力。
- `mss-boot-admin-antd`：基于 React、Ant Design v5、Umi Max v4 的前端管理台，页面与服务接口大体对应后端 API 模块。
- `mss-boot-docs`：组织级文档项目，基于 Dumi 构建，承接框架指南、Admin 产品文档、测试沉淀、交接说明、AI 注解协同规范和跨仓库协作记忆。

## 核心产品定位

- `mss-boot` 是框架底座，目标是在服务代码保持极简的前提下，提供完善工程化和扩展能力。
- `mss-boot-admin` 是开箱即用的单租户后台管理系统，当前产品主线聚焦权限治理、组织管理、系统配置、通知任务、国际化、监控统计与 AI 注解协同。
- 动态模型、虚拟模型、代码生成、多租户相关实现可能仍存在于代码中，但在文档中已被标记为历史存量或非主线能力；改动前应先确认目标是维护兼容、降级弱化，还是彻底清理。
- `mss-boot-docs` 是组织级知识入口，跨项目说明默认应优先沉淀到这里，而不是分散到业务仓库根目录。

## 当前阶段记忆

- 三期治理与运营能力补强已有交接记录，包含 CustomDept 数据权限自动填充、WebSocket 实时通知、登录与操作审计日志、扩展监控维度、Playwright E2E 测试。
- P3 AI 注解协同流程落地已评估为完成，重要改动应通过评审、计划、交接、评估四类产物沉淀。
- P4 集成与扩展护栏已评估为完成，国际化、对象存储/上传、WebSocket、API-first 扩展能力应纳入治理、安全、文档和测试检查清单。
- P5 历史能力降级与叙事收口已评估为完成，动态模型与模板/代码生成均为 L3 弱化能力：保留但非主线，生产环境建议使用标准 Controller 模式。
- 产品叙事应避免把 `mss-boot-admin` 表述为低代码平台或以动态模型/代码生成为核心卖点。
- alpha/dev 与 beta 后端均归属同一个 devops 集群：alpha/dev 使用 `mss-boot-dev`，对外域名为 `https://admin-alpha.mss-boot-io.top` / `https://admin-api-alpha.mss-boot-io.top`；beta 使用独立 namespace `mss-boot-beta`，Cloudflare 前端 `https://admin-beta.mss-boot-io.top` 对接后端 API `https://admin-api-beta.mss-boot-io.top`。不要再用 `~/.kube/baas.yaml` 推断 alpha/beta 后端状态。
- `mss-boot-beta` 配置可复制 `mss-boot-dev` 形态，但数据库和 Redis 复用 dev 实例时必须使用新库/独立 Redis 空间，避免污染 dev 数据。
- prod 环境规划为新部署 namespace `mss-boot-prod`：Cloudflare 前端 `https://admin.mss-boot-io.top` 对应 prod 后端，后端只部署打 tag 的版本；prod 数据库、Redis、Secret、外部集成配置应独立于 dev/beta。
- 当前后端 GitHub 镜像运行时缺少 `Asia/Shanghai` 时区数据，Postgres DSN 在 Kubernetes Secret 中使用 `TimeZone=UTC`；若未来镜像内补齐 tzdata 或嵌入 Go timezone 数据，再评估恢复业务期望时区。
- `mss-boot-dev` 的运行凭据集中在 Kubernetes Secret `mss-boot-runtime`，文档和记忆中只记录资源名和配置形态，不写入明文密码、Token 或完整 DSN。
- 前后端发布节奏区分：后端可较频繁发布到 alpha/dev 和 beta，优先用 commit SHA tag 或 digest 记录；前端不能随意发布 Cloudflare，必须等功能完整、可对外公开且冒烟通过后再发布。`admin-beta` 是外部展示环境，必须展示发布前完整功能，不能暴露半成品。
- 2026-06-04 `/app-config` 弹框问题是在旧拓扑下用 `mss-boot-dev` 验证修复：`codex/fix-language-public-beta` 发布 `ghcr.io/mss-boot-io/mss-boot-admin:351b50db09a8ccd02d3197383e5434aeff4aa73f`，`mss-boot-dev` 已更新到该 SHA；`/admin/api/languages/public?pageSize=999`、`/admin/api/app-configs/profile`、`/admin/api/app-configs/base` 均返回 200。后续按新拓扑应将对外 beta 展示切到 `mss-boot-beta`。
- 2026-06-04 基于 GitHub 当前数据完成开源组织评分与 AI 时代增长方案，当前综合评分为 6.4/10；核心路线是将 `mss-boot-io` 打造成 agent-readable、governance-first、production-maintainable 的 Go 微服务后台开源组织。详细方案见 `mss-boot-docs/aigc/prompts/open-source-ai-era-growth-plan.zh-CN.md`。
- 2026-06-05 执行开源组织评分提升实施：新增/完善组织级 `.github` 社区健康文件、默认 issue/PR 模板、标签规范、reusable workflows；四核心仓库补齐 Scorecard、govulncheck/CodeQL、Dependabot、PR Guard、Issue Triage、Docs Drift、Weekly Digest、Release Draft、社区健康文件与 CI 门禁；`mss-boot-admin` AI 指令中的多租户/虚拟模型/代码生成已调整为历史兼容能力。可直接通过源码实现的开源组织化能力已推进到 9.2+ 准备态，真实公开评分仍依赖 GitHub 外部设置完成。
- 2026-06-05 使用 `gh` 复核 GitHub 公网状态：可见仓库 38 个，公开 34 个，私有 4 个，总 stars 128，总 forks 39；社区健康分为 `.github` 12、`mss-boot` 62、`mss-boot-admin` 75、`mss-boot-admin-antd` 62、`mss-boot-docs` 37。该结果反映的是 main 分支公网状态，不包含本地尚未提交/推送的治理改动；后续推送合并并完成外部设置后需重新复核。
- 2026-06-05 外部账号/组织设置待办已落盘到 `mss-boot-docs/aigc/prompts/open-source-operations-backlog.zh-CN.md`；good first issue 任务工厂已落盘到 `mss-boot-docs/aigc/prompts/open-source-good-first-issue-factory.zh-CN.md`。
- 2026-06-05 为保证新增 CI 门禁不是“纸面门禁”，同步修复 `mss-boot` 与 `mss-boot-admin-antd` 的测试基线：`mss-boot` 全量 `go test ./...` 可在无外部 Redis/MySQL/AWS SSO 时通过；`mss-boot-admin-antd` 在 Node 18 + pnpm 9 下通过 frozen install、`pnpm tsc`、`pnpm lint:js`、`pnpm test -- --runInBand`、`pnpm build:local`。本地 Node 24 不适合该前端仓库的 Umi 4.0.x 工具链验证。
- 2026-06-05 已执行可直接落地的社区运营动作：同步 `.github`、`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs` 的 labels taxonomy；创建 12 个低风险 `good first issue`；给 `mss-boot-admin` 历史 open issue 做轻量标签分流；给 `mss-boot` 与 `mss-boot-admin` 的 Dependabot PR 增加 `needs-triage` 与 `area/backend`。执行记录见 `mss-boot-docs/aigc/prompts/open-source-community-ops-execution-2026-06-05.zh-CN.md`。

## 角色协作记忆

默认采用面向开源组织的 leader-first 多 agent 工作方式：

- 后续复杂交付默认开启多 agent 工作流，完整规则见 `mss-boot-docs/aigc/prompts/multi-agent-delivery-workflow.zh-CN.md`；该流程已从普通项目交付升级为开源组织治理流程，目标是公开可追溯、可复现、可审计、可维护。
- 用户默认与 leader 沟通，leader 负责目标、范围、优先级、阶段边界、里程碑识别、agent 协调、返工闭环、验收汇总和对外总结，角色入口在 `mss-boot-admin/aigc/prompts/roles/leader-role.zh-CN.md`。
- 面向社区协作时，先由 triage agent 做 issue/需求分流：确认问题类型、复现信息、影响仓库、优先级、是否需要 RFC/ADR。
- PM agent 先理解需求并出方案；leader 审核方案是否符合项目要求、开源质量要求和用户真实需求，再拆分执行。
- 架构设计师主责 `mss-boot`，负责框架边界、抽象治理和长期演进。
- 后端开发主责 `mss-boot-admin`，负责 API、DTO、模型、服务、权限、配置和业务逻辑，角色入口在 `mss-boot-admin/aigc/prompts/roles/backend-developer-role.zh-CN.md`。
- 后台前端 admin agent 主责 `mss-boot-admin-antd`，负责页面、路由、状态、服务对接、交互体验、本地验证和 Cloudflare 发布前准备，角色入口在 `mss-boot-admin-antd/aigc/prompts/roles/frontend-developer-role.zh-CN.md`。
- 文档整理 doc agent 主责 `mss-boot-docs` 与各仓库 `aigc/prompts/`，负责方案、交接、测试结论、发布信息和组织记忆沉淀。
- 测试 test agent 负责联调验证、冒烟测试、回归结论和失败复现；发现问题直接退回 leader 返工。
- security agent 负责密钥、权限、依赖、镜像、配置和发布链路风险检查；release agent 负责 changelog、版本说明、镜像/Cloudflare 发布清单和回滚信息。
- architect 与 leader 在测试和安全检查通过后共同 review 代码和架构；review 有问题则回到 leader，leader 再协调对应 agent 修复。

当任务涉及联调、验收、回归或交接，应尽早引入开发测试视角，并把验证入口、测试数据、风险点和结论沉淀到 `mss-boot-docs`。

多 agent 交付闭环为：`Triage 分流 -> PM 方案 -> Leader 审核 -> 必要时 RFC/ADR -> Backend/Admin/Doc 分工 -> Leader 集成验收 -> Test 测试 -> Security 检查 -> Architect + Leader Review -> Release 整理 -> Doc 落盘 -> Leader/Maintainer 汇总`。测试、安全或 review 不通过时回到 Leader 返工，循环直到满足要求。

开源组织门禁默认包括：issue/PR 信息完整、跨仓影响清楚、公共 API/schema/权限/认证变更有设计记录、测试证据可复现、安全风险已检查、文档和 changelog 同步、发布镜像或前端版本可追踪。

若用户仅回复 `1`，默认解释为继续当前任务或执行下一步；如果有多条分支，优先推进当前主任务路径上优先级最高的下一步。

## 架构记忆

- `Action + Controller` 是 `mss-boot` / `mss-boot-admin` 的核心生产力来源，当前结论是优先保留主架构，不做高风险重写。
- 资源动作语义优先于 ORM 细节：Controller 表达资源动作，Action 执行动作，Provider 适配 GORM / MGM / K8S 等后端。
- Hook + Scope 是多租户、鉴权、审计、统计、缓存清理等横切能力的统一注入点，但需要治理执行顺序、错误语义和幂等性。
- 主要风险是 Router -> Controller -> Action -> Hook -> DB 调用链较长，复杂问题需要跨层排查。
- 演进优先级：先补 Action 链路可观测性，再规范 Hook，再对热点模块拆分复杂 `Control`，最后推进类型化与生成增强。
- 新模块优先使用显式 `Create` / `Update` 分离；旧模块按错误率和改动频率逐步治理。

## 单租户化记忆

- 当前产品方向已从多租户叙事收敛到单租户后台系统，多租户相关实现仍可能存在于代码中。
- 后端去多租户评估基线认为：`mss-boot-admin` 触点约 47 个文件，`mss-boot` 触点约 9 个文件，属于中大型重构。
- 去多租户推荐 PR 顺序：核心骨架去租户、模型与控制器去租户、缓存/存储/虚拟模型收口、迁移与文档收尾。
- 高风险点：鉴权与数据范围变化导致权限放大、迁移脚本与 `tenant_id` 索引处理不完整、虚拟模型 `MultiTenant` 语义变化。
- 缓存结论需按条件理解：若保留多租户，应补 tenant-aware key/helper 与失效治理；若推进单租户化，应同步重构缓存 key 与失效逻辑，避免旧 tenant 前缀残留。
- 前端多租户主要集中在 `/tenant` 路由、`Tenant` 页面、`tenant.ts` 服务、`typings.d.ts`、`multiTenant` 字段和 i18n 文案，尚未形成统一租户上下文驱动请求架构。
- 前端去多租户推荐顺序：下线路由与菜单文案、删除页面和 API 客户端、更新 typings 与 `multiTenant` 展示逻辑、执行 lint/build/关键页面回归并联调后端。

## 工程行为准则

- 改代码前先定位所属仓库和责任边界，不把业务修复误上升为框架改造。
- 优先做最小侵入改动，沿用既有包结构、命名、Option 模式、Controller/Action 分层和服务组织方式。
- 新增 HTTP 资源能力时，优先复用 `pkg/response/controller` 和已有 action 模式。
- 配置能力优先复用 `pkg/config.Init` 与既有 provider 机制。
- HTTP 鉴权优先沿用 `response.AuthHandler` 和现有中间件体系。
- 修改后优先执行与变更点强相关的测试，再考虑大范围校验。
- 发现文档与代码不一致时，先说明证据来源和时间点，不在没有证据时判定问题归属。

## 目录与命令记忆

### mss-boot

- 模块名：`github.com/mss-boot-io/mss-boot`
- Go 版本：`go 1.26`
- 关键目录：
  - `core/`：服务管理、日志与运行时组件
  - `pkg/config/`：配置结构与 Provider 能力
  - `pkg/response/`：Controller、Action、返回与错误抽象
  - `pkg/middlewares/`：认证、限流、用户上下文
  - `pkg/security/`、`pkg/store/`：安全与 OAuth2 相关能力
  - `virtual/`：虚拟资源模型、API 和 Action 编排
  - `proto/`：gRPC 协议定义与生成文件
- 常用校验：`go test ./...`

### mss-boot-admin

- 模块名：`github.com/mss-boot-io/mss-boot-admin`
- Go 版本：`go 1.25.3`
- 依赖框架：`github.com/mss-boot-io/mss-boot`
- 关键目录：
  - `apis/`：后台 API
  - `models/`：数据模型
  - `dto/`：请求与响应 DTO
  - `service/`：业务服务
  - `router/`：路由装配
  - `config/`：配置与初始化
  - `docs/`：Swagger 生成产物
  - `aigc/prompts/`：后端角色、leader 角色、Action 架构评审、去多租户计划与阶段交接文档
- 常用命令：
  - 数据库迁移：`go run main.go migrate`
  - 生成接口信息：`go run main.go server -a`
  - 启动服务：`go run main.go server`
  - 测试：`go test ./...`
- 当前关键记忆文件：
  - `aigc/prompts/roles/leader-role.zh-CN.md`
  - `aigc/prompts/roles/backend-developer-role.zh-CN.md`
  - `aigc/prompts/action-architecture-review.zh-CN.md`
  - `aigc/prompts/tenant-removal-handoff-summary.zh-CN.md`
  - `aigc/prompts/phase3-governance-operations-handoff.zh-CN.md`
  - `aigc/prompts/p3-ai-annotation-evaluation.zh-CN.md`
  - `aigc/prompts/p4-extension-guardrails-handoff.zh-CN.md`
  - `aigc/prompts/p5-legacy-capability-handoff.zh-CN.md`

### mss-boot-admin-antd

- 技术栈：React 18、Ant Design v5、Umi Max v4、TypeScript
- 关键目录：
  - `config/routes.ts`：前端路由
  - `src/pages/`：页面入口
  - `src/services/admin/`：OpenAPI 生成或维护的接口服务
  - `src/locales/`：国际化资源
  - `aigc/prompts/`：前端角色、前端多租户评估、去多租户影响计划与交接摘要
- 常用命令：
  - 开发：`pnpm start` 或 `npm run start`
  - OpenAPI 生成：`npm run openapi`
  - 类型检查：`npm run tsc`
  - 测试：`npm run test` 或 `npm run test:coverage`
- 当前关键记忆文件：
  - `aigc/prompts/roles/frontend-developer-role.zh-CN.md`
  - `aigc/prompts/frontend-multi-tenant-cache-evaluation.zh-CN.md`
  - `aigc/prompts/frontend-tenant-removal-impact-plan.zh-CN.md`
  - `aigc/prompts/frontend-tenant-removal-handoff-summary.zh-CN.md`

### mss-boot-docs

- 技术栈：Dumi 2
- 组织定位：组织级文档项目
- 关键目录：
  - `docs/`：组织文档内容
  - `docs/admin/`：Admin 产品、治理、测试、部署、安全与路线图
  - `docs/coding/`：开发编码指南
  - `docs/guide/`：框架使用指南
  - `aigc/prompts/`：AI 角色、协作与提示词记忆
- 常用命令：
  - 开发：`pnpm start`
  - 构建：`pnpm run build`

## 文档与提示词落盘规则

- 当前 workspace 是多 Git 仓库并列结构，不是单仓库；组织级记忆必须以 `mss-boot-docs` 作为长期可信入口。
- 组织级、跨仓库、发布流程、多 agent 协作、环境拓扑、验收规则等记忆必须落盘到 `mss-boot-docs/aigc/prompts/`，不能只写在业务仓库、临时对话或未持久化上下文中，否则会丢失。
- 新生成的提示词、AI 记忆和规范文档必须写入对应仓库的 `aigc/prompts/` 或其子目录。
- 组织级、跨仓库文档优先写入 `mss-boot-docs`；仓库内角色、计划、评审、交接、评估类文档写入对应仓库的 `aigc/prompts/`。
- 不要把提示词或 AI 记忆文件直接写入仓库根目录。
- 文件名使用小写英文和连字符，中文版本使用 `.zh-CN.md` 后缀。
- 同名文件已存在时优先更新原文件；需要保留历史时追加日期后缀。
- 文档应保持开源协作语气：基于可见代码给出可验证结论，说明评估时点、适用范围、关键假设，不写入凭据、私有地址或个人隐私。

## 后续工作默认判断

- 涉及框架抽象、通用能力、配置 Provider、响应 Action、服务生命周期：优先进入 `mss-boot`。
- 涉及后台 API、权限、菜单、组织、配置、通知、任务、监控、日志、告警、Token/OAuth2：优先进入 `mss-boot-admin`。
- 涉及页面、路由、表格表单、国际化显示、接口服务调用：优先进入 `mss-boot-admin-antd`。
- 涉及说明、测试计划、验收结果、路线图、交接沉淀、AI 注解规范：优先进入 `mss-boot-docs`。
- 涉及跨仓库需求时，应先收敛目标和边界，再分别实现后端、前端、文档与测试沉淀。
- 涉及里程碑、收尾或提交时，先检查功能、测试、文档和服务可交付状态，再分别处理多仓库提交与交付汇报。
