---
title: 权限与组织治理说明
order: 18
nav:
  order: 1
  title: admin
description: mss-boot-admin 权限体系、组织管理、数据权限与治理架构说明
keywords: [admin governance permission rbac casbin organization]
---

## 概述

本文档描述 `mss-boot-admin` 的权限与组织治理架构，包括：

- 用户、角色、部门、岗位的关系模型
- 菜单与 API 权限控制机制
- 数据权限（Data Scope）实现
- Casbin 权限引擎集成方式

目标读者：需要理解系统权限边界、准备接入新模块或评估治理能力的开发者和架构师。

## 核心治理模型

### 1. 用户管理

**数据模型** (`models/user.go`)

```
User
├── UserLogin (嵌入结构)
│   ├── RoleID       → 关联角色
│   ├── DepartmentID → 关联部门
│   ├── PostID       → 关联岗位
│   └── Scope()      → 数据权限过滤方法
├── Name, Email, Phone, Avatar
├── Status (启用/禁用)
└── OAuth2 绑定信息
```

**关键 API 入口**

| 路径 | 功能 | 权限控制 |
|------|------|----------|
| `/admin/api/user/login` | 用户登录 | JWT 签发 |
| `/admin/api/user/info` | 获取当前用户信息 | 加载 Casbin 权限策略 |
| `/admin/api/user` | 用户 CRUD | 标准 RBAC 控制 |
| `/admin/api/user/change-password` | 用户修改密码 | 需要原密码验证 |

**权限加载流程**

登录成功后，`UserInfo` 接口通过 `gormdb.Enforcer.GetFilteredPolicy()` 加载用户角色的所有权限策略，返回给前端用于菜单渲染和按钮权限判断。

### 2. 角色管理

**数据模型** (`models/role.go`)

```
Role
├── Name        → 角色名称
├── Root        → 是否超级管理员角色
├── Default     → 是否默认角色（新用户自动绑定）
├── Status      → 启用状态
```

**关键 API 入口**

| 路径 | 功能 | 说明 |
|------|------|------|
| `/admin/api/role` | 角色 CRUD | 标准管理接口 |
| `/admin/api/role/authorize/:roleID` | 获取角色权限 | 返回菜单/API权限绑定 |
| `/admin/api/role/authorize/:roleID` (POST) | 设置角色权限 | 写入 Casbin 策略 |

**权限绑定机制**

`SetAuthorize` 接口通过事务操作 `CasbinRule` 表：
- 先清除该角色的所有现有策略
- 再批量插入新的菜单/API权限绑定
- Casbin watcher 实时同步策略变更

### 3. 部门管理

**数据模型** (`models/department.go`)

```
Department (树形结构)
├── ParentID    → 上级部门
├── Children    → 下级部门列表
├── LeaderID    → 部门负责人
├── Name, Code, Sort
├── Status
```

**关键 API 入口**

| 路径 | 功能 | 特点 |
|------|------|------|
| `/admin/api/department` | 部门树 CRUD | 支持 depth 预加载 |
| `/admin/api/department/tree` | 部门树查询 | 返回完整层级结构 |

**在权限体系中的作用**

- 用户绑定部门后，可参与"当前部门"类型的数据权限计算
- 部门层级支持"当前及下级部门"的数据范围扩展
- 部门负责人字段为后续扩展预留

### 4. 岗位管理

**数据模型** (`models/post.go`)

```
Post (树形结构)
├── ParentID    → 上级岗位
├── Children    → 下级岗位列表
├── DataScope   → 数据权限范围枚举
├── Name, Code, Sort
├── Status
```

**数据权限范围 (DataScope)**

| 值 | 含义 | 数据可见范围 |
|---|------|-------------|
| `All` | 全部数据 | 无过滤 |
| `CurrentDept` | 本部门数据 | `department_id = user.department_id` |
| `CurrentAndChildrenDept` | 本部门及下级 | 部门树子查询 |
| `CustomDept` | 自定义部门 | 配置的部门列表 |
| `Self` | 仅本人数据 | `created_by = user.id` |
| `SelfAndChildren` | 本人及直属下级 | 待扩展 |
| `SelfAndAllChildren` | 本人及所有下级 | 待扩展 |

**Scope 方法实现**

`UserLogin.Scope()` 方法根据岗位的 `DataScope` 值，应用对应的 GORM Scope 过滤条件，实现数据层权限控制。

## 菜单与 API 权限

### 5. 菜单模型

**数据模型** (`models/menu.go`)

```
Menu (树形结构)
├── Type        → 菜单类型枚举
│   ├── DIRECTORY → 目录节点（仅展示）
│   ├── MENU      → 功能菜单（可点击）
│   ├── API       → API 权限节点
│   ├── COMPONENT → 组件权限节点
├── Permission  → 权限标识符
├── Path        → 路由路径/API路径
├── Method      → HTTP 方法（仅 API 类型）
├── ParentID    → 父菜单
├── Name, Icon, Sort
├── Status
```

