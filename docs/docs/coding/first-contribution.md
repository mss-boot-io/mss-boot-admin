---
title: 首次贡献指南
nav:
  order: 2
  title: coding
description: 在 mss-boot-admin 单仓库中完成一个小而可验证的首次 PR
keywords: [first contribution pull request docs tests]
---

# 首次贡献指南

后端、Framework、前端、生成器、机器契约和文档现在都位于
[`mss-boot-io/mss-boot-admin`](https://github.com/mss-boot-io/mss-boot-admin)。
文档不是独立仓库；根据改动目录选择最小充分验证即可。

## 1. Fork 和克隆

~~~bash
git clone https://github.com/<your-account>/mss-boot-admin.git
cd mss-boot-admin
git remote add upstream https://github.com/mss-boot-io/mss-boot-admin.git
git fetch upstream
~~~

从最新 <code>main</code> 创建一个小范围分支：

~~~bash
git switch main
git pull --ff-only upstream main
git switch -c docs/fix-one-guide
~~~

## 2. 先读范围约束

1. 阅读根目录 <code>AGENTS.md</code>。
2. 修改文档时再阅读 <code>docs/AGENTS.md</code>。
3. 检查 <code>.mss/project.yaml</code>、<code>.mss/capabilities.yaml</code> 和
   <code>.mss/commands.yaml</code>，不要用旧文档猜测当前命令。
4. 保留工作区中不属于本任务的改动。

## 3. 选择小范围

适合首次 PR：

- 修正一个错误命令、路径或链接；
- 补充一个缺失的状态、失败路径或排障步骤；
- 为已有行为增加一个聚焦测试；
- 修复一个有明确复现的小 UI 问题。

不要在首个 PR 中混合架构重写、数据库破坏性迁移、发布、生产配置和无关格式化。

## 4. 验证

| 改动 | 最小入口 |
| --- | --- |
| 文档 | <code>corepack pnpm@9.15.9 --dir docs build</code> |
| Framework | <code>cd mss-boot && GOWORK=off go test ./...</code> |
| Admin Go | 聚焦 <code>go test</code>，再运行受影响模块测试 |
| Admin Web | lint、TypeScript、聚焦测试和相关 build |
| 跨组件 | <code>go run ./cmd/mss verify --changed</code> |

只报告实际运行的命令。文档发布准备还需要在 Codex 内置浏览器检查桌面、窄屏、导航、深色
主题和控制台；构建通过不等于视觉验收通过。

## 5. 提交和 PR

使用 Conventional Commits，例如：

- <code>docs(release): clarify database rollback</code>
- <code>test(auth): cover denied session revoke</code>
- <code>fix(web): preserve menu selection on refresh</code>

PR 描述应写清：

- 问题与范围；
- 关键文件；
- 实际验证结果；
- 迁移、安全、兼容和发布影响；
- 跳过项及原因。

所有发布意图的改动都必须先合并到 <code>main</code>。不要从贡献分支创建 tag、package、镜像
或 GitHub Release。

## 6. 获取帮助

一般问题使用
[`mss-boot-admin` Issues](https://github.com/mss-boot-io/mss-boot-admin/issues)。疑似漏洞遵循
[Security Policy FAQ](/devops/security-policy-faq)，不要在公开 Issue 中粘贴漏洞细节或凭据。

更多规则见
[`docs/CONTRIBUTING.md`](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/CONTRIBUTING.md)。
