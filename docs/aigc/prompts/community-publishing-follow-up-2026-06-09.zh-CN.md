# 外部社区发布后跟进记录

## 背景

- 记录时间：2026-06-09
- 承接文件：`external-community-publishing-execution-2026-06-08.zh-CN.md`
- 执行模式：leader 统一巡检，多 agent 分别检查 GitHub 社区状态、外部反馈、公开 docs 缺口。

## 巡检结论

- GitHub 仍然是唯一稳定反馈主入口。
- Dev.to、CSDN 暂无评论反馈。
- X 可确认推文存在，但没有可操作公开回复。
- Reddit `r/opensource` 帖子被 AutoModerator 移除，原因是发帖账号未满 1 年；不要争辩，不重复跨版发布。
- 掘金链接当前返回 404，不能作为有效反馈入口，后续需确认是否审核、隐藏或链接变化。
- OSCHINA 仍因当前 Chrome profile 无登录态暂缓。
- 知乎仍因手机号绑定/验证码要求暂缓，AI agent 不代填账号敏感信息。

## GitHub-first 回流规则

| 外部反馈类型 | GitHub 承接方式 |
| --- | --- |
| bug、部署失败、可复现问题 | GitHub Issue，附环境、命令、日志、复现步骤 |
| 架构、roadmap、取舍建议 | GitHub Discussions |
| 文档缺口、FAQ、教程缺口 | `mss-boot-docs` docs backlog 或直接补文档 |
| 低风险参与任务 | `good first issue` / `help wanted` |
| 安全问题 | 私有安全渠道，不在外部评论区展开 |

## 需要继续推进的事项

1. 在 Discussion `#380` 标注 Reddit 和掘金当前不能作为稳定入口。
2. 把社区运营流程写入公开 docs，避免后续重复解释。
3. 下一轮发布前复核 GitHub About / topics / homepage。
4. 清理仍指向 virtual model / generator-first / 低代码主线的外部描述。
5. `mss-boot-admin` 旧 issue 需要重新分流，尤其虚拟模型相关 issue 应标注降级状态。
6. Discussion `#99` 的数据表生成问题应沉淀到 FAQ，说明当前推荐路径是 Go model + migration，不再推荐虚拟模型作为新主线。
7. SQL migration / rollback RFC `mss-boot-admin#374` 适合邀请外部数据库迁移经验反馈。

## 巡检节奏

- T+24h / T+48h：检查 X、Dev.to、Reddit、掘金、CSDN。
- 第 1 周：每天检查 GitHub Discussions / Issues，真实反馈 48h 内响应。
- 7 天内：不再向 Reddit 其他社区重复发布。
- 每周：由 Weekly Digest 汇总社区、issue、PR、CI、failed workflow。
- 每月：输出 roadmap / release digest，优先补 quickstart、FAQ、release 可信度和 smoke evidence。

## 回复边界

- 不说“已达到 9.2+”，只能说“以 9.2+ 为验收目标推进”。
- 不求 star，不连续自顶，不复制粘贴刷屏。
- 不把外部平台评论当作最终决策，所有结论回流 GitHub。
- 不代填手机号、验证码、密码、实名信息或其他敏感账号信息。
- 不在公开评论区展开安全细节。

