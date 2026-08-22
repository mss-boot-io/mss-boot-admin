---
title: Agent 原生管理系统开发基础设施
order: 1
nav:
  title: 架构
  order: 2
description: mss-boot-admin 面向 Codex 等编码 Agent 的已实现基础设施、长期边界和演进路线
keywords: [agent native codex infrastructure generator mcp skills eval]
---

# Agent 原生管理系统开发基础设施

## 1. 文档目的

本文定义 `mss-boot-admin` 的长期完成形态、工程边界、阶段路线和验收标准。

项目不再只被理解为一个可运行的 Go 后台管理系统，也不以“在后台里增加聊天框”作为 AI 化目标。新的核心定位是：

> `mss-boot-admin` 是一套面向 Codex、Claude Code、other coding agents、Cursor 及其他编码 Agent 的管理系统开发基础设施。它提供稳定运行时、机器可读项目契约、确定性代码生成、标准 Agent Skills、统一 CLI、可重复开发环境、自动验证、能力评测和持续升级机制，使 Agent 在克隆仓库后可以直接进入开发闭环。

项目的最终用户既包括人类开发者，也包括编码 Agent。任何核心工程能力都必须同时满足“人能理解”和“机器能执行”。

### 当前实现状态

- P0–P7 的顶层契约、`mss` CLI、AdminModule 生成、Skills、MCP、Evals、Thin Host Blueprint
  和升级引擎已经落地；具体成熟度以 `.mss/capabilities.yaml` 为准。
- Supplier 已作为完整前后端生成和外部 Thin Host 黄金样例。
- `v1.3.0-rc.6` 已完成协调预览发布；唯一活动目标为 `v1.3.1` stable，当前稳定版仍是
  `v1.2.3`。
- P8 的既有横向 Admin 模块不会机械式整体迁移；新业务默认使用 `admin/modules/<name>`，
  老路径保持兼容维护。

## 2. 完成形态

最终目标流程如下：

```text
用户描述业务需求
  ↓
Agent 读取 AGENTS.md 和 .mss/project.yaml
  ↓
Agent 查询已有 capability，避免重复实现
  ↓
Agent 生成结构化 FeatureSpec / AdminModuleSpec
  ↓
mss CLI 校验规格并确定性生成基础代码
  ↓
Agent 实现非模板化业务规则
  ↓
mss verify 根据变更范围执行最小充分验证
  ↓
输出迁移、权限、安全、测试和兼容性报告
  ↓
提交可审查、可复现、可升级的 PR
```

完成形态必须达到以下结果：

1. Agent 不需要用户重复解释仓库目录和常用命令。
2. Agent 不需要通过模仿随机旧代码猜测新模块结构。
3. 新业务模块由结构化规格驱动，生成过程可重复、可审查、可验证。
4. 后端、前端、迁移、权限、菜单、国际化、测试和文档形成同一交付单元。
5. 项目提供统一 CLI，Skill 和 MCP 只调用 CLI，不各自实现第二套逻辑。
6. 所有自动修改支持 dry-run、幂等和变更清单。
7. “完成”由验证报告定义，而不是由 Agent 自述定义。
8. 基础设施升级能够应用到已经生成的业务系统，而不是只能复制一次模板。
9. Codex 是第一优先适配对象，但项目事实源保持工具中立。
10. 历史提示词、旧仓库路径和过期约定不会污染默认 Agent 上下文。

## 3. 非目标

当前路线明确不做以下事情：

- Admin 不再提供运行时动态建表、动态模型、虚拟 CRUD 或浏览器代码生成；这些能力不得成为新业务路径。
- 不让大模型直接生成所有重复代码并替代确定性生成器。
- 不把 MCP Server 设计成拥有独立业务逻辑的第二套后端。
- 不为每种编码 Agent 人工维护一份互相冲突的项目说明。
- 不要求 Agent 访问生产数据库、生产 Secret 或生产集群才能完成日常开发。
- 不把一次性 Prompt 数量当成 Agent 基础设施成熟度。
- 不在第一阶段大规模重写全部现有业务模块。
- 不在同一个 PR 中混合仓库迁移、架构重写和大量业务功能。

