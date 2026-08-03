---
title: 历史能力降级说明
order: 24
nav:
  order: 1
  title: admin
description: mss-boot-admin 历史能力分级、定位与后续演进说明
keywords: [admin legacy capability deprecation virtual model code generation]
---

## 概述

本文档明确 `mss-boot-admin` 中历史能力的定位、分级和后续演进方向，确保产品主线清晰，避免旧叙事影响新方向。

## 背景

根据产品方向调整，`mss-boot-admin` 已从"快速生成、低代码"方向转向"治理型 + 运营型 + AI 注解协同型 admin 平台"。

部分历史能力虽然仍存在，但不再作为产品主线继续强化。本文档明确这些能力的定位。

---

## 历史能力分级定义

| 级别 | 名称 | 含义 | 后续投资 |
|------|------|------|----------|
| L1 | 保留 | 继续作为核心能力维护和演进 | 正常投入 |
| L2 | 冻结 | 保留功能，但不再新增特性 | 仅修复严重问题 |
| L3 | 弱化 | 标记为非主线，限制使用场景 | 最小维护 |
| L4 | 弃用 | 计划移除，提供迁移指南 | 仅安全修复 |
| L5 | 删除 | 已从代码库移除 | 无 |

---

## 历史能力分级表

### 动态模型 (Virtual Model)

| 属性 | 说明 |
|------|------|
| **分级** | L3 弱化 |
| **实现状态** | 功能完整，可与治理体系集成 |
| **保留原因** | 部分用户可能依赖此能力进行快速原型开发 |
| **限制说明** | 不再作为主线推荐，文档中明确为"高级/实验性功能" |
| **后续投资** | 最小维护，仅修复严重 Bug |

**实现文件：**
```
mss-boot-admin/
├── apis/model.go        # 模型管理 API
├── apis/virtual.go      # 虚拟 API 文档
├── apis/field.go        # 字段管理 API
├── models/model.go      # 模型定义
└── models/field.go      # 字段定义

mss-boot/virtual/        # 核心虚拟模型实现
├── model/               # 模型抽象
└── action/              # 动作抽象
```

**功能范围：**
- ✅ 动态定义数据模型（表名、字段、类型）
- ✅ 自动生成 REST API（CRUD）
- ✅ 自动创建菜单和权限
- ✅ 自动生成国际化资源
| ⚠️ 多租户已移除 | 单租户架构 |
- ⚠️ 需重启服务才能生效

**限制：**
1. 不适合复杂业务逻辑场景
2. 性能可能不如手写代码
3. 调试和排障成本较高
4. 无测试覆盖

### 模板/代码生成 (Template/Code Generation)

| 属性 | 说明 |
|------|------|
| **分级** | L3 弱化 |
| **实现状态** | 功能完整，通用模板引擎 |
| **保留原因** | 可作为扩展机制，支持自定义模板 |
| **限制说明** | 不提供内置模板，用户需自行准备 |
| **后续投资** | 最小维护，仅修复严重 Bug |

**实现文件：**
```
mss-boot-admin/
├── apis/template.go     # 模板 API
├── dto/template.go      # 模板 DTO
├── pkg/generator.go     # 生成器核心
├── pkg/parse.go         # 模板解析
└── pkg/git.go           # Git 操作

mss-boot-admin-antd/
└── src/pages/Generator/ # 前端生成器界面
```

**功能范围：**
- ✅ 从 GitHub 仓库获取模板
- ✅ 支持 Go template 语法
- ✅ 自动提取模板参数
- ✅ 生成后推送到目标仓库
- ⚠️ 需要用户提供 GitHub Token

**限制：**
1. 无内置模板库
2. 需要用户理解 Go template 语法
3. 需要 GitHub OAuth 认证
4. 文档不完整

---

## 主线 vs 非主线能力对照

### 主线能力（L1 保留）

