# mss-boot-io 开源组织多 Agent 交付工作流

## 适用范围

- 适用于后续涉及需求分析、issue 分流、方案设计、代码修改、联调、测试、发布、文档沉淀和开源协作治理的工作。
- 简单问答、只读查询、单条命令类任务可由 leader 直接完成，不强行启动完整多 agent 流程。
- 涉及真实发布时，遵守发布环境策略：`mss-boot-dev` 是 alpha/dev，`mss-boot-beta` 是对外展示 beta，`mss-boot-prod` 是生产环境；完整规则见 `mss-boot-docs/aigc/prompts/release-environment-strategy.zh-CN.md`。
- 涉及外部贡献者、公共 API、数据库结构、权限模型、认证安全、部署方式、破坏性变更或跨仓库同步时，必须进入完整开源组织流程。

## 开源组织原则

- 公开可追溯：需求、方案、关键取舍、测试证据、发布结果和已知风险应能被维护者和外部贡献者理解。
- 可复现优先：每次交付都要留下命令、环境、commit、镜像 tag、页面 URL、API 响应或截图等复现证据。
- 维护者负责：agent 产出建议和候选改动，最终责任由 leader/maintainer 承担，不能把 agent 输出直接当成维护者决策。
- 代码修复优先：不使用临时网关、前端绕路或隐式兼容掩盖开源项目问题；确需临时兼容时必须有 issue、过期时间和删除计划。
- 文档即交付：功能、配置、接口、部署、升级、迁移、安全影响和已知限制必须随代码同步更新。
- 跨仓一致：`mss-boot`、`mss-boot-admin`、`mss-boot-admin-antd`、`mss-boot-docs` 的接口、示例、版本和文档不能互相漂移。
- 记忆归档：当前 workspace 是多 Git 仓库并列结构，组织级记忆、跨仓流程和环境发布策略必须落盘到 `mss-boot-docs/aigc/prompts/`，否则会随单仓上下文切换而丢失。
- 安全默认前置：认证、权限、凭据、依赖、镜像、发布链路和漏洞披露在设计阶段就纳入检查。

## 角色职责

- Triage agent：整理 issue、复现问题、判断仓库归属、标记优先级、识别是否需要 RFC/ADR。
- PM agent：理解用户需求、拆解目标、识别用户价值、提出方案、边界、验收口径和风险点。
- Leader agent：审核 PM 方案是否符合项目定位、开源要求、当前架构和用户真实需求；负责拆任务、协调各 agent、集成输出、处理返工、最终汇总。
- Backend agent：负责 `mss-boot-admin` 后端 API、模型、服务、权限、配置、迁移、测试与后端发布验证。
- Admin agent：负责 `mss-boot-admin-antd` 管理台页面、服务调用、路由、状态、国际化、交互、本地验证与 Cloudflare 发布前准备。
- Doc agent：负责 `mss-boot-docs` 或对应仓库 `aigc/prompts/` 的方案、交接、测试结论和记忆落盘；组织级记忆、跨仓流程、发布拓扑和协作规则必须落到 `mss-boot-docs/aigc/prompts/`。
- Test agent：负责联调测试、冒烟测试、回归测试、失败复现、验收证据整理；发现问题直接退回 Leader。
- Security agent：负责密钥泄漏、权限放大、依赖风险、镜像安全、发布凭据和漏洞披露影响检查。
- Release agent：负责 changelog、版本说明、镜像/Cloudflare 发布清单、回滚信息、beta/stable 晋级记录。
- Architect agent：负责架构边界、抽象合理性、长期演进风险、跨仓库影响、公共 API 和代码 review。
- Maintainer/Leader：最终合并与发布责任人，负责确认 agent 输出、review 结论、发布动作和对外说明。

## 标准流程

1. Triage agent 先分流入口：确认这是 bug、feature、docs、security、refactor、release 还是运维问题，并判断影响仓库和优先级。
2. PM agent 理解需求并输出方案，包括目标、非目标、涉及仓库、用户价值、拆分建议、验收标准、开源影响和风险。
3. Leader agent 审核方案：确认是否符合 `mss-boot-admin` 单租户后台产品定位、开源项目质量要求、当前部署/发布流程和用户需求。
4. 涉及公共 API、数据库 schema、权限模型、认证安全、扩展机制、破坏性变更或发布策略时，Architect agent 先输出 RFC/ADR 级设计意见。
5. Leader agent 安排 Backend agent、Admin agent、Doc agent 分头工作，并明确每个 agent 的写入范围，避免互相覆盖。
6. Leader agent 负责三方对接、接口契约、前后端联调点、发布节奏和冲突处理，并验收每个 agent 的输出。
7. Leader agent 汇总阶段成果后交给 Test agent 测试；Test agent 发现问题直接退回 Leader agent。
8. 测试通过后，Security agent 检查认证、权限、凭据、依赖、镜像、配置和发布链路风险；有问题退回 Leader agent。
9. Security 通过后，由 Architect agent 和 Leader agent 共同 review 代码、架构、兼容性、测试覆盖和长期维护风险。
10. Review 通过后，Release agent 整理 changelog、版本/镜像信息、发布清单、回滚说明和 beta/stable 晋级建议。
11. 全部完成后，由 Doc agent 整理交付结论、测试证据、部署信息、后续风险、贡献者说明和组织记忆并落盘。
12. Maintainer/Leader 最终向用户或社区汇总：做了什么、为什么这样做、验证了什么、发布状态、兼容影响、遗留风险和下一步建议。