## 4. 设计原则

### 4.1 单一事实源

项目结构、能力、命令、模块规格和验证规则必须有唯一机器可读来源：

```text
AGENTS.md            人与 Agent 共同遵循的顶层规则
.mss/                机器可读项目契约
mss CLI              所有确定性操作入口
.agents/skills/      Agent 工作流编排
```

`CLAUDE.md`、other coding agents 指令和 Cursor Rules 只能是由事实源生成或引用的薄适配层。

### 4.2 规格优先

先生成和校验规格，再写代码：

```text
自然语言需求 → FeatureSpec → AdminModuleSpec → 代码与验证
```

规格用于表达实体、字段、权限、菜单、关系、数据范围、工作流和验收条件。

### 4.3 确定性优先

重复、机械和容易遗漏的代码必须由生成器负责，包括：

- 基础 Model、DTO、Repository、Service 和 Handler。
- Migration。
- OpenAPI 基础定义。
- 权限和菜单声明。
- 前端列表、表单、详情、Service 和类型。
- 国际化键。
- 基础单元测试、契约测试和 E2E 骨架。
- 文档骨架。

Agent 负责业务判断、特殊约束和复杂交互，不负责每次重新手写模板代码。

### 4.4 CLI 优先

所有能力先实现为可测试的 CLI 或内部 Go Package，再暴露给 Skill、CI 和 MCP。

```text
Skill ─┐
CI ────┼──> mss CLI ──> generator / validator / inspector
MCP ───┘
```

### 4.5 幂等和可回滚

所有写操作必须满足：

- 默认只生成计划或输出，不写文件；需要变更时显式使用 `--write` 或对应的受确认 apply 参数。
- 重复执行不会持续产生无意义 diff。
- 输出修改文件列表和摘要。
- 不自动提交 Git。
- 不执行不可逆生产操作。
- 失败时保留可诊断信息。

### 4.6 变更范围驱动验证

`mss verify --changed` 根据 Git Diff 计算验证集合，避免任何小改动都执行完整流水线，同时不允许漏掉迁移、权限、前端类型和文档漂移检查。

### 4.7 兼容优先、渐进迁移

现有横向目录继续运行；新模块采用垂直切片；旧模块逐个迁移。基础设施改造不得以一次性重写全部业务为前提。

## 5. 目标架构

```text
┌────────────────────────────────────────────────────────────┐
│                 Codex / Claude / other coding agents / Cursor          │
└──────────────┬──────────────────┬──────────────────────────┘
               │                  │
          AGENTS.md          Agent Skills
               │                  │
               └─────────┬────────┘
                         │
                 ┌───────▼────────┐
                 │    mss CLI     │
                 ├────────────────┤
                 │ doctor         │
                 │ context        │
                 │ setup/dev      │
                 │ module         │
                 │ inspect        │
                 │ verify         │
                 │ upgrade        │
                 │ eval           │
                 └───────┬────────┘
                         │
       ┌─────────────────┼───────────────────┐
       │                 │                   │
┌──────▼──────┐  ┌───────▼────────┐  ┌──────▼────────┐
│ .mss specs  │  │ generator      │  │ validators    │
│ schemas     │  │ codemods       │  │ reports       │
└─────────────┘  └────────────────┘  └───────────────┘
                         │
       ┌─────────────────┼───────────────────┐
       │                 │                   │
┌──────▼──────┐  ┌───────▼────────┐  ┌──────▼────────┐
│ Go backend  │  │ React frontend │  │ docs / tests  │
│ mss-boot    │  │ web/antd-v6    │  │ migrations    │
└─────────────┘  └────────────────┘  └───────────────┘
```

MCP Server 是 `mss CLI` 的薄协议适配层，不直接读写项目内部结构。

