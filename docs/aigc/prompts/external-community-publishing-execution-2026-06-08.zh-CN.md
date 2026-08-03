# 外部社区发布执行记录

## 背景

- 记录时间：2026-06-08
- 执行模式：leader 统一控制，多 agent 并行复核中文渠道、海外渠道和开源质量边界。
- 主目标：受控扩散 mss-boot-io 开源项目，把外部反馈回流到 GitHub Discussions / Issues / Docs。
- 质量原则：不刷屏，不求 star，不夸大成熟度，不宣传未验收的评分达成，不宣传虚拟模型等已降级/即将废弃方向。

## 多 agent 分工

- Leader：负责最终判断、外部发布动作、GitHub 承接、记忆落盘。
- China Publishing Agent：复核掘金、CSDN、知乎、OSCHINA 语境和中文互动话术。
- Global Publishing Agent：复核 X、Dev.to、Reddit 语境和海外社区节奏。
- Quality & Open Source Standard Agent：复核开源质量、传播边界、反馈回流路径。

验收结论：

- 采用“受控扩散 + GitHub-first 承接”。
- Hacker News、Product Hunt 暂缓。
- Reddit 仅保留 `r/opensource` 本轮发布，`r/golang` 因规则明确禁止 AI-generated content 作为帖子/评论，跳过且不绕规则重发。
- 不发布“9.2+ 已达成”这类不可独立验证表述，只能表述为“以 9.2+ 为验收目标推进”。

## 已执行发布

### GitHub Discussions

- 入口：`https://github.com/orgs/mss-boot-io/discussions`
- 承接帖：`https://github.com/mss-boot-io/mss-boot-admin/discussions/380`
- 本轮补充评论：`https://github.com/mss-boot-io/mss-boot-admin/discussions/380#discussioncomment-17224388`
- 作用：把 X、Dev.to、Reddit、掘金、CSDN 的外部反馈统一回流到 GitHub。

### X / Twitter

- 标题：mss-boot-admin governance positioning post
- URL：`https://x.com/lwnmengjing/status/2064002598915678322`
- 状态：已发布。
- 备注：后续不连续自顶，不无关 @，只在有真实问题时回复。

### Dev.to

- 标题：Building a Go Admin Framework Around Governance, Not Just CRUD
- URL：`https://dev.to/lwnmengjing/building-a-go-admin-framework-around-governance-not-just-crud-5fb7`
- 状态：已发布。
- 备注：定位为架构反馈文，不是 launch / star 请求。

### Reddit

- `r/opensource`
  - 标题：Looking for open-source governance feedback on a Go admin project
  - URL：`https://www.reddit.com/r/opensource/comments/1u0axfq/looking_for_opensource_governance_feedback_on_a/`
  - 状态：已发布。
  - 备注：Discussion flair；后续只回复直接问题，不重复跨版推广。
- `r/golang`
  - 状态：跳过。
  - 原因：社区规则明确禁止 GPT/AI-generated content 作为帖子、目标或评论。

### 掘金

- 标题：用 Go 搭一个可治理的微服务后台：mss-boot 本地启动与核心模块初探
- URL：`https://juejin.cn/spost/7648882056709275711`
- 状态：已发布。
- 备注：分类为后端；标签选择流程中最终包含后端相关标签。

### CSDN

- 标题：mss-boot 部署排障笔记：Go 微服务后台从本地运行到配置检查
- URL：`https://blog.csdn.net/weixin_40814867/article/details/161809693`
- 状态：已发布，页面提示“发布成功！正在审核中”。
- 备注：文章定位为部署排障笔记；使用原创、全部可见。

## 暂缓和阻塞项

### OSCHINA

- 状态：暂缓。
- 原因：当前 Chrome profile 打开 OSCHINA 时显示“登录/注册”，没有有效登录态。
- 后续：维护者完成登录后再发布中文开源项目定位文。

### 知乎

- 状态：账号要求阻塞。
- 原因：写文章页已填好标题和正文，但点击发布时要求绑定手机号并发送验证码。
- 后续：需要维护者完成账号手机号/验证码动作；AI agent 不代填手机号、验证码或其他敏感账号信息。
- 已准备标题：AI 协作时代，Go 微服务后台框架为什么需要“治理型”能力？

### Hacker News / Product Hunt

- 状态：暂缓。
- 原因：当前更适合工程治理反馈，不是面向终端用户的公开 launch；需要稳定 demo、截图、FAQ、快速响应机制和更强的英文 quickstart。

## 互动规则

- 外部平台只做必要回复，不做连续自顶。
- 有 bug、部署失败、复现问题：引导到 GitHub Issues，并要求版本、环境、启动命令、关键日志、复现步骤。
- 有架构、功能、路线图建议：引导到 GitHub Discussions。
- 有安全问题：引导到安全反馈渠道，不在外部评论区展开细节。
- 有“AI 概念过度”质疑：说明 AI 只辅助整理上下文、风险、测试计划、文档缺口，维护者负责最终判断。
- 有 self-promotion 质疑：低姿态解释是在收集开源治理反馈，尊重社区规则和版主处置。

## 推荐回复模板

部署/启动问题：

```text
感谢反馈。方便的话补充一下 Go/Node/pnpm 版本、数据库类型、启动命令、关键日志，以及是否执行过迁移。能复现的问题我们会整理到 GitHub Discussions / Issues 和 docs，方便后续统一维护。
```

AI 概念质疑：

```text
这个质疑合理。这里说的 AI 协作不是让 AI 接管代码或决策，而是辅助整理 PR 说明、风险点、测试计划、文档缺口和 Issue 上下文。维护者仍然负责最终判断。
```

Reddit self-promotion 质疑：

```text
Fair point. I shared it here because it is open source and I’m looking for feedback on governance/RBAC/docs rather than promotion. If this is not a good fit for the subreddit, I’m fine with the moderators removing it.
```

外部反馈回流：

```text
这个问题适合沉淀成公开讨论。我们会把结论同步到 GitHub Discussions / Issues / docs，避免只留在外部平台评论区。
```

## 后续动作

1. 24-48 小时内巡检 X、Dev.to、Reddit、掘金、CSDN 的真实回复。
2. 有效反馈必须回流 GitHub，并记录来源 URL、分类、处理状态。
3. 7 天内不再向 Reddit 其他社区重复发布。
4. 下一轮发布前，先补齐 GitHub repo About 中与当前定位冲突的旧描述，例如仍指向 virtual model 的表述。
5. OSCHINA 需要维护者重新登录后再继续。
6. 知乎需要维护者完成手机号绑定/验证码后再继续。
7. 继续用 GitHub Discussions 作为社区事实源，优先提升 quickstart、FAQ、贡献路径、CI 绿灯率和 release 可信度。

