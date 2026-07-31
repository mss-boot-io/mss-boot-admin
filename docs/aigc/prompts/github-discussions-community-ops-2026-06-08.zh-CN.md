# GitHub Discussions 社区运营执行记录

## 背景

- 执行时间：2026-06-08
- 执行入口：https://github.com/orgs/mss-boot-io/discussions
- 执行方式：使用维护者已登录 Chrome Profile，通过 GitHub 网页操作。
- Leader 原则：多 agent 可以并行起草内容、检查语气、准备互动建议；所有对外发布、评论、置顶、关闭、转 issue 等动作由 leader 统一执行和验收。

## 已发布 Discussions

### #377 Welcome to the mss-boot-io community

- URL：https://github.com/orgs/mss-boot-io/discussions/377
- Category：Announcements
- 目的：建立社区入口，说明 mss-boot 当前定位、核心仓库、贡献入口和中英文交流方式。
- 状态：已发布并在 Discussions 首页置顶。

### #378 Roadmap: toward an AI-governed Go admin framework

- URL：https://github.com/orgs/mss-boot-io/discussions/378
- Category：Ideas
- 目的：公开 roadmap，收敛项目方向为 AI-governed Go microservice admin framework。
- 重点：强调 Agent-readable context、governance-first admin workflows、release discipline、contributor experience。
- 范围说明：虚拟模型是降级且即将废弃功能，只接受存量安全收口，不继续作为主宣传方向。

### #379 中文交流入口：问题、部署、贡献和使用反馈

- URL：https://github.com/orgs/mss-boot-io/discussions/379
- Category：General
- 目的：为国内开发者提供中文交流入口，承接部署、使用反馈、贡献咨询和文档缺口。
- 语气：中文主导，强调复现步骤、版本、日志、截图和 issue/discussion 分流。

### #380 Help wanted: reviewers, docs, tests, and first contributions

- URL：https://github.com/orgs/mss-boot-io/discussions/380
- Category：General
- 目的：号召社区贡献 review、文档、测试、前端体验、部署示例和 issue 到 FAQ 的沉淀。
- 语气：英文主导，强调小而可验证的 PR 和 review 贡献同样重要。
- 已补充评论：列出本周第一批贡献候选方向，包含 docs、troubleshooting、mobile smoke checklist、backend tests、dependency/release review。

### #381 English support: questions, adoption, and contribution help

- URL：https://github.com/orgs/mss-boot-io/discussions/381
- Category：General
- 目的：承接英文问题、采用评估、贡献咨询和文档缺口。
- 原则：英文支持仍以 GitHub 为单一事实源，外部反馈需要回流到 GitHub。

### #382 How to ask for help: issue and discussion checklist

- URL：https://github.com/orgs/mss-boot-io/discussions/382
- Category：Q&A
- 目的：说明何时用 Discussion、何时用 Issue，以及高质量问题所需的版本、环境、命令、日志和复现步骤。
- 安全提示：不要公开 secrets、tokens、passwords、private keys 或生产凭据。

### #383 FAQ seed: quick start, deployment, login, migration, and smoke tests

- URL：https://github.com/orgs/mss-boot-io/discussions/383
- Category：Q&A
- 目的：建立 FAQ 种子池，用于把重复问题转为 docs、FAQ 或 good first issue。
- 首批 FAQ 候选：本地启动、前端对接 alpha API、Go/Node/pnpm 版本、TimescaleDB/PostgreSQL、Redis、登录失败、migration、Swagger、环境策略、bug report、贡献入口。

### #384 Release and environment policy: alpha, beta, and production

- URL：https://github.com/orgs/mss-boot-io/discussions/384
- Category：Announcements
- 目的：公开 alpha/beta/prod 的发布策略和前端 Cloudflare 发布约束。
- 重点：后端 beta 可以更快迭代，公开 beta 必须通过冒烟测试，prod 只发 tag 版本。

### #385 Show and tell: share your mss-boot setup, deployment, or workflow

- URL：https://github.com/orgs/mss-boot-io/discussions/385
- Category：Show and tell
- 目的：征集真实部署、工作流、截图、架构图和 AI 协作使用经验。
- 安全提示：不要公开 secrets、credentials、private URLs 或客户数据。

## 已补充的社区索引

- 已在 #377 Welcome 中追加社区索引，链接 #378 到 #385。
- 目的：外部平台导流后，新用户可以直接从 Welcome 进入 roadmap、中文入口、英文支持、FAQ、环境策略和 show-and-tell。

## 已互动的历史讨论

### #99 How to generate a data table?

- URL：https://github.com/orgs/mss-boot-io/discussions/99
- 已将维护者历史回复标记为 answer。
- 已补充 2026 更新说明：虚拟模型已经降级并进入废弃路径，新项目推荐使用 Go model + migration。
- 目的：避免社区继续把虚拟模型理解为主推荐路径。

## 多 Agent 社区运营模式

### Leader Agent

职责：

