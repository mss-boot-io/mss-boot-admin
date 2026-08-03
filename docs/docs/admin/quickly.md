---
title: 快速开始
order: 11
nav:
  order: 1
  title: admin
description: 快速启动mss-boot-admin
keywords: [admin quickly start]
---

`mss-boot-admin` 是前后端分离项目，开发时需要同时启动后端 `admin` 与前端 `antd`。

## 环境要求

:::warning
go version >= 1.21

node version >= 18.16.0

pnpm version >= 8

port 8080(mss-boot-admin), 8000(mss-boot-admin-antd)
:::

## 1. 下载项目

```shell
# 下载后端项目
git clone https://github.com/mss-boot-io/mss-boot-admin.git
# 下载前端项目
git clone https://github.com/mss-boot-io/mss-boot-admin-antd.git
```

## 2. 初始化后端

后端默认可使用本地 SQLite（`config/application.yml` 中默认 `database.driver: sqlite`），可先无需额外数据库服务。

```shell
# 进入后端项目
cd mss-boot-admin
# 迁移数据库
go run . migrate
```

> 如需切换 MySQL/PostgreSQL，请按实际情况设置 `DB_DSN`，并参考仓库 `config` 与 `compose` 目录。

## 3. 启动后端服务

```shell
# 启动后端服务
go run . server
```

## 4. 启动前端服务

```shell
# 进入前端项目
cd ../mss-boot-admin-antd
# 安装依赖
pnpm install
# 启动前端服务
pnpm dev
```

## 5. 验证启动状态

```shell
# 前端
curl -I http://127.0.0.1:8000
# 后端
curl -I http://127.0.0.1:8080/healthz
```

## 6. 常用开发命令

```shell
# 后端（admin）
go run . migrate
go run . server

# 前端（antd）
pnpm dev
pnpm -s tsc --noEmit
```

## 7. 一键启动（VS Code 任务）

如果你在 `mss-boot-admin` 仓库中已配置任务，可直接使用：

- `start-project`：并行启动后端与前端
- `stop-project`：按端口停止后端与前端

可通过 VS Code 命令 `Tasks: Run Task` 选择对应任务。
