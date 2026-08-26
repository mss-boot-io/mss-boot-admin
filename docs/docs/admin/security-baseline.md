---
title: 安全基线
order: 7
description: v1.3.5 Admin 身份、授权、密钥、浏览器与供应链最低要求
---

# v1.3.5 安全基线

## 身份与会话

- 密码只保存带盐单向 verifier，不支持显示或恢复；
- 改密、解绑 OAuth 等敏感操作要求近期重新认证；
- 浏览器使用 HttpOnly 会话和 CSRF 防护；
- WebSocket 使用一次性短期 ticket，不在 URL 放长期凭据；
- 生产使用 HTTPS、Secure Cookie 和精确 origin。

## 授权

- 每个受保护 API 在后端执行 Casbin 或业务授权；
- 行级范围在查询和写入层同时约束；
- UI 隐藏、菜单过滤和路由守卫不是授权；
- 新权限必须有允许与拒绝测试；
- 状态变更不使用 GET。

## 密钥与配置

- 不提交 token、密码、私钥、生产 DSN、kubeconfig 或云凭据；
- 使用环境或部署平台 secret reference；
- 日志、审计、验证报告和浏览器输出脱敏；
- API key 只显示一次，服务端保存哈希；
- 可选集成失败不终止无关能力。

## 浏览器与前端

- CORS 对凭据请求使用精确来源；
- 不把服务端权限或长期令牌放入 localStorage；
- 外链、上传、富文本和动态内容按上下文转义；
- 依赖、锁文件、公开导出和生产 bundle 均纳入验证。

## 供应链与发布

- v1.3.5 只能从已经合入 `main` 的精确干净提交发布；
- 工具安装校验 `SHA256SUMS.tools-v1.3.5`；
- Go 模块关闭 workspace 验证公共解析；
- npm 包从 npmjs 匿名安装并冻结锁；
- 标签、Release 和 digest 不移动、不覆盖。

## 上线门禁

```sh
mss doctor --strict
mss verify --all
```

同时完成迁移、权限正反例、真实业务流程、浏览器控制台和回滚演练。任何绕过项必须明确
记录为阻断或剩余风险。
