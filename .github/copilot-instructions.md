# mss-boot-admin 项目开发指南

## 项目概述
mss-boot-admin 是一个基于 Gin + React + Ant Design v5 + Umi v4 + mss-boot 的前后端分离权限管理系统。本项目采用现代化的微服务架构设计，支持多租户、RBAC权限管理、国际化、虚拟模型等企业级功能。

## 核心技术栈

### 后端技术
- **Web框架**: Gin - 高性能的Go HTTP框架
- **ORM**: GORM - 数据库操作
- **权限管理**: Casbin - RBAC权限控制
- **认证**: gin-jwt + OAuth2 - 支持JWT和第三方登录
- **API文档**: Swag + gin-swagger - 自动生成Swagger文档
- **数据库**: 支持 MySQL 8.0+, PostgreSQL, SQLite, SQL Server, DM(达梦)
- **缓存**: Redis + redislock
- **消息队列**: NSQ, Kafka (支持AWS MSK)
- **配置中心**: Consul, 支持多配置源(本地文件、embed、S3、数据库、MongoDB)
- **监控**: Pyroscope (性能分析), Prometheus (指标采集)
- **云服务**: AWS SDK (EKS, S3, Secrets Manager)

### 核心依赖库
- **mss-boot**: 内部核心框架库
- **协议**: gRPC, WebSocket (gorilla/websocket)
- **调度**: robfig/cron - 定时任务
- **工具**: spf13/cobra (命令行), go-git (Git操作)

## 项目架构

### 目录结构规范

```
mss-boot-admin/
├── apis/          # API控制器层 - 定义RESTful接口和路由
├── cmd/           # 命令行入口 - Cobra命令定义
│   ├── migrate/   # 数据库迁移命令
│   └── server/    # 服务启动命令
├── config/        # 配置管理 - 支持多配置源
├── dto/           # 数据传输对象 - 请求/响应结构体
├── models/        # 数据模型层 - GORM实体定义
├── service/       # 业务逻辑层 - 核心业务处理
├── middleware/    # 中间件 - 认证、权限、租户等
├── router/        # 路由配置 - 注册控制器
├── center/        # 中心化服务 - 租户、缓存、队列管理
├── pkg/           # 工具包 - 通用工具函数
├── notice/        # 通知模板 - 邮件等通知
└── compose/       # Docker Compose配置 - 本地开发环境
```

### 核心设计模式

#### 1. **多租户架构 (Multi-Tenancy)**
- 通过 `ModelGormTenant` 基类实现租户隔离
- 租户通过请求头 `Referer` 中的域名识别
- 支持租户级别的数据库作用域过滤
- 核心类型: `models.Tenant`, `center.TenantImp`

```go
// 示例：多租户模型定义
type YourModel struct {
    models.ModelGormTenant  // 继承多租户基类
    // 其他字段...
}
```

#### 2. **权限管理 (RBAC with Casbin)**
- 基于 Casbin 的 RBAC 权限控制
- 支持角色-菜单-API三级权限映射
- 数据权限支持7种范围: 全部数据、本部门、本部门及子部门、自定义部门、仅本人、本人及下属、本人及所有下属
- 核心类型: `models.Role`, `models.Menu`, `models.API`, `pkg.AccessType`

```go
// 权限类型枚举
const (
    MenuAccessType      AccessType = "menu"      // 菜单权限
    APIAccessType       AccessType = "api"       // API权限
    ComponentAccessType AccessType = "component" // 组件权限
)
```

#### 3. **认证系统 (Authentication)**
- JWT Token认证机制
- 支持多种登录方式:
  - 用户名密码登录 (`pkg.UsernameLoginProvider`)
  - GitHub OAuth2 登录 (`pkg.GithubLoginProvider`)
  - 飞书/Lark OAuth2 登录 (`pkg.LarkLoginProvider`)
  - 邮箱验证码登录 (`pkg.EmailLoginProvider`)
  - 邮箱注册登录 (`pkg.EmailRegisterProvider`)
- 支持Personal Access Token (PAT)
- 核心类型: `middleware.Auth`, `models.UserLogin`, `models.UserAuthToken`

#### 4. **虚拟模型 (Virtual Model)**
- 支持动态配置模型字段和行为
- 前端可通过配置生成CRUD界面
- 核心类型: `models.Model`, `models.Field`

#### 5. **统计分析 (Statistics)**
- 内置统计接口，自动跟踪数据变化
- 支持实时统计和定时校准
- 核心类型: `models.Statistics`, `center.StatisticsImp`

