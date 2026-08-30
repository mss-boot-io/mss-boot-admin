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
- `--all` 是显式完整审计入口；发布资格只在冻结后的精确 merged-main commit 上使用
  机器合同声明的 release-evidence 模式执行一次。

发布负责人使用的规范入口是：

```sh
mss verify --all --release-evidence --expect-commit <full-sha>
```

该命令名称是长期公开入口；具体检查矩阵仍由 `.mss/commands.yaml`、当前 Feature 与
`mss-release` Skill 管理，本站不复制那份会随实现演进的机器合同。

机器报告位于 `.mss/reports/verify.json`，Markdown 摘要用于审查。退出码和单项结果都
应保留。具体检查选择、工作目录和参数以本地 `.mss/commands.yaml` 与 `mss verify --plan`
为准，公开页面不复制完整发布矩阵。

## 固定完整验证证据

发布台账必须记录完整 commit、报告摘要或哈希和退出结果，并证明验证前后 tracked-clean。
报告单独存在不构成发布授权；它也不替代合并到 `main`、Tag actor、不可变引用、OIDC、
provenance 和公共制品对账。完整执行顺序由 `mss-release` Skill 与 v1.3.7 Feature 约束，
避免本站成为另一份会漂移的发布手册。

## 手动扩展资格验证

本地证据通过后，如变更涉及 Agent 安装器、跨平台进程管理、前端包交付或浏览器运行
环境，可按 `.mss/commands.yaml` 手动运行对应扩展工作流。记录其完整 Head SHA；不得用
重跑成功掩盖确定性缺陷，也不得以托管工作流替代本地验证或浏览器验收。

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