## 6. 目标仓库结构

```text
mss-boot-admin/
├── AGENTS.md
├── .agents/
│   └── skills/
├── .codex/
│   └── config.toml
├── .mss/
│   ├── project.yaml
│   ├── capabilities.yaml
│   ├── commands.yaml
│   ├── lock.yaml
│   ├── schemas/
│   ├── modules/
│   ├── architecture/
│   ├── evals/
│   └── reports/
├── cmd/
│   ├── mss/
│   └── mss-mcp/
├── internal/
│   └── mss/
│       ├── project/
│       ├── doctor/
│       ├── generator/
│       ├── inspector/
│       ├── verifier/
│       ├── upgrader/
│       └── eval/
├── admin/
│   └── modules/
│       ├── all/
│       ├── runtime/
│       └── supplier/
├── templates/
│   ├── application/
│   └── module/
├── tools/
│   ├── codemods/
│   └── contracts/
├── mss-boot/
├── web/antd-v6/
├── docs/
├── scripts/
├── compose/
└── Makefile
```

## 7. 核心规格

### 7.1 ProjectSpec

描述仓库级技术栈、路径、运行时和验证入口。

### 7.2 CapabilityCatalog

列出已有能力、稳定级别、所有者、相关代码和是否推荐用于新模块，防止 Agent 重复造轮子或继续强化历史能力。

### 7.3 AdminModuleSpec

定义管理模块的机器可读规格：

- 模块名、显示名和描述。
- 实体、表名和字段。
- 字段类型、校验、索引、搜索和展示。
- 关系和级联策略。
- 权限动作。
- 菜单和路由。
- 数据归属和访问范围。
- 工作流状态和转换。
- 前端页面能力。
- 测试和验收要求。

### 7.4 FeatureSpec

描述跨模块需求、影响范围、兼容要求和验收条件；可以引用多个 `AdminModuleSpec`。

### 7.5 ValidationPlan

由变更范围和项目契约自动计算，明确必须执行的命令、预期产物和跳过原因。

### 7.6 FoundationLock

记录生成项目所使用的基础设施版本、Blueprint 版本、模块版本和升级历史。

## 8. CLI 完成形态

### 8.1 环境与上下文

```shell
mss doctor [--format json]
mss context [--format json]
mss spec validate <spec-file> --format json
mss feature plan <feature-file> --format json
mss verify --changed --plan --format json
```

### 8.2 初始化和开发

```shell
mss setup
mss dev
mss dev --detach
mss dev status
mss dev logs backend
mss dev stop
```

### 8.3 规格和生成

```shell
mss spec init <name> --kind module
mss spec validate <file>
mss module generate <file>
mss module generate <file> --write
```

### 8.4 验证

```shell
mss verify --changed
mss verify --module <name>
mss verify --all
mss verify --format json
```

### 8.5 升级

```shell
mss upgrade status --format json
mss upgrade admin <version> --foundation <path> --format json
mss upgrade admin <version> --foundation <path> --apply --yes --format json
```

### 8.6 评测

```shell
mss eval list
mss eval run <case>
mss eval report
```

## 9. 标准 Agent Skills

当前提供：

```text
mss-project-onboarding
mss-new-application
mss-add-module
mss-add-field
mss-add-permission
mss-add-workflow
mss-debug-fullstack
mss-review-change
mss-upgrade-foundation
mss-release
```

每个 Skill 必须：

- 明确触发条件。
- 读取 `.mss` 事实源。
- 调用 `mss CLI`。
- 写明禁止事项。
- 给出验证命令。
- 输出结构化交付摘要。
- 不复制其他 Skill 的实现逻辑。

## 10. MCP Server 边界

MCP 首期只暴露稳定 CLI 能力。

只读工具：

```text
mss_get_project_context
mss_list_capabilities
mss_list_modules
mss_inspect_module
mss_get_change_impact
mss_get_validation_plan
```

