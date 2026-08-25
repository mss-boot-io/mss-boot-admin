---
title: 规格
order: 3
description: Feature 与 AdminModule 规格的职责、命令和审查要求
---

# 规格

规格是人和 Agent 都可审查的变更合同；生成文件不是需求来源。

## Feature

适用于跨模块、中大型或高风险变化，应包含：

- 目标与非目标；
- 参与者和模块；
- 可测试需求与约束；
- 验收层级、阶段和证据；
- 风险、迁移、发布和回滚。

```sh
mss spec init order-approval --kind feature --module orders --output .mss/features/order-approval.yaml --write
mss spec validate .mss/features/order-approval.yaml --format json
```

## AdminModule

垂直业务模块规格位于 `.mss/modules/<name>.yaml`，描述模型、字段、API、权限、菜单、
前端、迁移和测试。新增字段使用前向迁移，不用破坏性重建。

```sh
mss spec validate .mss/modules/orders.yaml
mss module generate .mss/modules/orders.yaml --format json
mss module generate .mss/modules/orders.yaml --write
mss module generate .mss/modules/orders.yaml --check
```

## 审查清单

- 是否已有能力可扩展；
- 状态变更是否使用非 GET；
- 后端是否执行授权并覆盖拒绝测试；
- 多写操作是否有事务、幂等和冲突语义；
- 模型变化是否有空库与升级路径；
- UI 是否覆盖 loading、empty、error、denied 与 locale；
- 生成是否路径受限、稳定且两次运行无差异；
- 文档、机器合同和实际代码是否一致。