```go
// 实现统计接口
func (*YourModel) StatisticsName() string { return "your-model-total" }
func (*YourModel) StatisticsType() string { return "your-model" }
func (*YourModel) StatisticsTime() string { return pkg.NowFormatDay() }
func (*YourModel) AfterCreate(tx *gorm.DB) error {
    _ = center.Default.NowIncrease(ctx, model)
    return nil
}
```

### 数据库设计规范

#### 基础模型类型

1. **ModelGorm** - 基础模型（无租户）
   - ID (varchar 64)
   - CreatedAt, UpdatedAt, DeletedAt (软删除)

2. **ModelGormTenant** - 多租户模型
   - 继承 ModelGorm
   - TenantID (varchar 64) - 租户隔离字段
   - CreatorID (varchar 64) - 创建人（支持数据权限）
   - Remark (text) - 备注

3. **命名规范**
   - 表名前缀: `mss_boot_`
   - 外键命名: `{model}_id`
   - 索引: 租户ID、创建人ID、状态字段需建索引

### API开发规范

#### 控制器开发模式

使用 `response.Controller` 框架自动生成RESTful API:

```go
// 示例：标准控制器定义
func init() {
    e := &YourController{
        Simple: controller.NewSimple(
            controller.WithAuth(true),                      // 启用认证
            controller.WithModel(new(models.YourModel)),    // 关联模型
            controller.WithSearch(new(dto.YourModelSearch)), // 搜索DTO
            controller.WithModelProvider(actions.ModelProviderGorm), // 数据提供者
            controller.WithScope(center.Default.Scope),     // 作用域
        ),
    }
    response.AppendController(e) // 注册控制器
}

type YourController struct {
    *controller.Simple
}
```

#### 自动生成的路由
- `GET /api/your-models` - 列表查询
- `GET /api/your-models/:id` - 详情查询
- `POST /api/your-models` - 创建
- `PUT /api/your-models/:id` - 更新
- `DELETE /api/your-models/:id` - 删除

#### Swagger注解规范

```go
// @Summary     接口摘要
// @Description 详细描述
// @Tags        标签分组
// @Accept      json
// @Produce     json
// @Param       参数名 位置 类型 是否必须 "描述"
// @Success     200 {object} response.Response{data=YourType} "成功响应"
// @Failure     400 {object} response.Response "错误响应"
// @Router      /api/your-path [method]
// @Security    Bearer
```

### 数据传输对象 (DTO) 规范

#### 搜索DTO
```go
type YourModelSearch struct {
    response.Search                       // 继承分页搜索基类
    Name            string `form:"name"` // 查询字段
    Status          string `form:"status"`
}
```

#### 请求/响应DTO
- 使用 `json` tag 定义JSON字段
- 使用 `binding` tag 定义验证规则
- 使用 `query` tag 定义查询参数，但是要同时使用 `form` tag 定义表单/查询参数
- 使用 `uri` tag 定义URI参数

```go
type YourModelRequest struct {
    ID       string `uri:"id" binding:"required"` // URI参数
    OK       bool   `query:"ok" form:"ok"` // 查询参数
    Name     string `json:"name" binding:"required"` // 必填字段
    Status   string `json:"status" binding:"required,oneof=active inactive"` // 枚举值验证
    CreatedAt string `json:"created_at" binding:"datetime"` // 日期时间格式验证
    // 其他字段...
}
```

### 配置管理

#### 配置优先级
1. 环境变量 (最高优先级)
2. 命令行参数
3. Secret配置 (AWS Secrets Manager等)
4. 远程配置中心 (Consul等)
5. 本地配置文件
6. Embed配置 (编译时嵌入)

#### 核心配置项
```go
type Config struct {
    Auth        Auth            // JWT认证配置
    Server      Listen          // HTTP服务配置
    GRPC        GRPC            // gRPC服务配置
    Database    Database        // 数据库配置
    Cache       *Cache          // 缓存配置
    Queue       *Queue          // 队列配置
    Locker      *Locker         // 分布式锁配置
    Storage     *Storage        // 对象存储配置
    Application Application     // 应用配置
    Task        Task            // 定时任务配置
    Pyroscope   Pyroscope       // 性能分析配置
    Clusters    Clusters        // 集群配置(K8s)
}
```

#### 环境变量约定
- `DB_DSN` - 数据库连接字符串（必需）
- `REDIS_ADDR` - Redis地址
- `CONFIG_CENTER` - 配置中心地址
- `LOG_LEVEL` - 日志级别

### 中间件系统

#### 内置中间件
1. **Auth** - JWT认证 (`middleware.Auth`)
2. **CORS** - 跨域处理
3. **Tenant** - 租户识别
4. **Casbin** - 权限校验
5. **Logger** - 日志记录
6. **Recovery** - 异常恢复

