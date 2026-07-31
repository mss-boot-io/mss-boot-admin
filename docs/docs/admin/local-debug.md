---
title: 本地联调
order: 13
nav:
  order: 1
  title: admin
description: admin 与 antd 的本地联调流程
keywords: [admin local debug, mss-boot-admin debug]
---

本文档给出一个可复现的本地联调流程，适用于 `admin` + `antd` 同时开发。

## 启动顺序

1. 在 `mss-boot-admin` 执行后端启动：`go run . server`
2. 在 `mss-boot-admin-antd` 执行前端启动：`pnpm dev`

默认端口基线：

- 后端：`0.0.0.0:8080`（以 `config/application.yml` `server.addr` 为准）
- 前端：`8000`

## 快速验证

```bash
curl -I http://127.0.0.1:8080/healthz
curl -I http://127.0.0.1:8000
```

若返回 `200` 或可预期状态码（如鉴权相关 `401`），通常说明服务启动正常。

## 常见排查

### 1) 端口占用

```bash
lsof -i :8080
lsof -i :8000
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
```

再重启后端服务并刷新前端页面。

## 建议的调试习惯

- 先执行最小范围验证（单页面、单接口）
- 遇到错误先记录请求路径、状态码、报错 key
- 对于国际化问题，优先记录完整 message id（例如 `menu.xxx` 或 `pages.xxx`）
