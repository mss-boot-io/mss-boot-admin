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

当前系统已支持：

- 文件大小限制
- 文件类型白名单
- MIME 检测

上线前建议明确：

- [ ] `storage:maxSize` 已设置为业务可接受值
- [ ] `storage:allowedTypes` 仅保留必要类型
- [ ] `/public/` 访问路径已纳入代理控制

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
- [ ] 上传限制已配置
- [ ] 告警通知渠道可用且未泄露凭据
- [ ] 日志保留周期已配置

## 推荐阅读

- [生产部署标准化](/admin/production-standardization)
- [容器化与生产部署](/admin/docker)
- [SECURITY Policy FAQ](/devops/security-policy-faq)
