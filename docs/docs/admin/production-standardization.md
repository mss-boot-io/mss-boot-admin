---
title: 生产部署标准化
order: 27
nav:
  order: 1
  title: admin
description: mss-boot-admin 的单机、容器、代理与运维基线说明
keywords: [admin production deploy standardization nginx redis mysql]
---

## 目标

把 `mss-boot-admin` 的生产部署从“能启动”提升到“可复用、可巡检、可运维”。

## 推荐部署基线

### 组件建议

| 组件 | 推荐方案 | 说明 |
|------|----------|------|
| 数据库 | MySQL | 本地可用 SQLite，生产建议 MySQL |
| 缓存 | Redis | WebSocket 集群、缓存、部分配置依赖 |
| 反向代理 | Nginx | 统一入口、静态资源与 WS 代理 |
| 日志 | 文件输出 | 配合日志清理任务 |
| 上传 | 本地目录或 S3 | 单机优先本地，扩展优先对象存储 |

## 标准启动顺序

1. 启动数据库
2. 启动 Redis
3. 执行数据库迁移
4. 启动后端服务
5. 启动前端服务
6. 配置 Nginx
7. 验证 `/healthz`、前端首页、`/public/`、WebSocket

## 必备配置基线

```yaml
server:
  addr: 0.0.0.0:8080

logger:
  path: logs
  stdout: file
  level: info

task:
  enable: true
  spec: "0 */1 * * * *"

application:
  staticPath:
    /public: public
```

## 目录基线

```text
logs/     # 运行日志
public/   # 本地上传文件
config/   # 部署配置
backup/   # 数据库或配置备份
```

## 必做巡检

- [ ] 后端健康检查正常
- [ ] 前端页面可访问
- [ ] 登录后首页可正常加载
- [ ] 上传文件可通过 `/public/` 访问
- [ ] 日志目录持续写入
- [ ] 至少一个启用任务的 `checked_at` 正常变化
- [ ] Redis 可用时 WebSocket 集群模式正常启用

## 不建议的做法

- 不建议生产继续使用默认账号密码
- 不建议把通知密钥直接写死在仓库文件中
- 不建议关闭日志输出
- 不建议在未验证 `/public/` 代理时直接上线头像/上传功能

## 推荐阅读

- [容器化与生产部署](/admin/docker)
- [发布验证清单](/admin/release-verification-checklist)
- [安全基线指南](/admin/security-baseline)
