---
title: 本地联调
order: 13
nav:
  order: 1
  title: admin
description: admin 与 Ant Design V6 的本地联调流程
keywords: [admin local debug, mss-boot-admin debug]
---

本文档给出一个可复现的本地联调流程，适用于 `admin` + `web/antd-v6` 同时开发。

## 启动顺序

1. 首次迁移或路由变更后，在 `admin/` 执行一次 `STAGE=local go run . server -a`，
   把实际挂载路由同步到 API 注册表；命令成功后会自行退出。
2. 在仓库根目录执行 `go run ./cmd/mss dev`，默认启动后端与 V6 前端。
3. 如需分别调试，可在 `admin/` 执行 `go run . server`，在 `web/antd-v6/`
   执行 `corepack pnpm@10.34.5 start:dev`。

默认端口基线：

- 后端：`0.0.0.0:8080`（以 `config/application.yml` `server.addr` 为准）
- V6 前端：`8001`

## 快速验证

```bash
curl -I http://127.0.0.1:8080/healthz
curl -I http://127.0.0.1:8001
```

若返回 `200` 或可预期状态码（如鉴权相关 `401`），通常说明服务启动正常。

## 常见排查

### 1) 端口占用

```bash
lsof -i :8080
lsof -i :8001
```

若被占用，先停止冲突进程再重启服务。

### 2) 前端可访问但接口失败

优先确认：

- 后端是否已启动
- 前端代理配置是否指向正确后端地址
- 登录态是否过期（控制台常见 `401`）

### 3) 数据迁移后页面异常

先在后端仓库执行：

```bash
go run . migrate
STAGE=local go run . server -a
```

再重启后端服务并刷新前端页面。

### 4) 菜单“绑定 API”为空

先停止正在占用后端配置资源的本地服务，在 `admin/` 使用同一 stage 和数据库配置执行：

```bash
STAGE=local go run . server -a
```

该命令只同步路由并退出。重新启动后端后再次打开绑定窗口；如果仍为空，再检查同步命令
使用的数据库是否与当前后端完全一致。

## 建议的调试习惯

- 先执行最小范围验证（单页面、单接口）
- 遇到错误先记录请求路径、状态码、报错 key
- 对于国际化问题，优先记录完整 message id（例如 `menu.xxx` 或 `pages.xxx`）
