---
title: Feature 与模块规格
order: 3
nav:
  title: Agent 开发
  order: 2
description: 用结构化 FeatureSpec、AcceptanceSpec 和 AdminModule 消除 Agent 开发歧义
keywords: [feature spec acceptance admin module generator]
---

# Feature 与模块规格

Agent 原生开发不等于让模型直接把自然语言变成最终代码。推荐流程是：

```text
需求描述
  ↓
FeatureSpec
  ↓
一个或多个 AdminModule
  ↓
确定性生成
  ↓
业务逻辑实现
  ↓
可执行验收
```

## FeatureSpec 解决什么问题

`Feature` 适合中大型、跨模块或存在明显权限、安全、迁移和回滚要求的需求。

位置：

```text
.mss/features/<feature>.yaml
```

Schema：

```text
.mss/schemas/feature.schema.json
```

Feature 至少表达：

- 当前问题；
- 目标与明确非目标；
- 参与角色；
- 创建、扩展、弃用或删除的模块；
- 可追踪需求；
- 权限和业务规则；
- 安全、隐私、兼容性、数据和运维约束；
- 风险及缓解措施；
- 发布、迁移与回滚；
- 每条必须需求对应的验收证据。

### 初始化

```shell
go run ./cmd/mss spec init supplier-onboarding \
  --kind feature \
  --owner procurement-platform
```

默认只输出到 stdout。写入仓库：

```shell
go run ./cmd/mss spec init supplier-onboarding \
  --kind feature \
  --owner procurement-platform \
  --write
```

默认路径：

```text
.mss/features/supplier-onboarding.yaml
```

### 验证

```shell
go run ./cmd/mss spec validate \
  .mss/features/supplier-onboarding.yaml \
  --format json
```

验证器会检查：

- ID 格式和跨类别重复；
- Requirement 引用的 Actor 和 Module 是否存在；
- Acceptance 引用的 Requirement 是否存在；
- 每个 `must` Requirement 是否至少有一个直接验收项；
- Evidence 类型和值；
- Permission 是否使用 `resource:action`；
- SpecPath 是否逃逸仓库；
- 风险、迁移和回滚是否完整；
- 未声明字段是否存在。

## AcceptanceSpec 如何工作

Acceptance 不是“测试一下”这样的自然语言占位符，而是一个可执行证据契约：

```yaml
acceptance:
  - id: finance-read-only
    requirement: supplier-review
    statement: Finance can read supplier data but receives forbidden on mutation endpoints.
    level: security
    required: true
    evidence:
      - type: test
        value: modules/supplier/tests/authorization_test.go
      - type: command
        value: go test ./modules/supplier/... -run TestFinanceReadOnly
```

支持的 level：

```text
unit
contract
integration
e2e
security
migration
manual
```

支持的 evidence：

```text
command
test
path
report
manual
```

`manual` 只能用于无法合理自动化的检查，并应说明操作者和观察标准。

## AdminModule 解决什么问题

`AdminModule` 描述一个垂直管理模块的确定性生成输入。

位置：

```text
.mss/modules/<module>.yaml
```

Schema：

```text
.mss/schemas/admin-module.schema.json
```

它包含：

- Entity、表名和字段；
- 唯一索引、搜索、校验和关联；
- Ownership 和数据范围；
- Permission；
- Menu；
- 列表、表单、详情等 UI；
- 必需测试矩阵。

示例：

```text
.mss/modules/example-supplier.yaml
```

### 验证和生成

```shell
# 统一验证器会自动识别 kind
go run ./cmd/mss spec validate .mss/modules/supplier.yaml

# dry-run，默认不写文件
go run ./cmd/mss module generate .mss/modules/supplier.yaml --format json

# 审查计划后写入
go run ./cmd/mss module generate .mss/modules/supplier.yaml --write --format json

# 检查生成物漂移
go run ./cmd/mss module generate .mss/modules/supplier.yaml --check
```

生成范围包括：

```text
modules/<name>/
  model.go
  dto.go
  repository.go
  service.go
  handler.go
  permissions.go
  migration.go
  module.go
  tests/

web/antd/src/modules/<name>/
  pages/
  services/
  locales/
  routes
  permissions

docs/modules/<name>.md
```

实际文件以生成计划为准。

## Generated 与 Custom 边界

Agent 必须把重复机械代码和业务定制分离：

- 生成文件可由规格重新计算；
- 自定义逻辑进入明确的扩展文件或扩展函数；
- 生成器不能覆盖无法从规格重建的业务代码；
- 同一规格重复生成必须没有额外 Diff；
- 修改生成物而不更新规格会被 `--check` 或 CI 发现。

## 规格变更流程

字段、权限、菜单、工作流或验收发生变化时：

1. 先修改 Feature 或 AdminModule；
2. 重新验证规格；
3. dry-run 生成；
4. 审查迁移和兼容性；
5. 写入生成物；
6. 实现非模板化业务逻辑；
7. 运行变更感知验证；
8. 更新验收证据和交接。

不要先手写 Model、DTO、页面和权限，再回头补规格。