#### 中间件注册
```go
// 注册到middleware.Middlewares
middleware.Middlewares.Store("your-middleware", YourMiddlewareFunc())
```

### 任务调度系统

#### 定时任务开发
```go
// models/task.go
type Task struct {
    models.ModelGormTenant
    Name     string         `json:"name"`     // 任务名称
    Cron     string         `json:"cron"`     // Cron表达式
    Handler  string         `json:"handler"`  // 处理器名称
    Params   string         `json:"params"`   // 任务参数(JSON)
    Status   enum.Status    `json:"status"`   // 任务状态
}

// 注册任务处理器
pkg.RegisterTaskHandler("your-handler", YourHandlerFunc)
```

### 通知系统

#### 支持的通知方式
- 邮件通知 (SMTP)
- 站内消息 (`models.Notice`)
- WebSocket推送 (实时通知)

#### 邮件模板
- 模板路径: `notice/email/*.html`
- 支持变量替换

### 国际化 (i18n)

#### 多语言支持
- 语言配置存储在 `models.Language`
- 前端通过API获取语言资源
- 支持动态添加语言和翻译

### 测试规范

#### 单元测试
- 测试文件命名: `*_test.go`
- 测试数据: `testdata/` 目录

#### 集成测试
- 使用Docker Compose启动依赖服务
- 配置文件: `compose/*/docker-compose.yml`

### 部署规范

#### 启动流程
```bash
# 1. 设置数据库连接
export DB_DSN="user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"

# 2. 数据库迁移
go run main.go migrate

# 3. 生成API文档和路由
go run main.go server -a

# 4. 启动服务
go run main.go server
```

#### Docker部署
- Dockerfile提供标准镜像构建
- 支持多阶段构建优化镜像体积

### 代码生成工具

#### 微服务代码生成
- 基于模板生成微服务脚手架
- 模板管理: `models.Template`
- 生成器: `pkg.Generator`

### 性能优化建议

1. **数据库优化**
   - 使用连接池
   - 添加必要索引
   - 使用预加载减少N+1查询
   - 启用查询缓存

2. **缓存策略**
   - 热点数据Redis缓存
   - 使用redislock避免缓存击穿
   - 设置合理的过期时间

3. **并发控制**
   - 使用队列削峰填谷
   - 分布式锁避免竞态
   - 使用context控制超时

### 安全规范

1. **密码安全**
   - 使用Salt加密存储
   - 密码强度校验
   - 支持密码重置

2. **Token管理**
   - JWT支持刷新机制
   - Token可撤销（数据库黑名单）
   - 支持Personal Access Token

3. **SQL注入防护**
   - 使用GORM参数化查询
   - 避免原始SQL拼接

4. **XSS防护**
   - 输入验证
   - 输出转义

### 常见问题排查

#### 1. 租户识别失败
- 检查请求头 `Referer` 是否包含正确域名
- 确认租户域名已在 `mss_boot_tenant_domains` 表中配置

#### 2. 权限验证失败
- 检查Casbin策略是否加载
- 确认角色-菜单-API关联关系
- 查看 `mss_boot_casbin_rules` 表数据

#### 3. 数据库迁移问题
- 检查 `DB_DSN` 环境变量
- 确认数据库版本兼容性
- 查看 `cmd/migrate/migration/` 迁移文件

### 开发建议

1. **遵循项目规范**
   - 使用项目定义的基础模型
   - 继承标准控制器
   - 遵循命名约定

2. **充分利用框架能力**
   - 使用mss-boot提供的工具方法
   - 利用自动化的CRUD生成
   - 使用中间件扩展功能

3. **注重代码质量**
   - 添加必要注释
   - 编写单元测试
   - 及时生成Swagger文档

4. **关注性能**
   - 避免N+1查询
   - 合理使用缓存
   - 监控慢查询

## 相关链接

- [在线文档](https://docs.mss-boot-io.top)
- [前端项目](https://github.com/mss-boot-io/mss-boot-admin-antd)
- [视频教程](https://space.bilibili.com/597294782/channel/seriesdetail?sid=3881026)
- [体验环境](https://admin-beta.mss-boot-io.top) (账号: admin 密码: 123456)
- [Swagger文档](https://mss-boot-io.github.io/mss-boot-admin/swagger.json)

## 贡献指南

- 提交PR前请确保通过所有测试
- 遵循Go代码规范 (gofmt, golint)
- 更新相关文档和注释
- 提供清晰的commit message

---

**项目许可**: MIT License  
**维护团队**: mss-boot-io  
**Go版本要求**: 1.21+  
**数据库要求**: MySQL 8.0+, PostgreSQL, SQLite等