| 能力 | 说明 | 文档位置 |
|------|------|----------|
| 用户管理 | 认证、授权、自服务 | 治理文档 |
| 角色管理 | RBAC 权限绑定 | 治理文档 |
| 部门管理 | 组织架构 | 治理文档 |
| 岗位管理 | 数据权限 | 治理文档 |
| 菜单管理 | 功能入口 | 治理文档 |
| API 管理 | 接口权限 | 治理文档 |
| 系统配置 | 运维配置 | 运营文档 |
| 通知公告 | 消息推送 | 运营文档 |
| 定时任务 | 作业调度 | 运营文档 |
| 监控统计 | 运行状态 | 运营文档 |
| 审计日志 | 操作追溯 | 治理文档 |
| 国际化 | 多语言 | 扩展护栏文档 |
| 对象存储 | 文件上传 | 扩展护栏文档 |
| WebSocket | 实时通信 | 扩展护栏文档 |

### 非主线能力（L3 弱化）

| 能力 | 说明 | 定位 |
|------|------|------|
| 动态模型 | 运行时定义模型 | 高级/实验性功能 |
| 代码生成 | 模板引擎 | 高级/扩展机制 |

---

## 文档表述规范

### 主线能力表述

```
✅ 正确：
"mss-boot-admin 是一个治理型 + 运营型 admin 平台，提供完善的权限治理、
运营能力和 AI 注解协同支持。"

❌ 错误：
"mss-boot-admin 是一个低代码平台，支持快速生成 CRUD 代码。"
```

### 历史能力表述

```
✅ 正确：
"动态模型能力仍保留在代码库中，适合快速原型开发场景，但不作为主线推荐。
生产环境建议使用标准 Controller 模式开发。"

❌ 错误：
"动态模型是 mss-boot-admin 的核心能力之一。"
```

### README/首页表述

```
✅ 正确：
"mss-boot-admin 提供完整的后台治理与运营能力：
- 治理核心：用户、角色、部门、岗位、权限
- 运营能力：配置、通知、任务、监控、统计
- 扩展能力：国际化、存储、WebSocket、API-first

注：仓库中仍包含动态模型和代码生成能力，但这些不再作为产品主线。"

❌ 错误：
"mss-boot-admin 支持动态模型和代码生成，快速搭建后台系统。"
```

---

## 迁移指南

### 从动态模型迁移到标准 Controller

**迁移步骤：**

1. 创建 Go 模型文件：
```go
// models/my_entity.go
package models

type MyEntity struct {
    actions.ModelGorm
    Name        string `json:"name" gorm:"size:255;not null"`
    Description string `json:"description" gorm:"type:text"`
    Status      string `json:"status" gorm:"size:20;default:'active'"`
}
```

2. 创建 API 控制器：
```go
// apis/my_entity.go
package apis

func init() {
    e := &MyEntity{
        Simple: controller.NewSimple(
            controller.WithAuth(true),
            controller.WithModel(&models.MyEntity{}),
            controller.WithModelProvider(actions.ModelProviderGorm),
        ),
    }
    response.AppendController(e)
}
```

3. 导出数据并迁移

### 从代码生成迁移

**替代方案：**

1. 使用标准 Go 项目模板（如 `template-admin`）
2. 参考 mss-boot 框架文档，手动实现
3. 使用 IDE 代码生成功能

---

## 后续演进

### 短期（1-3 月）

1. 更新所有文档，明确主线与非主线关系
2. 在 UI 中添加历史能力的"实验性"标识
3. 补充迁移指南文档

### 中期（3-6 月）

1. 收集用户反馈，评估历史能力使用情况
2. 根据反馈决定是否进一步降级（L4 弃用）
3. 完善 Controller 模式的开发文档

### 长期（6-12 月）

1. 评估是否移除历史能力代码
2. 如移除，提供完整的迁移工具和文档
3. 确保主线能力足够替代历史能力

---

## 推荐阅读

- [产品方向调整](/admin/product-direction)
- [三期路线图](/admin/phase-3-roadmap)
- [权限与组织治理说明](/admin/governance-guide)
- [运营能力说明](/admin/operations-guide)
- [集成与扩展护栏](/admin/extension-guardrails)