---
title: 安全基线指南
order: 30
nav:
  order: 1
  title: admin
description: mss-boot-admin 的默认配置风险、凭据注入与上传安全建议
keywords: [admin security baseline upload credentials]
---

## 目标

把当前版本上线前必须确认的安全基线整理成最小清单。

## 一、默认配置风险

上线前必须确认：

- [ ] 默认账号密码已修改
- [ ] 默认数据库连接未直接暴露到公网
- [ ] 默认 Redis 密码已替换
- [ ] 通知渠道 webhook 与 SMTP 凭据未写死在仓库中

## 二、凭据注入建议

优先使用环境变量或部署系统注入：

- 数据库连接串
- Redis 密码
- SMTP 用户名与密码
- DingTalk Secret
- WeChat Webhook Key

## 三、上传安全建议

`D0-safety` 内部检查点只覆盖 Upload admission 与 Local write boundary：

- `storage:maxSize` 是 bytes 整数；默认 10 MiB（`10485760` bytes），硬上限
  100 MiB（`104857600` bytes），非法或越界配置拒绝上传。
- `storage:allowedTypes` 是逗号分隔的 MIME types / `type/*` wildcards，例如
  `image/png,image/*`；文件扩展名不是安全策略。
- 这两个字段是 Storage AppConfig 的完整 allowlist。provider、endpoint、bucket 与
  凭据 key 的历史行不会返回；提交这些已移除的 key 会以稳定 422 整批拒绝。
- Provider 与 SecretRef 只能来自进程启动时的不可变 profile，不允许通过 Admin
  设置或 AppConfig API 注入、轮换或切换。
- 请求体在 multipart 解析前受限，选中文件还会做 max-plus-one 流式检查。
- Local 使用 `uploads/<opaque-uuid>`、`os.Root` confinement 与 `O_EXCL`
  create-only 写入；错误或取消清理 partial。用户 ID 和原始文件名不进入 key。

D1 的对象子切片也已完成：Provider / SecretRef 在启动时从 immutable profile
一次性解析，未知/非法 profile 拒绝安装对象资源，client 由单一 owner 管理。Admin
进程继续运行，两条上传路由固定返回 503，零 Local fallback。Local 只在 dev 模式与
`application.staticPath` 精确映射配置 root 时安装；`prod` Local 不安装。S3 在 D1
只持有 client，并在 `Put` 前返回 503。

这不等于生产存储已经就绪；Local 与 S3-compatible 仍为 `Legacy / Blocked`。
Nginx 代理、目录挂载、endpoint 拼接或 opaque key 不能单独补齐 Delivery 与授权边界。

上线前必须：

- [ ] `storage:maxSize` 已按 bytes 设置为业务可接受值，且不超过 `104857600`
- [ ] `storage:allowedTypes` 仅包含必要的 MIME types / wildcards
- [ ] ingress 阻断 `/admin/api/storage/upload` 与 `/admin/api/user/avatar` 的生产流量，
  同时不授予通用 `storage:upload` 权限作为纵深防御；头像入口没有独立 Casbin permission
- [ ] 未将 `/public/`、endpoint 拼接 URL 或 opaque key 当成对象读取授权

D1 的对象子切片已完成，且不能把旧 AppConfig 行作为兼容回退；Kafka lifecycle 仍未
完成。真实 S3 Put/Delivery、RustFS fixture 和 Local/S3-compatible 共用 conformance
suite 留在 `D4-authorization-object`。精确边界见
[D1 Object Provider/Owner 内部 checkpoint](/releases/v1-1-0-d1-object-provider-owner)。

## 四、通知渠道安全建议

- 邮件账号应使用独立发送账号
- 钉钉与企微 webhook 建议专用机器人
- 凭据变更后应做最小联通性验证

## 五、日志与审计建议

- 保留登录日志
- 保留审计日志
- 为日志清理任务设置明确保留周期

## 六、上线前最小安全检查

- [ ] 默认密码已替换
- [ ] 凭据通过环境变量注入
- [ ] 上传限制已按 bytes 与 MIME/wildcard 合同配置
- [ ] Legacy / Blocked 上传 provider 在生产入口保持关闭
- [ ] 告警通知渠道可用且未泄露凭据
- [ ] 日志保留周期已配置

## 推荐阅读

- [生产部署标准化](/admin/production-standardization)
- [容器化与生产部署](/admin/docker)
- [SECURITY Policy FAQ](/devops/security-policy-faq)
