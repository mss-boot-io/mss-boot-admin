---
title: 部署指南
order: 13
nav:
  title: 指南
  order: 1
keywords: [deployment, docker, production, install]
---

# 部署概述

mss-boot-admin 支持多种部署方式：

- **Docker 部署** - 推荐，快速部署
- **二进制部署** - 传统部署方式
- **Kubernetes 部署** - 容器编排部署

---

# Docker 部署

## 快速启动

### 使用 Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  mysql:
    image: mysql:8
    container_name: mss-mysql
    environment:
      MYSQL_ROOT_PASSWORD: your_password
      MYSQL_DATABASE: mss_boot_admin
      MYSQL_CHARSET: utf8mb4
      MYSQL_COLLATION: utf8mb4_unicode_ci
    volumes:
      - mysql-data:/var/lib/mysql
    ports:
      - "3306:3306"
    restart: unless-stopped

  backend:
    image: mss-boot-io/mss-boot-admin:latest
    container_name: mss-backend
    environment:
      DB_DSN: "root:your_password@tcp(mysql:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"
      SERVER_ADDR: ":8080"
      JWT_SECRET: "your-jwt-secret-key"
      stage: "prod"
    ports:
      - "8080:8080"
    depends_on:
      - mysql
    restart: unless-stopped

  frontend:
    image: mss-boot-io/mss-boot-admin-antd:latest
    container_name: mss-frontend
    environment:
      API_BASE_URL: "http://backend:8080"
    ports:
      - "8000:80"
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  mysql-data:
```

启动服务：

```bash
docker-compose up -d
```

查看状态：

```bash
docker-compose ps
```

访问系统：http://localhost:8000

## 自定义配置

### 后端配置

通过环境变量配置：

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| DB_DSN | 数据库连接 | - |
| SERVER_ADDR | 服务端口 | :8080 |
| JWT_SECRET | JWT 密钥 | - |
| stage | 环境 | local |
| LOG_LEVEL | 日志级别 | info |

### 前端配置

通过环境变量配置：

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| API_BASE_URL | 后端地址 | http://localhost:8080 |

---

# 二进制部署

## 构建后端

### 编译

```bash
cd mss-boot-admin

# 编译 Linux 版本
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o mss-boot-admin main.go

# 编译 Windows 版本
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o mss-boot-admin.exe main.go
```

### 配置文件

创建 `config/application-prod.yml`：

```yaml
server:
  addr: ":8080"
  name: "mss-boot-admin"

database:
  dsn: "root:password@tcp(127.0.0.1:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"

jwt:
  secret: "your-jwt-secret-key"
  expire: "2h"

logger:
  level: "info"
  json: true
  stdout: "file"
```

### 启动服务

```bash
# 设置环境
export stage=prod

# 启动
./mss-boot-admin server
```

或使用 systemd 管理服务：

创建 `/etc/systemd/system/mss-boot-admin.service`：

```ini
[Unit]
Description=mss-boot-admin backend service
After=network.target mysql.service

[Service]
Type=simple
User=mss
WorkingDirectory=/opt/mss-boot-admin
Environment="stage=prod"
ExecStart=/opt/mss-boot-admin/mss-boot-admin server
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
systemctl daemon-reload
systemctl enable mss-boot-admin
systemctl start mss-boot-admin
systemctl status mss-boot-admin
```

## 构建前端

### 编译

```bash
cd mss-boot-admin-antd

# 构建
pnpm build
```

构建产物在 `dist/` 目录。

### Nginx 配置

创建 `/etc/nginx/sites-available/mss-boot-admin.conf`：

```nginx
server {
    listen 80;
    server_name your-domain.com;

    root /opt/mss-boot-admin-antd/dist;
    index index.html;

    # 前端路由
    location / {
        try_files $uri $uri/ /index.html;
    }

    # API 代理
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 静态资源缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 30d;
        add_header Cache-Control "public, immutable";
    }
}
```

启用配置：

```bash
ln -s /etc/nginx/sites-available/mss-boot-admin.conf /etc/nginx/sites-enabled/
nginx -t
systemctl reload nginx
```

---

# Kubernetes 部署

## Namespace

创建 namespace：

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: mss-boot
```

## ConfigMap

创建配置：

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mss-boot-admin-config
  namespace: mss-boot
