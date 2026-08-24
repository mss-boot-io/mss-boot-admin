---
title: Blueprint 与升级
order: 4
description: v1.3.3 内置 Blueprint、Thin Host 所有权和三方升级合同
---

# Blueprint 与升级

v1.3.3 的 `mss` 二进制内置与版本、完整提交、提交时间和源码仓库绑定的
`management-system` Blueprint。采用者不需要 Foundation checkout。

## 创建

```sh
mss new app customer-admin --module github.com/acme/customer-admin --destination ./customer-admin
mss new app customer-admin --module github.com/acme/customer-admin --destination ./customer-admin --write --git-init
```

第一条只读计划，第二条在无冲突后原子写入并初始化 Git。目标必须在允许目录内，未知
文件导致失败，两次生成输出稳定。

## 文件所有权

| 类型 | 示例 | 升级行为 |
| --- | --- | --- |
| Blueprint 管理 | 组合入口、项目合同、公共配置胶水 | 通过三方合并升级 |
| 生成管理 | 菜单、路由、API 与 locale 投影 | 从规格重新生成 |
| 业务所有 | `internal/modules/<name>`、业务页面与测试 | 自动升级必须保留 |
| 未知 | 用户新增且未登记的文件 | 自动升级必须保留 |

`.mss/blueprint-manifest.json` 保存受管文件摘要，`.mss/lock.yaml` 保存当前发行版与
升级记录。不要手改摘要伪造一致。

## 升级计划

先备份仓库、配置和数据库，并安装目标 Distribution 的工具。确认二进制身份和 Blueprint
基线都存在：

```sh
mss --version
mss-mcp --version
mss upgrade status --format json
```

三方合并需要原始 Blueprint 摘要。手工拼装的仓库或丢失 manifest 的仓库必须先在新
目录生成目标版本 Thin Host，再按所有权迁入业务规格与业务文件；不得手写摘要把它伪装
成可升级基线。

```sh
mss upgrade status --format json
mss upgrade admin v1.3.3
```

计划比较旧基线、当前工作树和安装工具内置的新基线。它不写文件，并明确列出新增、更新、
保留和冲突。

## 应用

在备份、评审和冲突清零后：

```sh
mss upgrade admin v1.3.3 --apply --yes
mss upgrade status --format json
mss doctor --strict
mss verify --all
mss upgrade admin v1.3.3
```

写入使用事务式暂存，业务与未知文件保留，快照最后更新。第二次应用必须为空。请求版本
与安装工具内置发行版不一致时，先安装匹配版本，不要用本地源码替代公共制品。

## 失败与恢复

计划失败不会写入。应用失败应保留原工作树和可诊断报告；修复输入后重新计划。已经执行
数据库迁移的部署回滚必须同时考虑数据快照，不能只恢复文件。
