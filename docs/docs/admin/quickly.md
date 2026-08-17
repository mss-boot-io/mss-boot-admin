---
title: 快速开始
order: 11
nav:
  order: 1
  title: admin
description: 快速启动mss-boot-admin
keywords: [admin quickly start]
---

`mss-boot-admin` 是一个前后端分离的单一 monorepo。后端位于 `admin/`，默认前端位于
`web/antd-v6/`，这是唯一支持的 Admin 浏览器应用。版本和目录约定以仓库根目录的
`.mss/project.yaml` 为准。

## 环境要求

:::warning
Go 1.26.6

Node.js >= 24 且 < 25

corepack pnpm 10.34.5

port 8080（后端）, 8001（默认 V6 前端）
:::

## 1. 下载项目

```shell
# 下载单一仓库（已包含后端与前端）
git clone https://github.com/mss-boot-io/mss-boot-admin.git
cd mss-boot-admin
```

## 2. 初始化后端

后端默认可使用本地 SQLite（`config/application.yml` 中默认 `database.driver: sqlite`），可先无需额外数据库服务。

```shell
# 从仓库根目录进入后端
cd admin
# 迁移数据库
go run . migrate
```

> 默认 SQLite 无需额外服务即可启动。如需切换 MySQL/PostgreSQL，请按实际情况
> 设置 `DB_DSN`，并参考仓库根目录下的 `compose/`；不要把生产密码写入文档、
> 命令历史或提交到仓库。

## 3. 启动后端服务

```shell
# 启动后端服务
go run . server
```

## 4. 启动前端服务

```shell
# 从后端目录进入前端
cd ../web/antd-v6
# 安装依赖
corepack pnpm@10.34.5 install --frozen-lockfile
# 启动前端服务
corepack pnpm@10.34.5 start:dev
```

## 5. 验证启动状态

```shell
# 前端
curl -I http://127.0.0.1:8001
# 后端
curl -I http://127.0.0.1:8080/healthz
```

## 6. 常用开发命令

```shell
# 后端（admin）
cd admin
go run . migrate
go run . server

# 默认前端（web/antd-v6）
cd ../web/antd-v6
corepack pnpm@10.34.5 start:dev
corepack pnpm@10.34.5 tsc
```

## 7. 一键启动（VS Code 任务）

如果你在 `mss-boot-admin` 仓库中已配置任务，可直接使用：

- `start-project`：并行启动后端与默认 V6 前端
- `stop-project`：按端口停止后端与前端

可通过 VS Code 命令 `Tasks: Run Task` 选择对应任务。
