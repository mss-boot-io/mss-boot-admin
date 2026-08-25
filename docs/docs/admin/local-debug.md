---
title: 本地调试
order: 4
description: 使用 mss dev 管理 Thin Host 后端与前端
---

# 本地调试

在生成的 Thin Host 根目录执行：

```sh
mss doctor --strict
mss setup
mss dev --detach
mss dev status --format json
```

全新本地数据库第一次交互式 `setup` 会用内置隐藏提示读取管理员密码，策略为
8-128 个字符且同时包含字母和数字。非交互自动化只在 setup 进程中从密钥存储注入
`MSS_ADMIN_INITIAL_PASSWORD`；重复迁移不需要再次提供。

默认本地拓扑是后端 `8080`、前端 `8001`。以当前项目的
`.mss/commands.yaml` 为权威，不手工维护第二套启动脚本。

## 日志与停止

```sh
mss dev logs backend
mss dev logs admin-web --follow
mss dev stop
```

若服务未就绪，依次确认：

1. `doctor --strict` 是否报告版本或锁漂移；
2. 端口监听者是否属于当前项目；
3. 数据库迁移和可选资源是否就绪；
4. 前端代理目标、CORS origin 与 Cookie 设置是否一致；
5. 后端和前端日志中最早的直接失败。

不要用刷新页面或重复启动掩盖第一个失败。修复后重新执行状态、真实 API 和浏览器流程。
