---
title: 集成验证
order: 5
description: v1.3.5 Thin Host 的分层自动检查和浏览器验收
---

# v1.3.5 集成验证

先运行最小相关检查，再扩大范围。所有报告必须说明实际执行命令、结果和未执行项。

## 统一入口

```sh
mss verify --changed
mss verify --all
```

`--changed` 按 Git 差异选择检查；`--all` 用于合并、升级或交付前完整资格验证。

## 后端

```sh
GOWORK=off go test ./...
GOWORK=off go build ./cmd/server
```

持久化变化还需覆盖空库迁移和上一版本升级；权限变化同时需要允许与拒绝测试；多写步骤
需验证事务、冲突和幂等语义。

## 前端

```sh
corepack pnpm@10.34.5 --dir web install --frozen-lockfile
corepack pnpm@10.34.5 --dir web lint
corepack pnpm@10.34.5 --dir web test
corepack pnpm@10.34.5 --dir web build
```

生成 API、路由、菜单与 locale 发生变化时必须运行漂移检查，不能只验证 TypeScript
编译。

## 浏览器验收

使用 Codex 内置浏览器验证：

- 登录和会话恢复；
- 允许和拒绝两种权限路径；
- 加载、空、可重试错误、403、404、冲突；
- 桌面与窄屏；
- zh-CN 与 en-US；
- 深链刷新、键盘焦点和控制台；
- 关键写操作后的真实服务端结果。

不得用源码内的 standalone browser 脚本代替用户要求的内置浏览器证据。

## 证据

记录版本、提交、工具版本、数据库、命令、退出码、浏览器路径和已知限制。容器健康、
TCP 可达、任务显示 done 或页面出现都只是局部证据。
