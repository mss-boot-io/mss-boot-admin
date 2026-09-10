---
title: Blueprint 与升级
order: 4
description: v1.3.7 正式稳定版 Thin Host 文件所有权、三方升级、验证和恢复合同
---

# Blueprint 与升级

本页向人类解释 v1.3.7 Thin Host 的升级合同；Agent 的执行权威仍是目标仓库的
`AGENTS.md`、`.mss/` 和适用 Skill。v1.3.7 是当前协调稳定版，v1.3.5 与 v1.3.6 是
永久停止的不可变部分发布，不能作为升级目标或依赖来源。Docs 网站异步发布，不阻断
v1.3.7 组件采用和三方升级。

## Blueprint 来源

正式 v1.3.7 Root Release 的 `mss` 与 `mss-mcp` 内置同一个 `management-system`
Blueprint，并把版本、完整源提交、构建时间和仓库身份写入工具及生成结果。采用者不需要
Foundation checkout；本地编译或源码目录不能替代这条来源链。

创建过程先返回只读计划，只有显式确认后才原子写入。目标路径必须受限，未知文件导致
失败，生成结果固定完整协调版本的公共依赖与冻结锁，并写入同源 manifest、lock 和快照。
两次相同生成应产生稳定输出。

## 文件所有权

| 类型 | 示例 | 升级行为 |
| --- | --- | --- |
| Blueprint 管理 | 组合入口、项目合同、公共配置胶水 | 通过旧基线、当前文件和目标基线三方合并 |
| 生成管理 | 菜单、路由、API 与 locale 投影 | 从 AdminModule 等规格重新生成并检查漂移 |
| 业务所有 | `internal/modules/<name>`、业务页面与测试 | 自动升级保持原字节，显式冲突交给业务处理 |
| 未知 | 用户新增且未登记的文件 | 自动升级必须保留，不静默纳入或删除 |

`.mss/blueprint-manifest.json` 保存受管文件摘要，`.mss/lock.yaml` 保存当前 Distribution
和升级记录。不得手改摘要或复制其他项目的 manifest 伪造一致。

## 标准 v1.3.7 升级流程

升级前必须**备份** Git 仓库、环境配置、数据库和业务数据，并确认恢复演练有效。安装
正式 v1.3.7 工具后，先确认 `mss` 与 `mss-mcp` 报告相同版本、源提交和构建时间；再确认
目标 Thin Host 仍含 `.mss/blueprint-manifest.json`。

```shell
mss --version
mss-mcp --version
mss upgrade status --format json
mss upgrade admin v1.3.7 --format json
mss upgrade admin v1.3.7 --apply --yes --format json
mss upgrade status
mss doctor --strict
mss verify --all
mss upgrade admin v1.3.7 --format json
```

第一条 `mss upgrade admin v1.3.7` 是只读计划，比较旧 Blueprint 基线、当前工作树与
v1.3.7 新基线，只列出 create、update、preserve 和 conflict，不写文件。逐项审查计划并
清零冲突后，才运行第二条 `mss upgrade admin v1.3.7 --apply`。应用使用事务式暂存，先
写受管文件，保留业务和未知文件，最后更新快照与锁。

随后运行严格环境诊断和完整验证。最后一条 `mss upgrade admin v1.3.7` 是幂等性证明：
结果必须没有 create、update、delete 或 conflict；对显式定制默认注册表的只读
`preserve` 说明可以保留，但文件字节不能变化。

## 手工拼装或丢失 manifest

手工拼装、丢失 manifest 或无法证明 `.mss/blueprint-manifest.json` 来源的仓库，不能
直接使用三方升级。不要从其他仓库复制 manifest，也不要让 Agent 根据当前文件猜摘要。

正确路径是在独立空目录使用正式 v1.3.7 工具生成新的 Thin Host，验证其公共依赖和冻结
锁后，按所有权迁入业务规格、业务自有代码、手写前端、配置意图与数据迁移。先让新基线
完整通过 `mss doctor --strict` 和 `mss verify --all`，再以正常 PR 替换旧部署。旧数据库
需要单独的迁移与恢复计划，不能因为文件迁移成功就跳过数据验证。

## 冲突、失败与恢复

- 计划失败不写入任何文件；先修复版本、路径、来源或 manifest 问题后重新计划；
- 存在 conflict 时禁止应用；由拥有该文件的一方决定合并内容，然后重新生成计划；
- 应用失败应恢复原工作树并保留脱敏诊断报告，不允许留下半写快照；
- 数据库迁移失败必须阻断部署；回退同时恢复匹配的代码、配置、锁、数据库快照和业务数据；
- 已发布 v1.3.7 身份不可移动或覆盖，发布后修复进入新的补丁版本；
- Docs 页面或站点失败只进入独立文档修复流程，不触发组件回滚。

## Agent 审查清单

人类把升级任务交给 Agent 时，应要求它报告工具身份、目标提交、备份状态、只读计划、
每个冲突的所有权决定、应用结果、完整验证和最终 no-op。Agent 不得为了消除冲突而删除
未知文件、改写业务文件、提交本地 `replace` 或跳过测试。
