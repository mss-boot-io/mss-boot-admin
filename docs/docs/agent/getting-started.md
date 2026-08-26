---
title: 在 Thin Host 中开始
order: 2
description: Agent 进入已生成 v1.3.5 Thin Host 后的检查、规划和交付顺序
---

# 在 Thin Host 中开始

本页从“仓库已经按[唯一快速开始](/getting-started)生成”继续，不重复工具安装和建项目。

## 1. 建立可验证上下文

```sh
mss context --format json
mss doctor --strict
git status --short --branch
```

读取根和目标目录最近的 `AGENTS.md`，再读取 `.mss/project.yaml`、
`.mss/capabilities.yaml` 与 `.mss/commands.yaml`。保留不相关的现有改动。

## 2. 先查能力，再写规格

```sh
mss spec init order-approval --kind feature --module orders --output .mss/features/order-approval.yaml --write
mss spec validate .mss/features/order-approval.yaml
```

中大型变化先定义目标、非目标、模块、需求、约束、验收、风险和回滚。已有能力优先扩展，
不要创建平行实现。

## 3. dry-run 与受控写入

```sh
mss module generate .mss/modules/orders.yaml --format json
mss module generate .mss/modules/orders.yaml --write
mss module generate .mss/modules/orders.yaml --check
```

检查变更列表和所有权。生成区从规格修改；业务所有文件由业务模块维护；未知文件不覆盖。

## 4. 最小验证后扩大

```sh
mss verify --changed --plan --format json
mss verify --changed
```

迁移增加空库和升级路径，权限增加正反例，前端增加 loading/empty/error/denied 与 locale，
高风险交互增加内置浏览器验收。只有需要时运行 `mss verify --all`。

## 5. 可审查交付

交付说明包括目标、重要文件、实际命令与结果、迁移、兼容性、安全影响、未执行检查和
下一可执行步骤。不得声称未运行的验证成功。
