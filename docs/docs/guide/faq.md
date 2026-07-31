---
title: 常见问题
order: 14
nav:
  title: 指南
  order: 1
keywords: [faq, troubleshooting, help, support]
---

# 安装问题

## Q: Go 版本要求是什么？

**A:** 需要 Go 1.21 或更高版本。

检查当前版本：

```bash
go version
```

如果版本过低，访问 [Go 官网](https://go.dev/dl/) 下载最新版本。

## Q: MySQL 版本要求是什么？

**A:** 推荐 MySQL 8.0 或更高版本。

MySQL 8.0 提供更好的 utf8mb4 支持和性能优化。

## Q: Node.js 版本要求是什么？

**A:** 需要 Node.js 18.16.0 或更高版本。

检查版本：

```bash
node -v
npm -v
```

## Q: 前端依赖安装失败怎么办？

**A:** 尝试以下步骤：

1. 清理缓存：

```bash
pnpm store prune
```

2. 删除现有依赖：

```bash
rm -rf node_modules pnpm-lock.yaml
```

3. 重新安装：

```bash
pnpm install
```

4. 如果仍有问题，尝试使用 npm：

```bash
npm install
```

---

# 启动问题

## Q: 后端启动失败，提示数据库连接错误？

**A:** 检查以下几点：

1. **MySQL 服务是否启动**：

```bash
# Docker 方式
docker ps | grep mysql

# 本地安装
systemctl status mysql
```

2. **数据库是否创建**：

```bash
mysql -uroot -p -e "show databases;"
```

如果没有，创建数据库：

```sql
CREATE DATABASE mss_boot_admin DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

3. **DSN 配置是否正确**：

检查环境变量或配置文件中的 `DB_DSN`：

```bash
echo $DB_DSN
```

格式应为：

```
root:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&loc=Local
```

4. **防火墙是否允许连接**：

确保 3306 端口可访问。

## Q: 后端端口冲突怎么办？

**A:** 修改端口配置：

方式一：环境变量：

```bash
export SERVER_ADDR=":9080"
```

方式二：配置文件 `config/application-local.yml`：

```yaml
server:
  addr: ':9080'
```

## Q: 前端端口冲突怎么办？

**A:** 修改端口配置：

创建 `.env.local`：

```
PORT=9000
```

或启动时指定：

```bash
pnpm dev --port 9000
```

## Q: 数据库迁移失败怎么办？

**A:** 检查以下问题：

1. **数据库连接正常**：

```bash
mysql -uroot -p -h 127.0.0.1 -e "select 1;"
```

2. **用户权限足够**：

确保用户有 CREATE、ALTER、INSERT 权限。

3. **查看详细错误**：

```bash
go run main.go migrate -v
```

4. **手动执行迁移**：

如果自动迁移失败，可以手动执行 SQL 文件（在 `cmd/migrate/migration/` 目录）。

## Q: 如何生成或新增数据表？

**A:** 当前推荐使用 Go model + migration 的方式新增数据表，不再推荐把
虚拟模型作为新项目的主路径。

推荐流程：

1. 在后端新增或调整 Go model，明确字段、索引和关联关系。
2. 在 migration 中添加数据库结构变更。
3. 执行迁移：

```bash
go run main.go migrate
```

4. 启动服务并验证 API、菜单、角色权限和前端页面是否符合预期。
5. 如果这个表会暴露给后台用户，补充对应的权限、菜单、文档和测试说明。

虚拟模型仍可能存在于存量代码中，但它已经是降级能力。新需求应优先走可审查
的 Go model、migration 和权限治理路径。

---

# 功能问题

## Q: 登录后提示"权限不足"？

**A:** 检查以下几点：

1. **用户是否分配角色**：

在用户管理中，检查用户是否有角色分配。

2. **角色是否有权限**：

在角色管理中，检查角色的菜单权限和 API 权限。

3. **菜单权限配置**：

确保菜单已正确分配给角色。

## Q: 菜单不显示？

**A:** 检查以下几点：

1. **菜单状态**：

菜单管理中，确保菜单状态为"启用"。

2. **用户角色权限**：

确保用户角色有该菜单的权限。

3. **菜单可见性**：

检查菜单的"可见"属性是否为"是"。

4. **前端路由配置**：

检查前端路由是否正确配置。

## Q: API 请求返回 401 Unauthorized？

**A:** 检查以下几点：

1. **Token 是否有效**：

检查请求头是否包含有效的 Authorization token。

2. **Token 是否过期**：

JWT Token 默认有效期 2 小时，过期后需重新登录或刷新。

3. **API 权限配置**：

在角色管理中，确保角色有该 API 的权限。

登录链路的完整排查步骤请参考 [登录排障](/admin/login-troubleshooting)。

## Q: 文件上传失败？

**A:** 检查以下几点：

1. **存储配置**：

检查对象存储配置是否正确（S3、OSS 等）。

2. **文件大小限制**：

检查服务器和 Nginx 的文件大小限制。

Nginx 配置：

```nginx
client_max_body_size 100M;
```

3. **文件类型限制**：

检查允许的文件类型配置。

4. **权限问题**：

确保上传目录有写入权限。

---

# 配置问题

## Q: JWT Token 有效期如何修改？

**A:** 在配置文件中修改：

```yaml
jwt:
  secret: 'your-secret-key'
  expire: '4h' # 修改为 4 小时
```

或通过环境变量：

```bash
export JWT_EXPIRE="4h"
```

## Q: 日志级别如何修改？

**A:** 在配置文件中修改：

```yaml
logger:
  level: 'debug' # debug / info / warn / error
```

或通过环境变量：

```bash
export LOG_LEVEL="debug"
```

动态修改（运行时）：

在监控页面可以动态调整日志级别。

## Q: 如何配置 OAuth2 第三方登录？

**A:** 配置步骤：

1. 在第三方平台（GitHub、飞书）创建应用，获取 Client ID 和 Client Secret。

2. 配置文件：

```yaml
oauth2:
  github:
    clientID: 'your-client-id'
    clientSecret: 'your-client-secret'
    redirectURL: 'http://your-domain/api/v1/oauth2/github/callback'
```

3. 前端配置登录入口。

## Q: 如何配置邮件通知？

**A:** 配置 SMTP：

```yaml
email:
  host: 'smtp.example.com'
  port: 587
  username: 'your-email@example.com'
  password: 'your-password'
  from: 'noreply@example.com'
```

---

# 部署问题

## Q: Docker 容器启动后无法访问？

**A:** 检查以下几点：

1. **容器状态**：

```bash
docker ps
docker logs mss-backend
```

2. **端口映射**：

确保端口正确映射：

```bash
docker port mss-backend
```

3. **网络连接**：

```bash
docker network ls
docker network inspect bridge
```

4. **防火墙规则**：

确保防火墙允许访问端口。

## Q: Kubernetes 部署后服务无法访问？

**A:** 检查以下几点：

1. **Pod 状态**：

```bash
kubectl get pods -n mss-boot
kubectl describe pod <pod-name> -n mss-boot
kubectl logs <pod-name> -n mss-boot
```

2. **Service 状态**：

```bash
kubectl get svc -n mss-boot
kubectl describe svc mss-boot-admin -n mss-boot
```

3. **Ingress 配置**：

```bash
kubectl get ingress -n mss-boot
kubectl describe ingress mss-boot-admin-ingress -n mss-boot
```

4. **DNS 解析**：

确保域名正确解析到 Ingress IP。

## Q: 生产环境性能慢怎么办？

**A:** 优化建议：

1. **数据库优化**：

- 增加连接池大小
- 添加索引
- 查询优化

```yaml
database:
  maxOpen: 100
  maxIdle: 20
  maxLifetime: '1h'
```

2. **缓存配置**：

使用 Redis 缓存热点数据。

3. **资源限制**：

合理配置 CPU 和内存限制。

4. **静态资源**：

- 启用 CDN
- 启用 gzip 压缩
- 配置缓存策略

---

# 安全问题

## Q: 如何报告疑似安全漏洞？

**A:** 不要在公开 Issue 中披露漏洞细节、token、密码、生产日志或可直接
滥用的复现步骤。请先阅读 [SECURITY Policy FAQ](/devops/security-policy-faq)，
并按受影响仓库的 `SECURITY.md` 路径提交私密报告。

## Q: 如何修改默认密码？

**A:** 首次登录后立即修改：

1. 登录系统（admin / 123456）
2. 进入个人中心
3. 修改密码

生产环境强制要求修改默认密码。

## Q: JWT 密钥强度要求？

**A:** 建议：

- 最少 32 字符
- 包含大小写字母、数字、特殊字符
- 不要使用简单字符串

生成强密钥：

```bash
openssl rand -base64 32
```

## Q: 如何启用 HTTPS？

**A:** 配置 SSL 证书：

**Nginx 配置**：

```nginx
server {
    listen 443 ssl;
    server_name your-domain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 其他配置...
}
```

**直接配置**：

```yaml
server:
  addr: ':443'
  certFile: '/path/to/cert.pem'
  keyFile: '/path/to/key.pem'
```

## Q: CORS 配置如何设置？

**A:** 配置允许的域名：

```yaml
cors:
  allowOrigins:
    - 'https://your-domain.com'
    - 'https://admin.your-domain.com'
  allowMethods:
    - 'GET'
    - 'POST'
    - 'PUT'
    - 'DELETE'
  allowHeaders:
    - 'Authorization'
    - 'Content-Type'
```

---

# 其他问题

## Q: 如何查看系统日志？

**A:** 多种方式：

1. **Docker 日志**：

```bash
docker logs mss-backend
docker logs -f mss-backend  # 实时查看
```

2. **日志文件**：

查看配置的日志文件路径。

3. **系统登录日志**：

在后台"日志管理"查看用户登录日志。

4. **操作日志**：

在后台"日志管理"查看用户操作日志。

## Q: 如何备份数据？

**A:** 备份方案：

1. **数据库备份**：

```bash
mysqldump -uroot -p mss_boot_admin > backup.sql
```

2. **配置文件备份**：

备份 `config/` 目录。

3. **定期备份**：

设置定时任务定期备份。

## Q: 如何升级版本？

**A:** 升级步骤：

1. **备份数据库**：

```bash
mysqldump -uroot -p mss_boot_admin > backup_before_upgrade.sql
```

2. **拉取新版本**：

```bash
git pull origin main
```

3. **检查变更**：

查看 CHANGELOG 了解变更内容。

4. **执行迁移**：

如果有数据库变更：

```bash
go run main.go migrate
```

5. **重启服务**：

```bash
docker-compose restart
# 或
systemctl restart mss-boot-admin
```

6. **验证功能**：

测试核心功能是否正常。

## Q: 如何获取帮助？

**A:** 多种渠道：

1. **在线文档**：

https://docs.mss-boot-io.top

2. **GitHub Issues**：

https://github.com/mss-boot-io/mss-boot-admin/issues

3. **社区交流**：

- 微信群
- QQ 群
- Telegram 群

4. **视频教程**：

https://space.bilibili.com/597294782

---

# 更多问题？

如有其他问题，请通过 GitHub Issues 提交：

https://github.com/mss-boot-io/mss-boot-admin/issues/new

我们会在第一时间回复。
