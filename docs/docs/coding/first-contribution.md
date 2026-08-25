---
title: Foundation 贡献者入门
order: 1
description: 明确隔离的 mss-boot-admin 源码贡献流程
---

# Foundation 贡献者入门

> 本页仅适用于修改 Foundation 本身的贡献者。创建业务应用请使用
> [v1.3.3 快速开始](/getting-started)，不要照搬本页的源码命令。

## 获取源码

```sh
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin
git switch main
```

阅读根和目标目录的 `AGENTS.md`，确认工作树和分支，再创建主题分支。不要覆盖无关
本地改动。

## 建立上下文

```sh
go run ./cmd/mss context
go run ./cmd/mss doctor
```

源码模式只用于贡献者。中大型变化先更新 `.mss/features/`，再实现最小一致变更。

## 验证

```sh
go run ./cmd/mss verify --changed
python3 tools/docs/check_current_docs.py
corepack pnpm@9.15.9 --dir docs build
```

按影响补充 Framework 独立测试、Admin 测试、前端 lint/test/build、迁移或浏览器证据。

## 提交

通过 PR 合入 `main`。不要直接推送 `main`、改写已推送历史或从未合并提交发布。交付
说明列出实际命令、结果、未执行项、兼容与安全影响。
