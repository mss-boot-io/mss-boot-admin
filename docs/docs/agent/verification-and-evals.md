---
title: 验证与评测
order: 6
description: change-aware verify、确定性 eval 和证据交付
---

# 验证与评测

## 先看计划

```sh
mss verify --changed --plan --format json
```

计划说明检测到的变更、选择的检查和报告路径，不执行外部命令。

## 执行检查

```sh
mss verify --changed
mss verify --module orders
mss verify --all
```

- `--changed` 是默认日常入口；
- `--module` 聚焦一个垂直模块；
- `--all` 用于升级、合并或发布资格。

机器报告位于 `.mss/reports/verify.json`，Markdown 摘要用于审查。退出码和单项结果都
应保留。

## Evals

```sh
mss eval list
mss eval run --all
mss eval report
```

Eval 衡量 Agent 是否遵守规格、边界、生成幂等、迁移、权限和验证合同；它不替代产品
测试或浏览器验收。

## 按影响增加证据

| 变化 | 最低附加证据 |
| --- | --- |
| Go | focused test，再运行受影响模块 |
| Framework | 独立 `GOWORK=off` test/vet |
| 前端 | lint、TypeScript、focused test、相关 build |
| 迁移 | 空库和上一版本升级 |
| 权限 | 后端允许与拒绝 |
| 生成器 | schema、golden、路径限制、两次幂等 |
| UI | Codex 内置浏览器桌面、窄屏、深链、控制台 |

报告未执行检查及具体原因；不要把计划、服务健康或单一页面截图写成完整通过。