写工具：

```text
mss_create_module_spec
mss_scaffold_module
mss_add_field
mss_sync_module
mss_run_validation
```

写工具默认 dry-run，并返回：

```json
{
  "changedFiles": [],
  "warnings": [],
  "validationPlan": [],
  "diffSummary": ""
}
```

## 11. 新模块架构

现有 `admin/apis/`、`admin/dto/`、`admin/models/`、`admin/service/` 横向目录继续兼容。
新业务默认采用垂直模块；Supplier 的真实结构为：

```text
admin/modules/supplier/
├── module.yaml
├── model_generated.go
├── dto_generated.go
├── service_generated.go
├── api_generated.go
├── migration_generated.go
├── authorization_generated.go
├── events_generated.go
├── custom.go
├── *_generated_test.go
└── AGENTS.md
```

前端对应：

```text
web/antd-v6/src/generated/modules/supplier/
├── SupplierPage.tsx
├── api.ts
├── contract.ts
├── query.ts
└── types.ts
```

生成器必须维护统一模块注册文件，禁止人工维护多个互相漂移的模块列表。

## 12. 统一验证模型

### L0：格式

- `gofmt`。
- Prettier。
- 生成文件格式稳定性。

### L1：静态检查

- Go vet / golangci-lint。
- TypeScript。
- ESLint。
- Schema 校验。

### L2：单元测试

- 受影响 Go Package。
- 受影响前端模块。

### L3：契约测试

- OpenAPI 合法性。
- API 与前端 Client 一致性。
- 路由、菜单和权限一致性。
- Migration 与 Model 一致性。

### L4：集成测试

- 数据库迁移。
- Redis、存储等可选依赖。
- 登录和 RBAC。

### L5：E2E

- 登录。
- 列表、创建、编辑、删除。
- 数据权限和越权拒绝。
- 移动端关键路径。

### L6：安全

- govulncheck。
- CodeQL。
- 依赖审计。
- Secret 扫描。
- 权限矩阵。

### L7：漂移检查

- 规格与生成代码。
- OpenAPI 与 Client。
- README 和项目版本。
- 能力目录与实际模块。

## 13. Agent Evals

基础设施必须通过任务集持续评估：

1. 新增标准 CRUD 模块。
2. 给现有模块增加字段并生成向前兼容 Migration。
3. 增加角色权限和数据归属限制。
4. 增加状态工作流。
5. 修复前后端接口不一致。
6. 从旧基础版本升级。
7. 修复一次真实 CI 故障。
8. 在不修改禁止目录的情况下完成需求。

评测指标：

```text
构建通过率
测试通过率
规格符合率
越权缺陷率
无关变更率
生成幂等率
人工修复次数
任务总耗时
Token 消耗
升级冲突率
```

## 14. 安全边界

- Agent 默认不读取生产 Secret。
- Setup 和测试不得要求生产 DSN。
- 生成器禁止写出仓库根目录之外的路径。
- MCP 写操作默认 dry-run。
- API 权限必须在后端强制执行，不能只隐藏前端按钮。
- 生成代码必须通过 SQL 注入、XSS、路径穿越和越权检查。
- 审计日志不得默认存储密码、Token 和敏感正文。
- 所有外部命令使用参数数组执行，禁止拼接未经验证的 Shell 文本。

## 15. 分支和提交策略

基础设施演进采用短周期、可回滚提交：

1. 先提交实现或规格，保证成果落盘。
2. 再执行格式化、单测和集成验证。
3. 发现问题后用后续修复提交收敛，不改写已推送历史。
4. 每个 PR 聚焦一个里程碑或一个可独立验收能力。
5. 生成器、Skill、MCP 和运行时改动分离审查。
6. 禁止把一次性迁移工作流长期保留在默认分支。

## 16. 演进路线

### P0：仓库可被 Agent 正确理解

交付：