- 统一发布口径、节奏和验收。
- 统一执行外部网页发布、评论、置顶、转 issue 等动作。
- 避免多个 agent 同时用同一账号发出不一致内容。
- 将所有已执行动作落盘到 `mss-boot-docs/aigc`。

### GitHub Community Agent

职责：

- 维护 Discussions、issue、PR、good first issue、roadmap、release digest。
- 将社区反馈分流到 issue、FAQ、docs 或 good first issue。
- 优先保证 GitHub 是单一事实源。

### China Community Agent

职责：

- 为掘金、OSCHINA、CSDN、知乎准备中文内容。
- 国内内容重教程、部署、问题排查、中文答疑、开源治理和贡献者转化。
- 不做硬广告，所有内容都要带 GitHub/docs 链接和可复现信息。

### Global Community Agent

职责：

- 为 Dev.to、X/Twitter、Reddit、Hacker News、Product Hunt 准备英文内容。
- 海外内容重工程可信、透明取舍、AI-governed 定位和反馈请求。
- Reddit/Hacker News 不做直接营销，优先以 seeking feedback 语气参与。

### Score-to-10 Agent

职责：

- 评估开源评分到 10 分的剩余缺口。
- 优先推荐 GitHub Actions、AI 自动化、低成本社区运营。
- 过滤不值得投入的方向，例如已降级的虚拟模型扩展。

## 后续节奏

第 1 周：

- 每天检查 Discussions 是否有新回复。
- 新回复 48 小时内响应。
- 能沉淀为工程问题的反馈转 issue。
- 能沉淀为文档缺口的反馈转 docs backlog。
- 能低风险交给社区的任务转 good first issue。

第 2 周：

- 发布第一篇中文技术内容到掘金或 OSCHINA。
- 发布第一篇英文技术内容到 Dev.to。
- X 准备 3 条 build-in-public 动态。
- Reddit 只在选定社区用 feedback 语气轻量尝试。

验收指标：

- GitHub Discussions 有清晰入口。
- 外部平台内容回流到 GitHub。
- 至少 1 个社区反馈被转化为 issue/docs/good first issue。
- 至少 1 个外部贡献者或潜在贡献者被明确邀请参与 review。

## 多 Agent 输出汇总

### GitHub Community Agent

- GitHub Discussions / Issues 是单一事实源。
- Discussions 用于开放讨论、使用经验、路线澄清；Issues 用于可复现 bug、明确功能请求、文档缺陷、可执行任务。
- 虚拟模型 / dynamic model / code generation 只作为 legacy / degraded / deprecating capability 回答，不作为主宣传，不承诺新增能力。
- 建议 2026-06-08 到 2026-06-14 的节奏：
  - 2026-06-08：完成入口巡检和置顶。
  - 2026-06-09：Roadmap 反馈收口。
  - 2026-06-10：中文入口集中维护。
  - 2026-06-11：Help wanted 转化日。
  - 2026-06-12：技术问题清障日。
  - 2026-06-13：周末轻维护。
  - 2026-06-14：发布 weekly digest。

### China Community Agent

- 国内平台优先级：掘金、CSDN、OSCHINA、知乎。
- 首轮中文内容：
  - 掘金：《用 Go 搭一个可治理的微服务后台：mss-boot 本地启动与核心模块初探》
  - CSDN：《mss-boot 部署排障笔记：Go 微服务后台从本地运行到配置检查》
  - OSCHINA：《我们为什么做 mss-boot：一个面向 AI 协作时代的治理型 Go 微服务后台框架》
  - 知乎：《AI 协作时代，Go 微服务后台框架为什么需要“治理型”能力？》
- 国内内容不做硬广告，重点是教程、部署、排障、开源治理和贡献者转化。

### Global Community Agent

- 海外首轮不做 launch，而是 seeking feedback。
- Dev.to 第一篇建议：`Building a Go Admin Framework Around Governance, Not Just CRUD`
- X/Twitter 三条方向：
  - governance-heavy Go + React admin framework。
  - Action/Hook abstraction 的 Go 取舍问题。
  - AI-assisted OSS workflow 不是让 AI 拥有决策，而是让维护面可见。
- Reddit 首选 `r/golang`，用 architecture feedback 语气，不要求 stars，不 cross-post。
- Hacker News / Product Hunt 暂缓，等 demo、README、文档、release、截图和维护者 24 小时互动准备充分。

### Score-to-10 Agent

- 从 9.2 到 10 的缺口主要是持续社区信任信号，而不是单纯“有没有社区”。
- 本周最高杠杆动作：
  - Discussions 种子内容。
  - AI 社区记忆。
  - GitHub 自动化周报/提醒。
  - FAQ 记忆池。
  - 社区健康基线。
- 不建议投入：
  - 大规模外部账号铺量。
  - 自建复杂社区平台。
  - 重视觉宣传物料。
  - 未验证的多语言全量翻译。
  - 让 AI 自动对外回复关键问题。
