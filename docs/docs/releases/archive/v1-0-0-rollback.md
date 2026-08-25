---
title: v1.0.0 回滚与恢复
order: 5
nav:
  title: 发布
  order: 3
description: 已发布 v1.0.0 部署失败后的 forward-fix、代码回退与完整数据恢复合同
keywords: [v1.0.0 rollback restore forward-fix recovery]
---

# v1.0.0 回滚与恢复

> 历史版本：本页仅保留不可变发布、升级与恢复证据，不用于新项目。当前使用路径请从 [v1.3.3 快速开始](/getting-started) 进入。

v1.0.0 是合并仓库后的首个稳定 1.0，包含安全清理与身份语义收紧，不能把“换回旧二进制”当作完整回滚。默认策略是 **停止写入并 forward-fix**；只有满足恢复条件时才回到升级前完整快照。未发布 v0.8.0 候选版的恢复演练或制品记录不能替代精确 v1.0.0 发布提交和制品的恢复证据。

## 首要原则

1. 立即从负载均衡移除异常实例并停止所有新旧 writer、定时任务和外部自动化；
2. 在任何修复前保存当前数据库、配置、日志、migration version、镜像 digest 和脱敏错误证据；
3. 不删除、不改写 migration version 行来强迫旧代码启动；
4. 不通过回填 PAT 明文、恢复已泄露 OAuth credential、重新开放 GET mutation 或放宽 root/默认角色来“恢复兼容”；
5. 数据库已提交但缓存清理失败时，以数据库为准，修复/清理缓存；不要为派生缓存失败回滚数据库；
6. 先判断失败属于制品、配置、代码还是数据迁移，再选择下面路径。

## 决策矩阵

| 状态 | 推荐动作 | 数据处理 |
| --- | --- | --- |
| migration 尚未开始 | 撤回候选制品，恢复旧流量 | 无数据库回滚；确认无写入 |
| migration 事务失败且 version 未记录 | 修复预检数据或代码后原样重跑 | 保留失败证据；不得手写 version |
| migration 成功、应用未开放流量 | 优先 forward-fix；必要时整库恢复 | 旧二进制仅在兼容性演练已证明时使用 |
| v1.0.0 已产生业务写入 | forward-fix | 直接恢复旧快照会丢失新写入，需明确停机/RPO 决策 |
| 仅前端失败 | 恢复上一静态制品并清 HTML/CDN 缓存 | 保留 v1.0.0 后端与数据库，不回滚数据 |
| 仅 Redis/通知失败 | 修复 Redis、清理派生缓存、重放通知（若支持） | 数据库提交保持有效 |
| 必须恢复已移除运行时工具 | 完整恢复 v0.7 代码、数据库和配置快照 | 不能靠新库中的残留表恢复菜单/策略 |
| 凭据泄露或权限扩大 | 继续隔离，rotate/revoke，forward-fix | 不允许用旧不安全状态恢复服务 |

## Forward-fix 流程

1. 冻结流量并建立 incident 记录；
2. 确认最后成功 migration 和失败组件；
3. 在恢复出的生产快照副本上复现；
4. 用新的、窄范围、幂等、前向 migration 或兼容代码修复，禁止编辑已发布 migration；
5. 重跑对应 SQLite/MySQL/PostgreSQL fresh、upgrade、repeat 和 failure rollback 测试；
6. 重跑权限负例、PAT/OAuth、ConfigRevision 和浏览器关键路径；
7. 先单实例、再小流量、最后全量恢复；
8. 将修复版本、数据影响和后续清理写入 Release notes。

以下问题优先 forward-fix：新增 revision/permission 行、旧代码可忽略的附加表、Redis 派生缓存、前端显示/静态交付、monitor 进程内历史。

## 代码回退边界

只回退二进制/前端而保留 v1.0.0 数据库，必须先在隔离恢复环境证明前一版本能够：

- 忽略新增 session、permission 和 ConfigRevision 行；
- 不把新默认角色重新提升为 root/default；
- 不写入绕过 revision 的主题/授权数据；
- 不读取或暴露已省略的 AppConfig credential；
- 不尝试恢复已移除的 runtime tool route；
- 不依赖已被清空的 PAT 明文或旧 OAuth password 语义。

若任一项不成立，旧二进制不得连接已升级生产库。

前端可独立回退的前提是后端仍保留旧投影兼容窗口。回退后清理 HTML、service worker、404 fallback 和 CDN 缓存，避免新旧 chunk 混用。

## 完整快照恢复

只有在维护窗口内、可以接受恢复点之后的新写入全部丢失，并且安全负责人批准恢复旧语义时，才执行整库恢复：

1. 停止全部 writer 并阻断外部流量；
2. 备份当前失败状态用于取证和后续数据比对；
3. 恢复同一时间点的数据库、配置、对象存储元数据和 v0.7 制品；
4. 验证 migration version 与恢复制品一致；
5. 保持历史 OAuth provider credential 已撤销，即使旧配置快照仍含该值也不得重新启用；
6. 轮换在事故中可能暴露的 auth key、PAT、OAuth secret 和数据库凭据；
7. 在隔离入口验证 root、非 root、登录、只读业务和审计；
8. 经风险确认后逐步恢复流量，并安排重新执行 v1.0.0 升级。

不要把 v1.0.0 产生的新业务行手工复制回旧库，除非存在单独评审的数据迁移计划。

## 不可逆或有损项

- PAT migration 清空兼容明文列；没有升级前备份时不能恢复原 token，只能重新签发；
- PAT 轮换或撤销成功后旧 bearer 必须继续失效；
- OAuth 历史账户的旧密码来源不可信，恢复本地登录只能走显式密码重置；
- 被 provider rotate/revoke 的 OAuth credential 不得恢复；
- 用户以 `null`/DELETE 主动删除的主题覆盖不是代码回滚对象；只有用户请求或明确数据恢复才可从备份恢复；
- monitor history 在进程内，重启后丢失，不进行数据库恢复；
- 已移除工具的历史业务表保留，但菜单/API/policy 已清理；恢复产品能力需要完整旧版本快照和额外安全审批。

## 恢复后验证

- 数据库、migration version、制品版本和配置来自同一恢复点；
- root 语义和非 root 拒绝路径符合恢复版本合同；
- 没有恢复已撤销 PAT/OAuth credential；
- 登录、OAuth（若启用）、关键读取、审计和备份任务正常；
- 前端 HTML 与 chunk 版本一致，无旧缓存交叉；
- 记录 RTO、RPO、丢失写入、人工数据处理和下一次 forward-fix 计划。

恢复成功不等于 v1.0.0 已发布。只有重新完成 [发布合同](/releases/archive/v1-0-0) 全部门禁后才能再次进入 stable 发布流程。