**菜单类型与权限关系**

| 类型 | 用途 | 权限控制方式 |
|------|------|-------------|
| DIRECTORY | 导航分组 | 仅影响菜单树结构 |
| MENU | 功能入口 | 决定用户是否可见该菜单 |
| API | 接口权限 | Casbin 控制接口调用 |
| COMPONENT | UI 组件 | 前端按钮/组件权限判断 |

### 6. API 模型

**数据模型** (`models/api.go`)

```
API (自动发现)
├── Path        → API 路径
├── Method      → HTTP 方法
├── Group       → API 分组
├── Description → 接口描述
├── Status      → 启用状态
```

**自动发现机制**

系统启动时扫描注册的路由，自动记录 API 入口，用于：
- API 管理界面的接口清单
- API 权限绑定的候选来源

### 7. Casbin 权限引擎

**策略模型** (`models/casbin_rule.go`)

```
CasbinRule
├── PType       → 策略类型 (p = policy)
├── V0          → 角色 ID
├── V1          → 权限类型 (MENU/API/COMPONENT)
├── V2          → 资源路径 (菜单ID/API路径)
├── V3          → 动作 (GET/POST/PUT/DELETE)
├── V4-V5       → 扩展字段
```

**策略结构示例**

```
p, role_1, MENU, menu_dashboard, GET    → 角色1可访问仪表盘菜单
p, role_1, API, /admin/api/user, GET    → 角色1可调用用户查询接口
p, role_1, API, /admin/api/user, POST   → 角色1可调用用户创建接口
```

**中间件集成** (`middleware/auth.go`)

每个请求经过 `Authorizator` 函数：
1. 从 JWT 获取用户角色 ID
2. 调用 `gormdb.Enforcer.Enforce(roleID, accessType, path, method)`
3. 根据策略结果决定是否放行

**实时同步**

配置 Casbin watcher 通过消息队列实现策略实时同步，权限变更无需重启服务。

## 权限控制流程

### 登录流程

```
用户输入凭证
    ↓
验证账号密码
    ↓
签发 JWT Token (包含 RoleID)
    ↓
前端请求 UserInfo
    ↓
加载 Casbin 权限策略
    ↓
返回用户信息 + 权限列表
    ↓
前端渲染菜单 + 按钮权限
```

### API 调用流程

```
前端发起请求 (携带 JWT)
    ↓
中间件解析 Token
    ↓
获取用户 RoleID
    ↓
Casbin Enforce(roleID, APIAccessType, path, method)
    ↓
策略匹配 → 放行/拒绝
    ↓
执行业务逻辑
    ↓
数据权限 Scope 过滤
    ↓
返回结果
```

### 权限变更流程

```
管理员修改角色权限
    ↓
调用 SetAuthorize 接口
    ↓
事务写入 CasbinRule 表
    ↓
Watcher 广播策略变更
    ↓
各节点同步策略
    ↓
新权限立即生效
```

## 与 HotGo 等项目的对比

| 能力维度 | mss-boot-admin | HotGo |
|----------|----------------|-------|
| 功能权限 | Casbin RBAC | Casbin RBAC |
| 数据权限 | Post.DataScope 枚举 | 多种 Scope 类型 + 自定义字段 |
| 菜单权限 | 树形菜单 + 类型区分 | 菜单 + API 绑定 |
| 实时同步 | Watcher + Queue | 实时生效 |
| 自定义部门 | ✅ 已支持 (CustomDept) | ✅ 支持 |
| 多租户 | 已移除（单租户架构） | SaaS 多身份支持 |

**CustomDept 功能已完善**

当前 `mss-boot-admin` 已支持完整的 `CustomDept` 自定义部门数据权限：

1. **后端实现** (`models/post.go`)
   - `DataScopeCustomDept` 枚举值已定义
   - `DeptIDSArr` 字段存储自定义部门 ID 列表
   - `UserLogin.Scope()` 方法正确处理 CustomDept 类型
   - `GetCustomDepartmentUserID()` 查询自定义部门下的用户

2. **前端实现** (`src/pages/Post/index.tsx`)
   - 岗位编辑表单支持 `dataScope` 选择
   - 当选择 `customDept` 时，动态显示部门多选器
   - 支持部门树形结构搜索和多选

3. **使用方式**
   - 创建/编辑岗位时，选择数据权限为"自定义部门"
   - 在弹出的部门选择器中勾选需要授权的部门
   - 该岗位的用户将只能查看所选部门的数据

**后续优化方向**

1. 增加权限变更审计日志
2. 补充权限继承与角色模板能力
3. 优化数据权限的自动注入体验

## 推荐阅读

- [产品方向调整](/admin/product-direction)
- [当前功能总览](/admin/current-capabilities)
- [运营能力说明](/admin/operations-guide)
- [四期路线图](/admin/phase-4-roadmap)