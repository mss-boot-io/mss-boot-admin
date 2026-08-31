---
title: 使用指南
order: 2
nav:
  title: 使用指南
  order: 2
description: Thin Host 日常开发、配置、验证、部署与故障排查入口
---

# 使用指南

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布；v1.3.7 已选
为 release candidate，但尚未稳定且不可采用。候选 Distribution 发布面可能处于不同公开阶段，
必须以远端发布台账为准；完整 stable promotion 和最终 current-stable policy 对账完成前，不得
安装、创建或升级，版本判断从[采用状态](/getting-started)进入。Docs 网站可异步候补且不阻断
该采用门禁。
:::

## 合同入口

| 任务 | 入口 |
| --- | --- |
| 查看源码能力与发布边界 | [源码能力](/admin/current-capabilities) |
| 理解候选工具与 Blueprint | [Blueprint 与升级状态](/agent/blueprints-and-upgrades) |
| 调试 Foundation 源码 | [本地调试合同](/admin/local-debug) |
| 配置数据库、会话、CORS 与资源 | [配置指南](/admin/configuration-guide) |
| 运行变更或完整验证 | [集成验证](/admin/integration-test-guide) |
| 构建容器和记录摘要 | [容器部署](/admin/docker) |
| 检查安全上线条件 | [安全基线](/admin/security-baseline) |
| 处理常见失败 | [FAQ](/guide/faq) |

Foundation 源码中的业务模块先修改 `.mss/modules/<name>.yaml`，再按仓库
`.mss/commands.yaml` 生成并检查确定性变更。不要手改生成区，也不要把源码生成结果
称为 v1.3.5 或 v1.3.6 下游应用。
