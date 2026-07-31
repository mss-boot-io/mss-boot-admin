# mss-boot-io 双线社区建设方案

## 背景

- 记录时间：2026-06-08
- 用户已在可由 Codex Chrome 插件操作的 Profile 中登录第一批社区账号。
- 国内社区使用中文内容；国外社区使用英文内容。
- 目标：以个人开发者可承受的成本，依赖 AI 和 GitHub 流水线尽量提高开源组织影响力、可信度和贡献者转化。

## 总原则

1. GitHub 是单一事实源：代码、issue、PR、release、roadmap、Discussions 优先沉淀在 GitHub。
2. 国内社区重“可落地、可复现、可交流”：教程、部署、问题排查、中文答疑、版本说明。
3. 海外社区重“可信工程叙事”：README、release notes、technical write-up、trade-off、Show HN/Dev.to/Reddit 的非营销式表达。
4. 不做纯广告式推广：所有外部内容必须带清晰问题、命令、截图、代码链接、验证方式或贡献入口。
5. 外部账号发布、评论、私信、提交表单都属于对外动作，Codex 执行前必须让维护者确认。
6. 虚拟模型是降级且即将废弃的功能，不作为社区宣传重点；只在存量用户维护场景中低投入说明。

## 定位话术

中文主定位：

> mss-boot 是面向 AI 协作时代的治理型 Go 微服务后台框架，重点在权限、配置、审计、发布、文档和 Agent 可读工程上下文。

英文主定位：

> mss-boot is an AI-governed Go microservice admin framework focused on RBAC, configuration, auditability, release discipline, and agent-readable engineering context.

不建议继续作为主线宣传的表达：

- 低代码生成平台。
- 动态模型主线。
- 单纯后台模板。
- 泛 AI 聊天框。

## 国内社区打法

### 掘金

用途：中文技术文章和系列教程。

推荐内容：

- 《用 mss-boot 5 分钟启动一个 Go 后台管理项目》
- 《AI Agent 如何参与开源项目治理：以 mss-boot 为例》
- 《Go 后台系统里的权限、菜单、API 与 Casbin 如何协同》
- 《个人开发者如何用 GitHub Actions 管理开源项目质量》

节奏：每周 1 篇，优先教程和复盘。

语气：工程化、可复现、少口号。文章必须包含 GitHub 链接、运行命令、适用人群和下一步贡献入口。

### OSCHINA

用途：项目介绍、版本发布、开源社区曝光。

推荐内容：

- 项目档案更新。
- v0.7.x 版本发布说明。
- 征集贡献者：文档、测试、前端体验、部署案例。
- 月度开源治理进展。

节奏：版本发布时同步，平时每 2 周 1 次。

语气：突出开源项目成熟度、贡献路径、中文交流友好。

### 知乎

用途：观点型长文和问答。

推荐内容：

- 《AI 时代，开源后台框架还应该解决什么问题？》
- 《个人开发者维护开源项目，如何靠 AI 降低社区运营成本？》
- 《为什么 mss-boot 把文档当成 AI Agent 的组织记忆？》

节奏：每月 1-2 篇，不追求高频。

语气：解释取舍、讲项目治理，不要写成产品广告。

### CSDN

用途：中文搜索流量和问题排查。

推荐内容：

- 安装部署教程。
- 本地开发环境。
- 常见报错排查。
- TimescaleDB、Redis、前后端联调、Cloudflare 发布流程。

节奏：每周或每两周 1 篇，适合复用文档内容改写。

语气：标题清楚、步骤完整、错误场景可搜索。

### 微信公众号

用途：低频沉淀和中文社群入口。

推荐内容：

- 月度进展。
- 版本发布。
- 外部贡献者感谢。
- 真实 issue 修复复盘。

节奏：每月 1 篇即可，避免个人维护成本过高。

## 海外社区打法

### Dev.to

用途：英文技术文章。

推荐内容：

- `Building an AI-governed Go admin framework`
- `How mss-boot uses docs as memory for AI agents`
- `A practical RBAC and admin workflow in Go`
- `Maintaining an open-source project as a solo developer with GitHub Actions and AI`

节奏：每 2 周 1 篇。

语气：少宣传，多讲技术细节、trade-offs、limitations、what changed。

### X / Twitter

用途：build in public、短动态、版本发布、截图和动图。

推荐内容：

- 每周 2-3 条短更新。
- PR 合并和 release 的简短说明。
- 贡献者感谢。
- AI workflow 的小片段。

语气：短、具体、透明。不要连续刷屏。

### Reddit

用途：参与相关讨论，不优先硬发推广贴。

