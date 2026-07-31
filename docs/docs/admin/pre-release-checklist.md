---
title: 发布前检查清单
order: 30
nav:
  order: 1
  title: admin
description: mss-boot-admin 发布前必须完成的检查项，确保生产环境安全稳定
keywords: [admin release checklist production deployment]
---

## 概述

本文档列出了 `mss-boot-admin` 正式发布前必须完成的所有检查项。

> **检查时间**：2026-04-03
> **当前版本**：产品打磨三轮完成后

---

## ✅ 已完成项

### 代码质量

- [x] **TypeScript 错误清零**
  - `pnpm exec tsc --noEmit` 通过
  - 无类型错误

- [x] **后端测试通过**
  - `go test ./...` 通过
  - 核心模块测试覆盖

- [x] **API 响应格式统一**
  - 所有变更操作返回 `{}` 而非 `null`
  - 响应信封一致（code, msg, data）

- [x] **三轮整改完成**
  - P0 稳定性问题全部修复
  - P1 体验一致性改进完成
  - P2 结构抽象已启动

### 功能验证

- [x] **核心功能测试**
  - 登录/登出 ✅
  - 用户管理 ✅
  - 角色权限 ✅
  - 部门岗位 ✅
  - 日志管理 ✅
  - 系统监控 ✅

- [x] **边界测试**
  - 大数据量加载 ✅
  - 特殊字符处理 ✅
  - SQL 注入防护 ✅

### 文档完善

- [x] **测试文档**
  - 全量测试用例 ✅
  - 测试执行报告 ✅
  - 测试数据脚本 ✅

- [x] **产品文档**
  - 产品打磨方案 ✅
  - 整改清单 ✅
  - HotGo 对比 ✅

---

## ⚠️ 必须完成项

### 1. 生产环境配置

#### 1.1 安全配置

**必须修改**：

```yaml
# config/application.yml
auth:
  key: 'YOUR-PRODUCTION-SECRET-KEY'  # ❌ 不要使用默认值
  timeout: '12h'
```

**配置方式**：
```bash
# 方式1: 环境变量
export AUTH_KEY="your-random-secret-key-at-least-32-chars"

# 方式2: 配置文件
auth:
  key: '${AUTH_KEY}'
```

**生成随机密钥**：
```bash
openssl rand -base64 32
```

#### 1.2 数据库配置

**生产环境建议**：

```yaml
database:
  driver: mysql  # 或 postgres
  source: '${DB_DSN}'  # 使用环境变量
  config:
    maxOpenConns: 100
    maxIdleConns: 10
    connMaxLifetime: 1h
```

**环境变量**：
```bash
export DB_DSN="user:password@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
```

#### 1.3 Redis 配置

**必须修改**：

```yaml
cache:
  redis:
    addr: '${REDIS_ADDR}'
    password: '${REDIS_PASSWORD}'  # ❌ 不要使用默认值 123456
```

**环境变量**：
```bash
export REDIS_ADDR="your-redis-host:6379"
export REDIS_PASSWORD="your-redis-password"
```

#### 1.4 通知配置

**邮件通知**：

```yaml
notification:
  email:
    enabled: true
    host: "smtp.example.com"
    port: 587
    username: "noreply@example.com"
    password: '${EMAIL_PASSWORD}'
    from: "noreply@example.com"
```

---

### 2. 安全加固

#### 2.1 默认账号

**必须修改默认密码**：

1. 首次启动后立即修改 `admin` 密码
2. 或在迁移脚本中强制修改

**建议**：
```go
// cmd/migrate/migration 中添加首次启动密码修改检查
```

#### 2.2 敏感信息检查

**检查清单**：

- [ ] 配置文件中无硬编码密码
- [ ] `.gitignore` 排除敏感文件
- [ ] 环境变量正确设置
- [ ] 日志不输出敏感信息

**检查命令**：
```bash
# 检查代码中是否有硬编码密码
grep -r "password.*=.*\"" --include="*.go" --exclude-dir=vendor
grep -r "secret.*=.*\"" --include="*.go" --exclude-dir=vendor
```

#### 2.3 HTTPS 配置

**生产环境必须使用 HTTPS**：

```yaml
# config/ssl.go
ssl:
  enabled: true
  cert: /path/to/cert.pem
  key: /path/to/key.pem
```

**Nginx 配置示例**：
```nginx
server {
    listen 443 ssl http2;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location /admin/api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

### 3. 数据库准备

#### 3.1 数据库创建

```sql
CREATE DATABASE mss_boot_admin
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;

CREATE USER 'mss_admin'@'%' IDENTIFIED BY 'strong-password';
GRANT ALL PRIVILEGES ON mss_boot_admin.* TO 'mss_admin'@'%';
FLUSH PRIVILEGES;
```

#### 3.2 迁移执行

```bash
# 设置数据库连接
export DB_DSN="mss_admin:strong-password@tcp(host:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"

# 执行迁移
cd mss-boot-admin
go run . migrate
```

#### 3.3 初始数据

**必须创建**：
- 默认管理员账号
- 默认角色
- 默认菜单

**已在迁移脚本中处理** ✅

---

### 4. 前端构建

#### 4.1 环境变量

```bash
# .env.production
API_BASE_URL=https://your-domain.com/admin/api
```

#### 4.2 构建命令

```bash
cd mss-boot-admin-antd
pnpm install
pnpm build
```

#### 4.3 静态资源

**部署位置**：
```yaml
# mss-boot-admin config/application.yml
application:
  staticPath:
    /public: public  # 前端静态资源
