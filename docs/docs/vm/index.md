---
nav:
  order: 4
  title: '虚拟模型'
description: 虚拟模型文档
keywords: [virtual model]
---

# 虚拟模型能力说明

虚拟模型是 `mss-boot-admin` 的历史存量能力，目前已经降级为 legacy /
compatibility 路径，不再作为新项目和新贡献的推荐方向。

当前推荐路径：

- 使用 Go model 描述业务实体；
- 使用 migration 管理数据库结构；
- 使用明确的 RBAC、API、菜单和配置治理流程；
- 在 GitHub Issues / Discussions 中先讨论 schema、权限和迁移影响。

如果你正在维护存量虚拟模型功能，请优先阅读：

- [Legacy capability deprecation](../admin/legacy-capability-deprecation.md)
- [Current capabilities](../admin/current-capabilities.md)

新的虚拟模型扩展需求通常不会直接进入主线实现。请先提供具体生产场景、
schema、权限约束和迁移需求，再作为 legacy compatibility 议题评估。