data:
  application.yml: |
    server:
      addr: ":8080"
    database:
      dsn: "root:password@tcp(mysql:3306)/mss_boot_admin?charset=utf8mb4&parseTime=True&loc=Local"
    jwt:
      secret: "your-jwt-secret-key"
    logger:
      level: "info"
      json: true
```

## Secret

创建敏感配置：

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mss-boot-admin-secret
  namespace: mss-boot
type: Opaque
stringData:
  jwt-secret: "your-jwt-secret-key"
  db-password: "your_db_password"
```

## Deployment

### 后端部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mss-boot-admin
  namespace: mss-boot
spec:
  replicas: 3
  selector:
    matchLabels:
      app: mss-boot-admin
  template:
    metadata:
      labels:
        app: mss-boot-admin
    spec:
      containers:
      - name: backend
        image: mss-boot-io/mss-boot-admin:latest
        ports:
        - containerPort: 8080
        env:
        - name: stage
          value: "prod"
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: mss-boot-admin-secret
              key: jwt-secret
        volumeMounts:
        - name: config
          mountPath: /app/config
          readOnly: true
        resources:
          requests:
            cpu: "100m"
            memory: "256Mi"
          limits:
            cpu: "500m"
            memory: "512Mi"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: mss-boot-admin-config
```

### 前端部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mss-boot-admin-antd
  namespace: mss-boot
spec:
  replicas: 2
  selector:
    matchLabels:
      app: mss-boot-admin-antd
  template:
    metadata:
      labels:
        app: mss-boot-admin-antd
    spec:
      containers:
      - name: frontend
        image: mss-boot-io/mss-boot-admin-antd:latest
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: "50m"
            memory: "128Mi"
          limits:
            cpu: "200m"
            memory: "256Mi"
```

## Service

创建服务：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mss-boot-admin
  namespace: mss-boot
spec:
  selector:
    app: mss-boot-admin
  ports:
  - port: 8080
    targetPort: 8080
  type: ClusterIP

---
apiVersion: v1
kind: Service
metadata:
  name: mss-boot-admin-antd
  namespace: mss-boot
spec:
  selector:
    app: mss-boot-admin-antd
  ports:
  - port: 80
    targetPort: 80
  type: ClusterIP
```

## Ingress

创建入口：

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: mss-boot-admin-ingress
  namespace: mss-boot
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: your-domain.com
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: mss-boot-admin
            port:
              number: 8080
      - path: /
        pathType: Prefix
        backend:
          service:
            name: mss-boot-admin-antd
            port:
              number: 80
```

---

# 生产环境检查清单

部署前检查：

## 1. 数据库

- [ ] 数据库已创建
- [ ] 字符集为 utf8mb4
- [ ] 用户权限已配置
- [ ] 数据库连接池参数已优化

## 2. 配置文件

- [ ] JWT 密钥已修改（不要使用默认值）
- [ ] 数据库密码已配置
- [ ] 日志级别为 info 或 error
- [ ] 端口配置正确

## 3. 安全配置

- [ ] JWT 密钥强度足够（至少 32 字符）
- [ ] HTTPS 已启用（生产环境必需）
- [ ] CORS 配置正确
- [ ] 防火墙规则已配置

## 4. 监控配置

- [ ] Prometheus 指标端点可访问
- [ ] 健康检查端点可访问
- [ ] 日志收集已配置
- [ ] 告警规则已配置

## 5. 性能优化

- [ ] 数据库连接池大小合理
- [ ] 资源限制已配置（CPU/内存）
- [ ] 静态资源缓存已配置
- [ ] gzip 压缩已启用

---

# 运维建议

## 日志管理

- 日志文件定期清理
- 日志级别动态调整
- 日志归档备份

## 数据备份

定期备份数据库：

```bash
# MySQL 备份
mysqldump -uroot -p mss_boot_admin > backup_$(date +%Y%m%d).sql

# Docker MySQL 备份
docker exec mysql mysqldump -uroot -pPASSWORD mss_boot_admin > backup.sql
```

## 监控告警

配置关键指标告警：

- CPU 使用率 > 80%
- 内存使用率 > 80%
- 磁盘使用率 > 80%
- 服务不可用

## 版本升级

升级步骤：

1. 备份数据库
2. 拉取新版本代码
3. 执行数据库迁移（如有）
4. 重新部署服务
5. 验证功能

---

# 下一步

- [核心功能](/guide/features) - 功能模块说明
- [常见问题](/guide/faq) - 部署问题解答
- [配置教程](/admin/configuration-guide) - 详细配置说明