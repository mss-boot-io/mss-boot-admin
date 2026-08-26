---
title: Complete Admin Distribution 与 Thin Host
order: 2
description: v1.3.5 完整 Admin 发行、编译期扩展、文件所有权与三方升级架构
---

# Complete Admin Distribution 与 Thin Host

## 决策

Admin 后端、Admin Web、Framework、Agent 工具和 Blueprint 使用同一个 v1.3.5
Distribution 版本协调，但保留独立发布制品。下游应用是 Thin Host，不是 Foundation
源码副本。

## 发行组成

| 组件 | v1.3.5 身份 | 所有权 |
| --- | --- | --- |
| Root tools | GitHub Release `v1.3.5` | new/setup/dev/verify/upgrade |
| Framework | `mss-boot/v1.3.5` | 领域无关运行时 |
| Admin | `admin/v1.3.5` | 完整后端与编译期扩展边界 |
| Admin Web | `web/antd-v6/v1.3.5`、npm `1.3.5` | 完整浏览器应用 |
| Blueprint | 嵌入 `mss v1.3.5` | Thin Host 受管文件 |
| Docs | `docs/v1.3.5` | 当前使用和发布合同 |

所有制品绑定一个完整 merged-main 提交，不能用分支头、本地替换或相邻版本拼装。
候选文档中的身份是发布目标；只有公共对账全部完成后，它们才成为可安装制品。

## Thin Host 内容

```text
cmd/server/main.go                 Admin 组合入口
internal/modules/<business>/       业务后端所有
web/src/business/                  业务前端所有
.mss/                             项目、模块、命令和锁合同
受管配置/构建胶水                 Blueprint 所有
```

Thin Host 不包含 Admin 核心路由、中间件、迁移、Framework、完整前端页面、Foundation
文档或发布工作流副本。

## 后端组合

`admin/app` 构造完整应用，`admin/business` 接收显式有序模块。模块注册在事务式
registry 中完成；迁移冲突、路由冲突、重入或部分注册失败必须 fail closed。所有业务
API 位于核心安全中间件后的受保护组中。

后端模块可以拥有：

- 前向迁移；
- 模型、DTO、服务和受保护 API；
- 权限、菜单投影与就绪检查；
- 正向和负向授权测试。

它不能替换认证、会话、CSRF、核心路由或迁移 registry。

## 前端组合

`@mss-boot-io/admin-web@1.3.5` 是唯一完整 SPA。Thin Host 使用公开 preset、runtime
和 business 导出注册业务路由与菜单。核心路由先注册，业务扩展随后注册，403/404
回退最后注册。

业务页面必须覆盖 loading、empty、retryable error、denied、responsive 和 locale。
浏览器 route guard 不替代后端授权。

## 内置 Blueprint

Release 构建把单一 Blueprint 源直接嵌入 `mss`，并注入版本、提交、时间戳和仓库。
`mss new app` 在空目录也能：

- 先产生只读计划；
- 检查目标路径和未知文件；
- 原子写入；
- 可选初始化 Git；
- 固定 v1.3.5 Go/npm 依赖与冻结锁；
- 写入 manifest、lock 和同源快照；
- 第二次生成无差异。

## 三方升级

```text
旧 Blueprint 基线 + 当前 Thin Host + v1.3.5 新基线
                         │
                         ▼
                 只读计划 / 冲突列表
                         │ 人工确认
                         ▼
                  --apply --yes
```

`mss upgrade admin v1.3.5` 默认只读。匹配版本从工具内置基线获取，不需要额外源码。
三方比较要求仓库保留生成时的 `.mss/blueprint-manifest.json`；手工拼装或基线丢失的
仓库必须迁入新生成的 Thin Host，不能伪造摘要。应用保留业务和未知文件，写入受管文件
后最后更新快照；完成后第二次计划为空。

## 发布与公共验证

1. 通过 PR 合并全部代码、合同、文档和工作流；
2. 冻结精确干净的 `origin/main` 提交；
3. 发布 Framework 并执行外部 Go 解析；
4. 发布 Admin 并执行外部组合测试；
5. 发布 Admin Web 候选、镜像与 GitHub Release；
6. 发布 Root 工具、安装脚本、摘要和来源；
7. 发布 Docs；
8. 最后发布官方 npmjs 包；
9. 从空目录安装公开工具并验证完整 Thin Host 生命周期。

任何修复都使用后续 PR 和补丁版本，不移动已公开 ref。

## mss-shop 证明

[mss-shop](/getting-started/mss-shop) 必须在 v1.3.5 公开后由 Release 工具生成，先提交
未修改基线，再加入通用单租户商城模块。它验证包导入、业务所有权、升级、自动测试和
Codex 内置浏览器流程，而不是复制 R1Shop 或 Foundation。
