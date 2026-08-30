---
title: 配置指南
order: 3
description: Thin Host 的配置来源、安全优先级和部署检查
---

# 配置指南

生成仓库的 `config/` 只保存可公开的默认值和说明。环境差异与密钥由部署平台注入，
不要提交真实 DSN、密码、OAuth secret、Cookie key 或云凭据。

## 配置原则

1. 编译默认值提供可启动的本地 SQLite 基线；
2. 仓库配置只声明非敏感环境差异；
3. secret 使用环境变量或部署平台 secret reference；
4. 生产启动前验证精确来源、权限和格式；
5. 可选提供方失败只影响对应能力，不应无条件终止无关功能。

## 必查分组

| 分组 | 生产要求 |
| --- | --- |
| `server` | 明确监听地址；只在受控网络暴露 metrics、pprof |
| `application` | 使用生产模式、正确公开 origin、精确 trusted proxies |
| `cors` | 只列实际 HTTPS 前端 origin，禁止凭据请求使用通配符 |
| `database` | 明确 driver、DSN、连接限制、迁移与备份计划 |
| `auth` | 强随机 key、HTTPS、Secure Cookie、合理 SameSite 和有效期 |
| `runtime.resources` | 用引用传递凭据，限制 Redis/队列/锁的网络与账号 |
| `challenge` | 启用时配置独立 key、pepper 版本、TTL 与限流 |
| `notification` | 默认关闭；启用时使用受限账号与脱敏日志 |
| `presentation` | recovery mode 仅作启动期紧急旁路并记录审计 |

## 本地基线

本地开发默认后端端口为 `8080`，前端端口为 `8001`，前端 origin 必须与 CORS
精确匹配。先运行：

```sh
mss doctor --strict
mss dev --detach
mss dev status
```

若端口被占用，应找出并停止属于当前项目的旧进程；不要广泛终止无关服务。

## 数据库

SQLite 适合本地开发。使用 MySQL 或 PostgreSQL 时：

- 在空库执行完整迁移；
- 从上一受支持版本执行升级路径测试；
- 验证唯一索引、外键、时间和 JSON 语义；
- 多写步骤使用显式事务；
- 在发布前完成可恢复备份演练。

## 浏览器会话与 CORS

生产必须使用 HTTPS 并开启 Secure Cookie。允许来源使用完整 scheme、host、port 精确
值；反向代理只信任实际控制的 IP/CIDR。WebSocket 使用一次性短期 ticket，不在 URL
传递长期令牌。

## 密钥引用

示例只使用引用，不写值：

```yaml
runtime:
  resources:
    main:
      provider:
        kind: redis
        redis:
          credentials:
            kind: password
            password:
              passwordRef: env://MSS_RUNTIME_REDIS_PASSWORD
```

日志、报告、浏览器控制台和 CI 输出同样必须脱敏。

## 上线前检查

```sh
mss doctor --strict
mss verify --all
```

另需验证真实 `/healthz`、`/readyz`、登录、权限拒绝、迁移、关键业务写操作和前端
深链刷新。容器存活不能替代这些合同。
