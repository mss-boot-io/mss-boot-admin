# 社区发布与 good-first 拆分执行记录

## 背景

- 执行时间：2026-06-09
- 承接：
  - `domestic-community-publishing-drafts-2026-06-09.zh-CN.md`
  - `community-ops-heartbeat-2026-06-09-0930z.zh-CN.md`
- 范围：国内社区发布、CSDN 格式修复、前端/docs 安全迁移 RFC 的 good-first 拆分。

## 国内平台发布状态

### 掘金

- 标题：把开源项目质量交给 GitHub 和 AI：mss-boot-io 的一轮 PR 治理记录
- 状态：已发布。
- URL：`https://juejin.cn/spost/7648967155561201727`
- 配置：分类 `后端`，标签 `GitHub`，话题 `AI 编程`。

### CSDN

已发布并修复格式的文章：

1. `mss-boot 部署排障笔记：Go 微服务后台从本地运行到配置检查`
   - URL：`https://blog.csdn.net/weixin_40814867/article/details/161809693`
   - 修复：去掉 raw Markdown 标记，例如 `##`、代码围栏和 Markdown 列表；改成 CSDN 友好的中文小节、段落和命令行文本。
   - 公开页复核：无 raw Markdown 标记；正文未重复标题。

2. `把开源项目质量交给 GitHub 和 AI：mss-boot-io 的一轮 PR 治理记录`
   - URL：`https://blog.csdn.net/weixin_40814867/article/details/161818170`
   - 修复：去掉正文里的重复标题；去掉 `#` / `##` / Markdown 列表；改成中文小节和自然段。
   - 公开页复核：无 raw Markdown 标记；正文未重复标题。

注意：

- 两篇文章重新发布后 CSDN 页面显示“发布成功！正在审核中”，公开页已能看到修改时间和修复后的正文。
- CSDN 会自动给部分英文技术词前后加空格，例如 `Go + React`、`Node .js`，这是平台渲染/排版行为，不属于原稿 Markdown 标记问题。
- 未勾选 GitCode 备份。

### 知乎

- 状态：未发布，保留草稿。
- 原因：知乎编辑器仍显示 `Markdown 语法输入中`，点击 `确认并解析` / 电脑预览后仍未把 Markdown 可靠转换成富文本。为避免把 raw Markdown 原文公开发布，已停止发布。
- 后续：需要先把知乎正文转换成适合知乎富文本编辑器的格式，再发布；如出现手机号、验证码、实名等敏感步骤，仍交给维护者处理。

## good-first 拆分

### `mss-boot-admin-antd`

父 RFC：`https://github.com/mss-boot-io/mss-boot-admin-antd/issues/101`

已拆分：

- `https://github.com/mss-boot-io/mss-boot-admin-antd/issues/103`
  - 标题：`docs: inventory frontend security migration impact`
  - 目标：整理 Umi / Vite / router 安全迁移影响面。
- `https://github.com/mss-boot-io/mss-boot-admin-antd/issues/104`
  - 标题：`docs: add frontend migration smoke and rollback checklist`
  - 目标：整理前端迁移 smoke 与回滚检查表。

父 issue 已评论回链：

- `https://github.com/mss-boot-io/mss-boot-admin-antd/issues/101#issuecomment-4659213269`

### `mss-boot-docs`

父 RFC：`https://github.com/mss-boot-io/mss-boot-docs/issues/32`

已拆分：

- `https://github.com/mss-boot-io/mss-boot-docs/issues/33`
  - 标题：`docs: inventory docs toolchain migration impact`
  - 目标：整理 dumi / Umi / Vite 文档站迁移影响面。
- `https://github.com/mss-boot-io/mss-boot-docs/issues/34`
  - 标题：`docs: add docs migration smoke and rollback checklist`
  - 目标：整理 docs 迁移 smoke 与回滚检查表。

父 issue 已评论回链：

- `https://github.com/mss-boot-io/mss-boot-docs/issues/32#issuecomment-4659213577`

## 后续动作

1. 巡检掘金和 CSDN 是否有评论或审核状态变化；有效反馈回流 GitHub Discussions / Issues。
2. 知乎发布前先改成富文本友好的正文，不再直接投递 Markdown。
3. 下一轮社区治理时，优先邀请贡献者从 `#103`、`#104`、`#33`、`#34` 这四个 good-first 文档任务开始。
4. 不重复向 Reddit 发布；上一轮 Reddit 因账号年龄被 AutoModerator 移除。

