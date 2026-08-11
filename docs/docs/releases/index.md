---
title: 发布与升级
order: 1
nav:
  title: 发布
  order: 3
description: mss-boot-admin 版本状态、升级、兼容性与回滚合同
keywords: [release upgrade rollback compatibility mss-boot-admin]
---

> 最新开发 checkpoint：[`86c0e8a` D3 Named Redis Resource](/releases/v1-1-0-d3-named-redis-resource)
> 已在 `c57ffc8` 脱敏资源 error-tree 上固化 one-client、isolated Scope、caller deadline、
> tracked exactly-once close 与 22 项 exact race evidence。Storage Runtime aggregate 仍为
> Planned；Sentinel control ACL、cluster 原子 multi-key、真实 Sentinel/cluster/TLS、Admin
> composition 和 leak evidence 仍未完成。

# 发布与升级

这里保存长期有效的版本合同。Git tag、GitHub Release、嵌套 Go 模块的外部解析结果和对应提交上的验证报告共同构成发布证据；分支名、`Unreleased`、`planned`、`preview` 或本地 `go.work` 替换都不代表稳定版本。

## v1.0.0 stable

当前状态：**已于 2026-08-09 发布**。

这是合并仓库后的首个稳定 1.0 版本边界。根 `v1.0.0` 与先行发布的
`mss-boot/v1.0.0` 均指向
`ee800262c035c5f4242aca1841d077554481d2c4`。公开 Release 直接证明发布事实；验收工单
[#471](https://github.com/mss-boot-io/mss-boot-admin/issues/471) 记录精确提交、workflow、制品与发布后证据。独立
`web/antd/v1.0.0` tag 尚未发布。

- [发布合同](/releases/v1-0-0)
- [从 v0.7.x 升级](/releases/v1-0-0-upgrade)
- [兼容性矩阵](/releases/v1-0-0-compatibility)
- [回滚与恢复](/releases/v1-0-0-rollback)

本次稳定发布遵循了以下顺序，后续版本继续复用：

1. 在已评审的同一发布提交上完成全部验证；
2. 发布 `mss-boot/v1.0.0`；
3. 在仓库外、关闭 workspace 替换后验证嵌套模块可解析；
4. 再发布根标签 `v1.0.0`；
5. 对已发布制品执行禁用缓存的安装与运行冒烟。

首次 publication 前失败时保持 preview。Framework 已公开后若外部解析或 pre-root gate 失败，
该组件记录为 `component-partial / evidence-incomplete`，根标签不发布，并从下一更高同步补丁
forward-repair；根版本公开后的 reconciliation 失败则记录为
`published / evidence-incomplete`。两种情况都不得移动或删除标签，并停止后续发布直到 evidence
issue 完成终态记录或链接已验证的替代列车。

## 下一公开版本：v1.1.0

当前采用“开发优先、冻结后集中验证”策略：原 `v1.0.1-v1.0.3` 和 alpha 切片只作为
内部开发波次，不创建 tag、GitHub Release 或版本化包。Challenge、Kafka Mark/lifecycle、
Upload admission 与 D1 object provider/owner 已形成安全 checkpoint，将继续作为永久
回归哨兵。`D1-provider-owner` 已整体完成；Kafka 保持 `AdapterQueue` 兼容面并通过新增
`ManagedAdapterQueue` 由 Admin 统一拥有、作为 `Runnable` 运行和有界关闭，但仍保持
Legacy/Blocked。`D2-contract-substrate` 正在推进：canonical email 的迁移、模型、Admin 写边界和
server schema-readiness 已形成开发 checkpoint；冻结 SHA 上仍须重跑 readiness 正负套件以及
MySQL/PostgreSQL 双 DSN zero-skip evidence，因此 capability 保持 Planned。`151a91c` 还完成了
downstream snapshot identity consumer checkpoint：CLI/MCP/doctor 共用严格 SnapshotStatus，区分
精确 source sentinel、有效 generated pair 与 malformed/orphaned state，并阻止 nested Admin module
冒充 root module。该 checkpoint 只有 fully anchored 本地测试和 workflow 静态合同；真实 GitHub Actions
尚未运行。冻结 SHA 仍须真实执行 `foundation-compatibility.yml`，证明四身份/digests、Blueprint
0.1→0.2 定制保留和第二次空升级；pre-root 仍须 release-built external artifact。

`D3-backend-runtime` 已形成两个 Framework checkpoint：资源图在 `d90b4c7` + `c830b5f`
建立确定性生命周期，`c57ffc8` 阻止 provider 对象从公共 error tree 泄漏；`86c0e8a`
新增 additive named Redis Resource。22 项完全锚定的单包 race×20 evidence 证明构造、
Scope 隔离、caller deadline、lease drain 与 one-close，并使用 standalone miniredis 和
stalled socket；Sentinel/cluster/TLS 仍只有 construction matrix，不能据此晋级 capability。

只有 Generator/Blueprint 与 Storage Runtime 目标全部完成、选定一个 `v1.1.0` 功能冻结 SHA
后，才手工启动 `release-readiness`，集中执行三数据库、browser、Provider、upgrade、
external consumer、recovery、`verify --all` 与 `eval --all`。全部通过后才进入 Framework →
外部解析 → root → post-publication reconciliation。

开发期 `.mss/release-policy.yaml` 保持 `publicationWorkflowsReady: false`；因此 bootstrap readiness 绿色只表示
开发候选结果，不授予 tag、Release 或 package 发布权。受保护 `release` environment、tag ruleset 和完整阶段证据
执行器完成后，先评审开启 workflow 能力，再选择该提交为冻结 SHA 并执行正式 qualification；pre-framework authority
仍需单独批准实际发布。

- [v1.1.0 开发优先完整路线](/releases/v1-0-1-to-v1-1-0-roadmap)
- [Challenge 内部安全 checkpoint](/releases/v1-0-1-challenge-safety)
- [Kafka Mark-after-success 内部 checkpoint](/releases/v1-0-1-kafka-ack-safety)
- [Upload admission 内部 checkpoint](/releases/v1-0-1-upload-admission-safety)
- [D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner)
- [D2 Canonical Email Identity 内部 checkpoint](/releases/v1-1-0-d2-canonical-email-identity)
- [D2 Downstream Snapshot Identity 内部 checkpoint](/releases/v1-1-0-d2-snapshot-identity)
- [D3 Resource Lifecycle 内部 checkpoint](/releases/v1-1-0-d3-resource-lifecycle)
- [D3 Named Redis Resource 内部 checkpoint](/releases/v1-1-0-d3-named-redis-resource)
