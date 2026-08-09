---
title: 发布与升级
order: 1
nav:
  title: 发布
  order: 3
description: mss-boot-admin 版本状态、升级、兼容性与回滚合同
keywords: [release upgrade rollback compatibility mss-boot-admin]
---

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

## 下一版本梯队

`v1.0.1` 先收口验证码、Kafka、上传和对象存储 changed-path provider 生命周期的安全/数据完整性风险；
`v1.0.2-v1.0.3` 建立版本身份、严格配置与升级证据；`v1.1.0` prerelease 再交付
Storage Runtime v2 和一条完整的 Generator 黄金业务竖切。

- [v1.0.1 至 v1.1.0 完整路线](/releases/v1-0-1-to-v1-1-0-roadmap)
- [v1.0.1 Challenge 安全切片](/releases/v1-0-1-challenge-safety)