```

**或使用 Nginx**：
```nginx
location / {
    root /path/to/mss-boot-admin-antd/dist;
    try_files $uri $uri/ /index.html;
}
```

---

### 5. 文档更新

#### 5.1 CHANGELOG

**必须创建**：`CHANGELOG.md`

```markdown
# Changelog

## [Unreleased]

### Added
- Phase 4: Runtime logs, monitoring, alerts, i18n improvements
- Phase 5: Production deployment, testing, security baseline

### Fixed
- Login page undefined status issue
- Department/Post navigation redirect paths
- Password reset success logic
- Frontend polling cleanup
- Menu API binding nil dereference risk
- Monitor disk partition out of bounds risk
- Alert checker duplicate close panic risk

### Changed
- Unified API response format
- Monitoring data precision (2 decimal places)
- Auth shell component abstraction
- Error response standardization

### Security
- Sensitive endpoint authentication coverage
- File upload validation
- Audit logging through service layer
```

#### 5.2 部署文档

**已存在**：`docs/admin/docker.md` ✅

**需要补充**：
- [ ] 生产环境配置示例
- [ ] 性能调优建议
- [ ] 常见问题排查

#### 5.3 API 文档

**Swagger 已生成** ✅

```bash
# 访问
https://your-domain.com/swagger/index.html
```

---

### 6. 监控告警

#### 6.1 监控指标

**必须配置**：
- [ ] CPU 使用率告警（> 80%）
- [ ] 内存使用率告警（> 85%）
- [ ] 磁盘使用率告警（> 90%）
- [ ] 服务健康检查

#### 6.2 日志收集

**建议配置**：
```yaml
logger:
  path: /var/log/mss-boot-admin
  level: info
  json: true  # 生产环境使用 JSON 格式
```

#### 6.3 告警通知

**配置邮件/钉钉/微信通知**：
```yaml
notification:
  email:
    enabled: true
  dingtalk:
    enabled: true
  wechat:
    enabled: true
```

---

### 7. 备份策略

#### 7.1 数据库备份

**定时备份脚本**：
```bash
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
mysqldump -h host -u user -p'password' mss_boot_admin > /backup/mss_boot_admin_$DATE.sql
find /backup -name "mss_boot_admin_*.sql" -mtime +7 -delete
```

**Cron 配置**：
```cron
0 2 * * * /path/to/backup.sh
```

#### 7.2 文件备份

**需要备份**：
- 上传的文件（`public/` 目录）
- 配置文件
- 日志文件（可选）

---

## 📋 发布检查流程

### 发布前（Pre-release）

```bash
# 1. 代码检查
cd mss-boot-admin
go test ./...
go vet ./...

cd mss-boot-admin-antd
pnpm exec tsc --noEmit
pnpm lint

# 2. 构建验证
pnpm build

# 3. 安全检查
grep -r "password.*=.*\"" --include="*.go"
grep -r "secret.*=.*\"" --include="*.go"

# 4. 文档检查
# CHANGELOG.md 存在
# README.md 更新
# 部署文档完整
```

### 发布中（Release）

```bash
# 1. 创建标签
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# 2. 构建发布版本
# 后端
go build -o mss-boot-admin .

# 前端
pnpm build

# 3. 部署
# 按照部署文档执行
```

### 发布后（Post-release）

```bash
# 1. 验证部署
curl https://your-domain.com/admin/api/healthz

# 2. 检查日志
tail -f /var/log/mss-boot-admin/app.log

# 3. 监控指标
# 访问监控页面确认数据正常

# 4. 通知团队
# 发送发布通知邮件/消息
```

---

## 🚨 禁止项

### 绝对不要

1. **不要在生产环境使用默认密码**
   - `auth.key`
   - `redis.password`
   - 管理员账号密码

2. **不要提交敏感信息到 Git**
   - `.env` 文件
   - 密钥文件
   - 生产配置文件

3. **不要禁用 HTTPS**
   - 生产环境必须使用 HTTPS

4. **不要跳过数据库备份**
   - 发布前必须确认备份策略

5. **不要忽略日志和监控**
   - 必须配置日志收集
   - 必须配置监控告警

---

## 📊 检查清单总结

### 必须项（Blocker）

- [ ] 修改 `auth.key` 生产密钥
- [ ] 修改 `redis.password` 生产密码
- [ ] 修改默认管理员密码
- [ ] 配置 HTTPS
- [ ] 配置数据库备份
- [ ] 创建 CHANGELOG.md

### 建议项（Recommended）

- [ ] 配置监控告警
- [ ] 配置日志收集
- [ ] 性能压测
- [ ] 安全扫描
- [ ] 文档完善

### 可选项（Optional）

- [ ] CDN 配置
- [ ] 负载均衡
- [ ] 自动化部署
- [ ] 灰度发布

---

## 发布决策

### 可以发布

当所有 **必须项** 完成时：
- ✅ 可以发布到生产环境
- ✅ 可以对外提供服务

### 建议延迟发布

如果有以下情况：
- ❌ 存在未修复的安全漏洞
- ❌ 核心功能测试失败
- ❌ 性能指标不达标
- ❌ 文档不完整

---

## 推荐阅读

- [Docker 部署指南](/admin/docker)
- [生产部署标准化](/admin/production-standardization)
- [安全基线](/admin/security-baseline)
- [发布验证清单](/admin/release-verification-checklist)