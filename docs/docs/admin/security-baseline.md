---
title: 安全基线
order: 7
description: v1.3.7 稳定版 Admin 身份、授权、密钥、浏览器与供应链最低要求
---

# v1.3.7 稳定版安全基线

:::warning
发布状态：**v1.3.7 是当前稳定版**；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布。
Docs 网站可通过 `docs/v*` 异步候补，不影响组件、稳定别名或采用。
:::

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

- v1.3.7 的 Root 工具、Go Module、官方 npmjs 包和多架构镜像都来自同一个精确
  merged-main 提交；安装时固定版本或 digest，并核验 checksum、npm provenance 与来源；
- v1.3.5 与 v1.3.6 是永久停止的部分发布列车，已公开标签、Release、包与镜像保持不可变，
  不补附缺失制品，也不与 v1.3.7 混用；
- v1.3.5 的 `SHA256SUMS.tools-v1.3.5` 只是未发布资产身份，v1.3.6 的 Root Release/工具
  也不能补齐其失败的 Root image 与官方 npm；
- GitHub Packages、本地 tarball、源码 checkout 或 `replace` 不能替代官方 npmjs 与公共
  Go Module 对账；
- Docs Tag 只发布网站，独立不可变且可异步候补，不参与组件完整性或采用门禁；
- 后续完整版本同样必须从精确干净的 merged-main 提交资格，并冻结标签、Release 和 digest。

## 上线门禁

Foundation 贡献者按 `.mss/commands.yaml` 运行源码诊断与验证；一般 `doctor`、`verify`
合同可以保留，但这些结果不能替代 v1.3.7 公共制品证据，也不能证明已停止列车可用。

同时完成迁移、权限正反例、真实业务流程、浏览器控制台和回滚演练。任何绕过项必须明确
记录为阻断或剩余风险。