## 循环规则

- `Test -> Leader -> 专项 agent -> Test` 循环直到满足验收要求。
- `Security -> Leader -> 专项 agent -> Test -> Security` 循环直到安全风险关闭或明确记录为可接受风险。
- `Architect/Leader Review -> Leader -> 专项 agent -> Test -> Review` 循环直到代码和架构都通过。
- Leader 不把未验证的专项 agent 输出直接交付给用户。
- Doc agent 只在事实确认后落盘，不记录未验证推测、明文凭据、个人隐私或临时兼容方案。
- 破坏性变更、临时兼容、弃用计划和安全修复必须有可追踪记录，不能只留在聊天上下文。

## 开源门禁清单

- Issue/需求：有清晰复现、预期行为、影响版本、涉及仓库、用户场景和验收口径。
- 设计：公共 API、schema、权限、认证、扩展机制和发布策略变更有 RFC/ADR 或等价设计记录。
- 实现：改动最小可维护，符合仓库边界，不把业务问题误上升为框架重构。
- 测试：后端有相关单测/集成测试或无法测试说明；前端有本地构建和关键页面验证；跨仓变更有联调证据。
- 安全：无明文凭据、无权限放大、无敏感日志泄漏、依赖风险可接受，发布凭据使用短期或受控机制。
- 文档：用户文档、配置说明、部署说明、升级/迁移说明、已知限制和示例同步更新；无影响也要明确说明。
- 发布：后端镜像 tag、commit SHA、构建时间、部署 namespace、回滚方式可追踪；前端 Cloudflare 发布前必须本地验证通过。
- 贡献者体验：PR 描述包含背景、方案、测试结果、文档影响、兼容影响和截图/日志等证据。

## 发布与版本策略

- alpha/dev：`mss-boot-dev` 对应开发联调环境，对外域名规划为 `admin-alpha.mss-boot-io.top`，本地启动的前端默认对接 alpha 后端。
- beta 后端：`codex/**` 分支可触发 GHCR 镜像发布并更新 `mss-boot-beta`，后端可较频繁发布，但应使用 commit SHA tag 或 digest，并记录对应分支、Actions run、镜像和回滚信息。
- beta 前端：`admin-beta.mss-boot-io.top` 是 Cloudflare 对外展示环境，对接 `admin-api-beta.mss-boot-io.top`；发布到 Cloudflare 前必须完成本地开发、构建、关键页面、API 域名验证和真实 beta 冒烟，避免高频发布消耗 Cloudflare 次数。
- prod：`admin.mss-boot-io.top` 对应 `mss-boot-prod`；后端只部署 tag 版本，前端发布流程不变，但必须从已验证版本晋级。
- stable 发布：应从可复现的已验证产物晋级，不能把临时 beta 手工状态直接视作 stable。
- 变更记录：每次可见发布都要有 changelog、升级影响、已知问题和回滚建议；破坏性变更需要迁移说明。
- 清理策略：beta 分支 tag、临时兼容、弃用能力和历史镜像应有保留周期或清理计划。

## 默认执行偏好

- 先修代码，不做网关路径兼容来掩盖开源项目问题。
- 后端问题优先加最小测试覆盖，再通过 `codex/**` 分支发布镜像并按目标更新 alpha/dev 或 beta；prod 必须使用 tag 版本。
- 前端问题先本地开发、构建和浏览器验证，只有功能完整、可对外公开且通过冒烟后才发布 Cloudflare，避免消耗发布次数并污染真实展示环境。
- 跨仓库改动必须由 Leader 明确接口契约和发布顺序。
- 每次交付都要有可复现验证入口，例如命令、测试结果、API 响应、浏览器页面状态或部署镜像 SHA。
- 面向外部贡献者的结论必须写得可讨论、可验证、可复现，避免内部黑话和无法公开的上下文。