- 顶层 `AGENTS.md` 重写。
- 清理绝对路径、旧仓库名和过时多租户说明。
- `.mss/project.yaml`。
- `.mss/capabilities.yaml`。
- `.mss/commands.yaml`。
- 目录地图和约束文档。
- `mss context` 最小实现。

验收：新会话 Agent 无需额外说明即可找到后端、前端、文档、测试和验证命令。

### P1：统一 CLI 和环境自检

交付：

- `mss doctor`。
- `mss setup`。
- `mss verify --changed/--all`。
- JSON 报告。
- Makefile 和 CI 统一调用 CLI。

验收：本地、Codex Cloud 和 GitHub Runner 使用同一套无交互命令。

### P2：规格与确定性模块生成器

交付：

- `AdminModuleSpec` JSON Schema。
- YAML 解析和语义校验。
- Go 后端模块生成。
- Migration、权限和菜单生成。
- React 前端模块生成。
- 国际化、测试和文档生成。
- dry-run、幂等和漂移检查。

验收：一份规格可生成可运行的完整管理模块，连续执行两次无额外 diff。

### P3：Agent Skills

交付：首批标准 Skills、参考资料和脚本。

验收：用户只描述业务需求，Agent 能自动选择正确工作流并完成验证。

### P4：开发进程管理和完整本地闭环

交付：`mss dev`、健康检查、日志、测试账号和可重复种子数据。

验收：一条命令启动完整环境，一条命令停止并清理。

### P5：MCP Server

交付：stdio MCP、只读工具、dry-run 写工具、Codex 项目配置。

验收：Agent 可通过结构化工具查询项目、生成规格、执行生成和读取验证报告。

### P6：Evals 和质量基线

交付：标准任务集、自动评测、基线报告和回归门禁。

验收：可以量化比较基础设施版本对 Agent 成功率的影响。

### P7：Blueprint 和持续升级

交付：`mss new app`、Foundation Lock、Upgrade Recipe、Codemod 和升级报告。

验收：由旧 Blueprint 创建的业务系统可升级到新版本，并保留业务自定义。

### P8：现有模块垂直化

交付：逐步迁移身份、组织、权限、审计、通知和任务模块。

验收：新结构成为唯一推荐路径，旧横向结构进入兼容维护。

## 17. 里程碑 Definition of Done

任何阶段只有同时满足以下条件才算完成：

- 设计与机器可读契约已经提交。
- 实现已经提交。
- 至少有单元或契约测试。
- CI 使用与本地一致的命令。
- 文档和示例可复现。
- 没有硬编码个人目录。
- 没有依赖生产凭据。
- 输出明确的兼容性和升级影响。
- 失败路径有稳定退出码和错误信息。
- 相关 Agent Eval 不退化。

## 18. 当前实施基线

截至 `v1.3.1` stable 准备，仓库已经形成以下可执行闭环：

1. 顶层 `AGENTS.md` 与 `.mss/` 提供人机共享事实源。
2. `mss context`、`doctor`、`setup`、`dev`、`verify`、`eval`、`new app` 和 `upgrade`
   使用统一 CLI 实现。
3. 标准 Skills 调用 CLI，不复制第二套生成、校验或发布逻辑。
4. AdminModule 规格确定性生成 Supplier 的后端、迁移、权限、菜单、前端、测试、E2E 和文档。
5. `management-system` Blueprint 生成 31 个受管文件的 Thin Host，并通过二次幂等和外部消费者门禁。
6. MCP 保持 CLI 的薄适配层，写操作默认 dry-run。
7. RC6 已从一个精确 merged-main 提交完成 Framework、Admin、Admin Web 和 Root 预览列车。

下一条可执行步骤是通过 Pull Request 合并 stable 准备，冻结新的精确 `main` 提交，重跑
checkpoint、feature-freeze、外部 Thin Host、三数据库/API 注册表和内置浏览器证据，再发布
`v1.3.1`。P8 继续采用按真实需求逐模块迁移，而不是一次性重写既有横向实现。
