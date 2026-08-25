---
title: Agent 架构摘要
order: 7
description: 人类意图、机器合同、确定性工具和运行时产品的边界
---

# Agent 架构摘要

```text
人类 / Agent
    │ 需求与审查
    ▼
AGENTS.md + .mss 规格与能力
    │ 确定性计划
    ▼
mss / mss-mcp
    │ 生成、验证、升级
    ▼
Thin Host 业务代码 ──编译期──> Admin + Admin Web ──> mss-boot
```

关键分离：

- Agent 工具属于开发时，不进入 Admin 运行时；
- Admin 是完整产品，Thin Host 只拥有业务和组合；
- Framework 领域无关，不依赖 Admin；
- 规格由人和 Agent 编辑，生成区由工具拥有；
- 后端授权、迁移和业务完成不由 UI 或 Agent 推断；
- 发布工具与包绑定一个 merged-main 提交。

完整设计见[Agent-native Foundation](/architecture/agent-native-foundation)与
[Complete Admin Distribution](/architecture/complete-admin-distribution-and-thin-business-host)。
