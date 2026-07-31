---
title: 首次贡献指南
nav:
  order: 2
  title: "coding"
description: 面向新贡献者的 30 分钟首个 PR 指南
keywords: [first contribution, pull request, docs, tests]
---

# 首次贡献指南

本文档面向第一次为 `mss-boot-io` 组织贡献代码或文档的协作者，目标是在大约 30 分钟内完成一个小而有价值的 PR。

## 先选对仓库

根据你要修改的内容选择目标仓库：

| 仓库 | 适合改什么 | 常用验证命令 |
| --- | --- | --- |
| [`mss-boot-io/mss-boot-docs`](https://github.com/mss-boot-io/mss-boot-docs) | 文档、教程、FAQ、流程说明 | `pnpm install --frozen-lockfile && pnpm build` |
| [`mss-boot-io/mss-boot`](https://github.com/mss-boot-io/mss-boot) | 框架能力、HTTP/gRPC 服务模块、配置与中间件 | `go test ./...` |
| [`mss-boot-io/mss-boot-admin`](https://github.com/mss-boot-io/mss-boot-admin) | 后端业务能力、接口、治理能力 | `go test ./... -v -coverprofile=coverage.out` |
| [`mss-boot-io/mss-boot-admin-antd`](https://github.com/mss-boot-io/mss-boot-admin-antd) | 前端页面、组件、交互和文档截图 | `pnpm test --coverage` |

如果只是补充说明、修正文案、增加 FAQ 或补充新手流程，优先从 `mss-boot-docs` 开始。

## Fork 与克隆

1. 在 GitHub 上 fork 目标仓库到自己的账号。
2. 克隆自己的 fork：

```bash
git clone https://github.com/<your-account>/mss-boot-docs.git
cd mss-boot-docs
```

3. 可选：添加上游远程，方便后续同步：

```bash
git remote add upstream https://github.com/mss-boot-io/mss-boot-docs.git
git fetch upstream
```

## 本地安装与验证

### 文档仓库

```bash
pnpm install --frozen-lockfile
pnpm build
```

需要本地预览时可以运行：

```bash
pnpm start
```

### Go 仓库

```bash
go test ./...
```

如果你修改的是 `mss-boot-admin`，在需要覆盖率输出时运行：

```bash
go test ./... -v -coverprofile=coverage.out
```

### 前端仓库

```bash
pnpm install --frozen-lockfile
pnpm test --coverage
```

## 选择一个小范围改动

适合首个 PR 的范围：

- 修复 README、教程或 FAQ 中的错误命令
- 补一段缺失的本地启动或测试步骤
- 为已有功能增加文档示例
- 为已存在行为补测试，不顺便重构无关代码

避免首次贡献就同时修改多个仓库、部署配置、组织设置或需要私有环境信息的内容。

## 分支命名建议

仓库目前没有强制的公开分支命名规则，但建议使用“改动类型/简短主题”的形式，方便 maintainer 识别：

- `docs/first-contribution-guide`
- `docs/fix-broken-command`
- `test/add-login-handler-coverage`
- `fix/http-config-example`

创建分支：

```bash
git checkout -b docs/first-contribution-guide
```

## 提交与 PR 标题

提交信息和 PR 标题建议使用 Conventional Commits 风格：

- `docs: add first contribution guide`
- `docs(admin): clarify local debug steps`
- `test(auth): cover expired token handling`

提交前先确认你改动对应的验证命令已经跑过。

## 提交前检查清单

发起 PR 前至少确认以下几项：

- 改动范围只覆盖一个明确问题
- 命令、链接、日期、仓库名都已手动核对
- 没有加入密钥、私有 URL、截图中的敏感信息
- 本地执行了对应仓库的基础验证命令
- PR 描述写清楚问题、改动和验证方式

对于 `mss-boot-docs`，可以直接参考仓库里的 [`CONTRIBUTING.md`](https://github.com/mss-boot-io/mss-boot-docs/blob/main/CONTRIBUTING.md) 和 PR 模板。

## 在哪里提问

如果你卡住了，优先使用对维护者最容易跟进的公开渠道：

1. 在你准备解决的 issue 下补充问题或确认范围
2. 如果没有对应 issue，就在目标仓库新开一个问题说明背景
3. 如果已经有 draft PR，可以直接在 PR 描述里列出待确认点

当问题与公开仓库内容直接相关时，优先使用 GitHub issue/PR 线程，避免把决策散落到不可追踪的聊天工具里。

## 一个推荐流程

```bash
git checkout -b docs/first-contribution-guide
pnpm install --frozen-lockfile
# 编辑 docs/**
pnpm build
git status
git add docs/coding/first-contribution.md docs/coding/index.md
git commit -m "docs: add first contribution guide"
git push origin docs/first-contribution-guide
```

然后在 GitHub 上发起 PR，并在描述中写清楚：

- 解决了哪个 issue
- 改动了哪些页面或文件
- 本地跑了什么验证命令

## 下一步

完成首个小 PR 后，再逐步尝试：

- 给已有教程补测试或截图
- 为 `mss-boot-admin` 补局部测试
- 为 `mss-boot-admin-antd` 修复文档与界面说明不一致的问题

先建立“选对仓库、改动够小、验证完整”的节奏，比一次做大更重要。