推荐社区方向：

- `r/golang`
- `r/opensource`
- `r/selfhosted`
- `r/vuejs`
- `r/devops`

操作原则：

- 先回复已有问题，再考虑发项目介绍。
- 发帖必须明确“我做了什么、解决什么问题、想要什么反馈”。
- 不在禁止 self-promotion 的社区硬发。
- 首发主题建议是寻求反馈，而不是宣布产品。

### Hacker News

用途：成熟后做一次高质量 `Show HN`。

前置条件：

- 英文 README 首屏清楚。
- Demo 或截图可访问。
- release 稳定。
- 文档有 quickstart。
- 项目问题和限制写清楚。

建议标题：

> Show HN: mss-boot, an AI-governed Go microservice admin framework

### Product Hunt

用途：海外正式发布节点，而不是日常运营。

前置条件：

- 官网/文档站完整。
- 截图、短视频、英文介绍准备好。
- beta 环境稳定。
- GitHub release 和 issue/PR 治理看起来可信。

节奏：等 9.2+ 工程可信度稳定后再发，不急。

### LinkedIn

用途：吸引工程负责人、企业用户和长期维护者。

推荐内容：

- 项目架构取舍。
- AI-assisted engineering governance。
- Release discipline。
- 开源维护复盘。

节奏：每月 1-2 条。

## GitHub 社区建设

优先级最高，因为所有外部推广最终都应回流到 GitHub。

建议启用或完善 GitHub Discussions 分类：

- Announcements
- Q&A
- RFC
- Show and Tell
- Help Wanted
- Chinese Community

建议维护节奏：

- issue 首响：48 小时内。
- PR 首评：48 小时内，外部首个 PR 尽量 24 小时内。
- 每周一次社区 digest。
- 每月一次 roadmap/release digest。
- 每次合并外部贡献，在 release note 或 digest 中感谢贡献者。

## 首批 14 天内容计划

第 1-3 天：

- GitHub Discussions：发布 welcome / contribution guide / roadmap 三个基础帖。
- 掘金：中文项目定位和 5 分钟上手文章。
- Dev.to：英文项目定位和 AI-governed 工程叙事文章。

第 4-7 天：

- OSCHINA：项目介绍和 v0.7.x 进展。
- X：连续 3 条短动态，分别介绍定位、release、贡献入口。
- Reddit：只选择 1 个社区，以 feedback 语气发帖或先参与讨论。

第 8-14 天：

- CSDN：部署/联调排查教程。
- 知乎：AI 时代开源后台框架观点文章。
- GitHub：整理第一周反馈，转成 issue、FAQ 或 good first issue。

## AI 自动化建议

可交给 GitHub Actions / AI 处理的工作：

- Weekly community digest 草稿。
- issue/PR 标签建议。
- release note 初稿。
- 外部平台文章草稿生成。
- 外部评论回复建议。
- good first issue 月度批量生成。
- 文档 FAQ 从 issue 中自动提取。

维护者仍需人工判断的工作：

- 对外发布最终确认。
- 社区争议回复。
- roadmap 优先级。
- 合并高风险 PR。
- 安全问题披露。
- 账号、支付、实名、验证码、权限配置。

## 评分推进路径

7.5 到 8.5：

- 完成中英文定位统一。
- 建立 GitHub Discussions。
- 每个核心仓库保持至少 5 个高质量 good first issue。
- 外部平台每周至少 1 个有效内容触点。

8.5 到 9.2：

- 形成可复用的 release digest、community digest、contributor thank-you 机制。
- 至少 3 个外部贡献者有被合并的 PR。
- 文档站成为对外唯一权威入口。
- 海外内容至少覆盖 Dev.to、X、Reddit 中的两个渠道。

9.2 到 10：

- 有稳定贡献者梯队。
- 有项目 showcase。
- 有公开 roadmap 和 RFC 流程。
- release 具备 SBOM、attestation、checksum、compatibility matrix。
- GitHub issue、PR、Discussions、docs、release 之间形成可追踪闭环。

## 下一步可执行动作

在维护者确认后，Codex 可以使用已登录的 Chrome Profile 执行：

1. 在 GitHub Discussions 创建首批社区基础帖。
2. 在掘金创建中文 5 分钟上手草稿。
3. 在 Dev.to 创建英文项目定位草稿。
4. 在 OSCHINA 更新项目介绍或发布版本动态。
5. 在 X 准备 3 条 build-in-public 短动态。
6. 将所有外部发布链接回填到 `mss-boot-docs/aigc`，并按周生成社区运营记录。

