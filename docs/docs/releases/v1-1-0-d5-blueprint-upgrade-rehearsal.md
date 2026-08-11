---
title: D5 Blueprint 0.1 to 0.2 upgrade rehearsal
order: 20
description: v1.1.0 D5 external three-way upgrade, customization preservation, idempotency, identity, and failure-atomicity evidence
keywords: [v1.1.0 D5 blueprint upgrade rehearsal customization atomicity identity]
---

# D5 Blueprint 0.1 to 0.2 upgrade rehearsal

本文记录一次在源码仓库之外完成的 `v1.1.0` 开发期演练。源基线为 Blueprint
`0.1.0`、Foundation commit
`35e41d3e38c2aaab8131db70db54bce3a108625b`；目标为 Blueprint `0.2.0`、
Foundation commit `160e2df196dc804f2fb1717f28404553a9538036`。

这是一条未打 tag 的 D5 checkpoint 证据，不是 feature-freeze SHA、GitHub Actions
`foundation-compatibility.yml` run、发布授权或 pre-root release-built artifact 证据。它只记录本次
新增的外部升级演练，不重新评价此前稳定行为。

## 成功升级与第二次空升级

演练先从 `0.1.0` 基线生成独立 downstream，再加入 downstream 自有业务文件
`admin/modules/rehearsalcustom/service.go`。首次 `upgrade plan` 与 `upgrade apply` 都无冲突，apply
退出码为 0：

| Action | 首次升级数量 |
| --- | ---: |
| `update` | 822 |
| `create` | 412 |
| `delete` | 60 |
| `unchanged` | 66 |
| `conflict` | 0 |

首次升级后，目标 Blueprint 管理 1,300 个现存文件；针对同一目标执行第二次 apply 时，结果为
`1,300 unchanged`，`nonUnchanged=0`。也就是说第二次执行没有 create、update、delete 或
conflict，不需要修补或手工清理才能达到幂等状态。

downstream 自有文件在升级前后的 SHA-256 完全相同：

```text
3eca5b0f15d1b4ee5d2fc6c63cb08aa242d4c4ae7270fc50503d8994e2c9fc6f
```

该文件不在 Foundation 管理清单内，升级未覆盖、移动或重新生成它。升级后的独立 downstream
还在 `admin/` 下以 `GOWORK=off go test ./modules/rehearsalcustom` 完成编译检查；输出为
`no test files`，命令退出成功。这个结果只证明自有模块继续编译，不把零测试误写成测试覆盖率证据。

## Snapshot identity 与本地健康状态

成功 apply 后，lock、manifest 与 `upgrade status` 对同一安装快照给出一致身份：

- Foundation commit：`160e2df196dc804f2fb1717f28404553a9538036`；
- Blueprint：`management-system` `0.2.0`；
- manifest 记录的 lock SHA-256：
  `1393bf8d1e8113fb1a77cc10e773987dbdf863b9eb7e51440c41c93b40141ad3`，与实际 lock 内容一致；
- strict agent doctor：`ready=true`，6 pass、0 non-pass；
- skills validate：`valid=true`，10 个 skill 全部通过。

这些结果证明本次开发候选写入了自洽的 lock/manifest pair，并能被当前 CLI 健康检查消费。它们不
替代冻结 SHA 上 CLI、MCP、doctor 和 workflow artifact 的完整一致性证据。

## 失败注入与原子恢复

失败路径使用另一份独立 `0.1.0` downstream。演练将一个目标阶段才会创建的文件
`web/antd/src/wrappers/emailChallenge.tsx` 设为只读，使 apply 在 late-create 阶段失败。结果如下：

- apply 退出码为 1；
- 失败前后 lock SHA-256 都是
  `5d8442eda61d962dc1551db37fe4d2c32c725a4c4aedf792c3f683dbc62e0b0e`；
- 失败前后 manifest SHA-256 都是
  `390718804b4e686f87a4823f67b00368eab3eee375643387e137dd3260feb072`；
- `gitChanges=0`，transaction journal 数量为 0；
- 注入目标文件不存在，没有留下部分创建结果；
- 清除注入条件后重新 plan 成功，并再次得到相同的无冲突 action 计数。

因此本次注入失败没有提前提交新身份、污染 downstream Git tree 或留下需要人工删除的事务日志。
恢复路径是移除外部失败条件并重新计划，不是手工改写 lock、manifest 或基线摘要。

## 发版前用户决定项

失败 apply 的 JSON 中仍出现 `success=true`。当前语义是：该字段只表示随 apply 返回的三方合并
**plan 无冲突**；它不表示文件执行成功。执行失败由进程退出码 1 表达，原子性证据也确认安装状态没有
改变。

这项语义不会推翻本次失败原子性结果，但只读取 JSON、忽略进程退出码的调用方可能误判。因此在
v1.1.0 发布前由用户明确选择：保留现有 `success` 的 plan 语义并强化调用约束，或新增
`executionSuccess`/等价执行状态字段。决定前保持该项可见，不在本 checkpoint 中擅自扩大 API
修改。

## 仍需在冻结后完成

选定 feature-freeze SHA 后，仍须从该完整 SHA 真实运行
`.github/workflows/foundation-compatibility.yml`，保存 run URL 和 artifact，并重做 0.1→0.2、定制
保留、身份一致与第二次空升级。Framework 发布后，pre-root 还须使用 release-built artifact、禁用
workspace replacement，并在源码仓库之外复核安装。当前开发演练不能复用为这两项发布证据。
