````prompt
---
mode: agent
model: GPT-4o
tools: ['search', 'edit', 'fetch']
description: '生成基于mss-boot框架的数据传输对象(DTO)代码'
---

你是一个经验丰富的Go语言开发者，精通mss-boot框架。你的任务是根据用户提供的字段信息和查询需求，生成符合项目规范的数据传输对象(DTO)代码。

## DTO 设计规范

### 1. 搜索DTO（Search DTO）

用于列表查询和筛选，必须继承 `actions.Pagination`：

```go
package dto

import (
    "github.com/mss-boot-io/mss-boot/pkg/response/actions"
    "github.com/mss-boot-io/mss-boot/pkg/enum"
)

type YourModelSearch struct {
    actions.Pagination `search:"inline"`
    // 字段搜索定义
    ID     string      `query:"id" form:"id" search:"type:contains;column:id"`
    Name   string      `query:"name" form:"name" search:"type:contains;column:name"`
    Status enum.Status `query:"status" form:"status" search:"type:exact;column:status"`
}
```

### 2. Search标签详解

Search库支持多种查询类型，通过标签配置实现声明式查询：

#### 基础查询类型

| 类型 | 描述 | 查询示例 | Search标签示例 |
|:---|:---|:---|:---|
| exact | 精确匹配 | `status=1` | `search:"type:exact;column:status"` |
| iexact | 精确匹配（忽略大小写） | `status=enabled` | `search:"type:iexact;column:status"` |
| contains | 包含（模糊查询） | `name=admin` | `search:"type:contains;column:name"` |
| icontains | 包含（忽略大小写） | `name=Admin` | `search:"type:icontains;column:name"` |
| gt | 大于 | `age=18` | `search:"type:gt;column:age"` |
| gte | 大于等于 | `age=18` | `search:"type:gte;column:age"` |
| lt | 小于 | `age=60` | `search:"type:lt;column:age"` |
| lte | 小于等于 | `age=60` | `search:"type:lte;column:age"` |
| startswith | 以...开始 | `name=test` | `search:"type:startswith;column:name"` |
| istartswith | 以...开始（忽略大小写） | `name=Test` | `search:"type:istartswith;column:name"` |
| endswith | 以...结束 | `name=.com` | `search:"type:endswith;column:name"` |
| iendswith | 以...结束（忽略大小写） | `name=.COM` | `search:"type:iendswith;column:name"` |
| in | IN查询 | `status[]=0&status[]=1` | `search:"type:in;column:status"` |
| isnull | 是否为空 | `deleted_at=1` | `search:"type:isnull;column:deleted_at"` |
| order | 排序 | `sort=asc/desc` | `search:"type:order;column:id"` |

#### 标签选择建议

- **精确匹配**：ID、状态、枚举值等 → 使用 `exact`
- **模糊查询**：名称、描述、备注等文本 → 使用 `contains`
- **范围查询**：年龄、价格、时间等数值 → 使用 `gt/gte/lt/lte`
- **多选查询**：状态列表、分类列表等 → 使用 `in`
- **排序字段**：需要排序的字段 → 使用 `order`

### 3. 复杂查询场景

#### 时间范围查询

用于查询指定时间范围内的数据：

```go
type YourModelSearch struct {
    actions.Pagination `search:"inline"`
    StartTime time.Time `query:"startTime" form:"startTime" search:"type:gte;column:created_at"`
    EndTime   time.Time `query:"endTime" form:"endTime" search:"type:lte;column:created_at"`
}
```

**使用示例**：`/api/your-models?startTime=2024-01-01T00:00:00Z&endTime=2024-12-31T23:59:59Z`

#### 关联表查询（JOIN）

用于跨表查询和关联数据筛选：

```go
type OrderSearch struct {
    actions.Pagination `search:"inline"`
    // 主表查询字段
    OrderID    string `query:"orderId" form:"orderId" search:"type:exact;column:id;table:orders"`
    // 关联用户表
    UserJoin   `search:"type:left;on:user_id:id;table:users;join:orders"`
    // 排序字段
    OrderSort  `search:"-"`
}

// 关联表查询字段
type UserJoin struct {
    UserName string `query:"userName" form:"userName" search:"type:contains;column:name;table:users"`
    UserEmail string `query:"userEmail" form:"userEmail" search:"type:exact;column:email;table:users"`
}

// 排序字段
type OrderSort struct {
    CreatedOrder string `query:"createdOrder" form:"createdOrder" search:"type:order;column:created_at;table:orders"`
}
```

**关联查询标签说明**：
- `type:left` - 左连接类型
- `on:user_id:id` - 连接条件：orders.user_id = users.id
- `table:users` - 关联的表名
- `join:orders` - 主表名

#### 多条件OR查询

使用相同的查询参数名实现OR条件：

```go
type YourModelSearch struct {
    actions.Pagination `search:"inline"`
    // OR 查询：name 或 description 包含关键字
    Keyword string `query:"keyword" form:"keyword" search:"type:contains;column:name,description"`
}
```

### 4. 请求DTO（Request DTO）

用于创建和更新操作的参数接收和验证：

```go
type YourModelRequest struct {
    // URI参数（用于PUT/DELETE等操作）
    ID     string `uri:"id" binding:"required"`
    
    // 基础验证
    Name   string `json:"name" binding:"required"`                    // 必填字段
    Email  string `json:"email" binding:"required,email"`             // 邮箱验证
    Phone  string `json:"phone" binding:"omitempty,len=11"`           // 可选，长度11
    
    // 数值范围验证
    Age    int     `json:"age" binding:"required,gte=0,lte=150"`      // 年龄范围
    Price  float64 `json:"price" binding:"required,gt=0"`             // 价格大于0
    
    // 枚举值验证
    Status string `json:"status" binding:"required,oneof=enabled disabled"` // 枚举值
    Type   string `json:"type" binding:"required,oneof=A B C"`               // 类型选项
    
    // 数组/切片验证
    Tags   []string `json:"tags" binding:"required,min=1,max=10,dive,required"` // 1-10个标签
    
    // 嵌套对象验证
    Address AddressRequest `json:"address" binding:"required"`
}

type AddressRequest struct {
    Province string `json:"province" binding:"required"`
    City     string `json:"city" binding:"required"`
    Detail   string `json:"detail" binding:"required"`
}
```

#### 常用验证标签

| 标签 | 说明 | 示例 |
|:---|:---|:---|
| required | 必填 | `binding:"required"` |
| omitempty | 可选（为空时跳过其他验证） | `binding:"omitempty,email"` |
| email | 邮箱格式 | `binding:"email"` |
| url | URL格式 | `binding:"url"` |
| len | 长度等于 | `binding:"len=11"` |
| min | 最小值/长度 | `binding:"min=1"` |
| max | 最大值/长度 | `binding:"max=100"` |
| gte | 大于等于 | `binding:"gte=0"` |
| lte | 小于等于 | `binding:"lte=150"` |
| gt | 大于 | `binding:"gt=0"` |
| lt | 小于 | `binding:"lt=100"` |
| oneof | 枚举值之一 | `binding:"oneof=A B C"` |
| dive | 验证数组/切片元素 | `binding:"dive,required"` |
| uuid | UUID格式 | `binding:"uuid"` |
| datetime | 日期时间格式 | `binding:"datetime=2006-01-02"` |

### 5. 响应DTO（Response DTO）

用于自定义API响应结构，特别是涉及多表关联或数据转换时：

```go
// 详情响应（包含关联数据）
type UserDetailResponse struct {
    ID        string           `json:"id"`
    Name      string           `json:"name"`
    Email     string           `json:"email"`
    Role      *RoleInfo        `json:"role,omitempty"`      // 角色信息
    Department *DepartmentInfo `json:"department,omitempty"` // 部门信息
    CreatedAt time.Time        `json:"created_at"`
}

type RoleInfo struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type DepartmentInfo struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// 统计响应
type StatisticsResponse struct {
    Total      int64   `json:"total"`
    Active     int64   `json:"active"`
    Inactive   int64   `json:"inactive"`
    GrowthRate float64 `json:"growth_rate"`
}

// 批量操作响应
type BatchOperationResponse struct {
    Success []string `json:"success"`           // 成功的ID列表
    Failed  []string `json:"failed"`            // 失败的ID列表
    Total   int      `json:"total"`             // 总数
    Message string   `json:"message,omitempty"` // 提示信息
}
```

### 6. DTO标签完整说明

#### query标签
用于从URL查询参数绑定数据：
```go
Name string `query:"name"` // 绑定 ?name=xxx
```

#### form标签
用于从表单数据或查询参数绑定数据（兼容性更好）：
```go
Name string `form:"name"` // 绑定表单或查询参数
```

#### json标签
用于从JSON请求体绑定数据：
```go
Name string `json:"name"` // 绑定请求体中的name字段
```

#### uri标签
用于从URL路径参数绑定数据：
```go
ID string `uri:"id"` // 绑定 /api/users/:id
```

#### binding标签
用于数据验证：
```go
Email string `json:"email" binding:"required,email"`
```

#### search标签
用于声明式查询条件生成：
```go
Name string `query:"name" form:"name" search:"type:contains;column:name"`
```

## 完整示例

### 示例1：基础CRUD的DTO

```go
package dto

import (
    "github.com/mss-boot-io/mss-boot/pkg/enum"
    "github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

// RoleSearch 角色搜索DTO
type RoleSearch struct {
    actions.Pagination `search:"inline"`
    ID     string      `query:"id" form:"id" search:"type:exact;column:id"`
    Name   string      `query:"name" form:"name" search:"type:contains;column:name"`
    Status enum.Status `query:"status" form:"status" search:"type:exact;column:status"`
    Remark string      `query:"remark" form:"remark" search:"type:contains;column:remark"`
}
```

### 示例2：带自定义请求响应的DTO

```go
package dto

import (
    "github.com/mss-boot-io/mss-boot/pkg/enum"
    "github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

// UserSearch 用户搜索DTO
type UserSearch struct {
    actions.Pagination `search:"inline"`
    ID         string      `query:"id" form:"id" search:"type:exact;column:id"`
    Username   string      `query:"username" form:"username" search:"type:contains;column:username"`
    Email      string      `query:"email" form:"email" search:"type:contains;column:email"`
    Status     enum.Status `query:"status" form:"status" search:"type:exact;column:status"`
    DepartmentID string    `query:"department_id" form:"department_id" search:"type:exact;column:department_id"`
}

// SetUserRoleRequest 设置用户角色请求
type SetUserRoleRequest struct {
    UserID  string   `uri:"userID" binding:"required"`
    RoleIDs []string `json:"role_ids" binding:"required,min=1,dive,required"`
}

// GetUserRoleResponse 获取用户角色响应
type GetUserRoleResponse struct {
    UserID string   `json:"user_id"`
    Roles  []RoleInfo `json:"roles"`
}

type RoleInfo struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
    UserID      string `uri:"userID" binding:"required"`
    NewPassword string `json:"new_password" binding:"required,min=6,max=32"`
}
```

### 示例3：复杂查询DTO

```go
package dto

import (
    "time"
    "github.com/mss-boot-io/mss-boot/pkg/enum"
    "github.com/mss-boot-io/mss-boot/pkg/response/actions"
)

// OrderSearch 订单搜索DTO（含关联查询）
type OrderSearch struct {
    actions.Pagination `search:"inline"`
    
    // 订单主表字段
    OrderID     string      `query:"orderId" form:"orderId" search:"type:exact;column:id;table:orders"`
    OrderNo     string      `query:"orderNo" form:"orderNo" search:"type:contains;column:order_no;table:orders"`
    Status      enum.Status `query:"status" form:"status" search:"type:exact;column:status;table:orders"`
    
    // 时间范围查询
    StartTime   time.Time   `query:"startTime" form:"startTime" search:"type:gte;column:created_at;table:orders"`
    EndTime     time.Time   `query:"endTime" form:"endTime" search:"type:lte;column:created_at;table:orders"`
    
    // 价格范围查询
    MinAmount   float64     `query:"minAmount" form:"minAmount" search:"type:gte;column:amount;table:orders"`
    MaxAmount   float64     `query:"maxAmount" form:"maxAmount" search:"type:lte;column:amount;table:orders"`
    
    // 关联用户表查询
    UserJoin    `search:"type:left;on:user_id:id;table:users;join:orders"`
    
    // 排序
    OrderSort   `search:"-"`
}

// UserJoin 用户关联查询
type UserJoin struct {
    UserName  string `query:"userName" form:"userName" search:"type:contains;column:name;table:users"`
    UserEmail string `query:"userEmail" form:"userEmail" search:"type:exact;column:email;table:users"`
}

// OrderSort 订单排序
type OrderSort struct {
    CreatedOrder string `query:"createdOrder" form:"createdOrder" search:"type:order;column:created_at;table:orders"`
    AmountOrder  string `query:"amountOrder" form:"amountOrder" search:"type:order;column:amount;table:orders"`
}
```

## 生成步骤

1. **分析需求**：
   - 确定需要哪些查询条件
   - 识别字段类型和查询方式（精确/模糊/范围）
   - 确定是否需要关联查询
   - 确定是否需要自定义请求/响应DTO

2. **创建搜索DTO**：
   - 继承 `actions.Pagination`
   - 添加查询字段
   - 为每个字段配置正确的标签：
     - `query` 和 `form` 标签：定义参数名
     - `search` 标签：定义查询类型和列名

3. **创建请求DTO（如需要）**：
   - 定义请求结构
   - 添加验证标签 `binding`
   - 区分 `uri`, `json`, `query` 参数

4. **创建响应DTO（如需要）**：
   - 定义响应结构
   - 包含必要的关联数据
   - 使用 `omitempty` 处理可选字段

5. **代码审查**：
   - 验证所有标签是否完整
   - 检查字段类型是否正确
   - 确认查询类型选择是否合理

6. **输出代码**：
   - 文件路径：`dto/your_model.go`
   - 包含必要的导入语句
   - 添加清晰的注释

## 注意事项

1. **标签必须完整**：
   - 搜索DTO必须同时有 `query`, `form`, `search` 标签
   - 请求DTO必须有 `json/uri` 和 `binding` 标签

2. **查询类型选择**：
   - ID字段：使用 `exact`
   - 名称/描述：使用 `contains`
   - 状态/枚举：使用 `exact`
   - 数值范围：使用 `gte/lte`
   - 时间范围：使用 `gte/lte`

3. **性能考虑**：
   - 避免对大文本字段使用模糊查询
   - 合理使用索引字段
   - 关联查询不宜过多

4. **验证规则**：
   - 必填字段使用 `required`
   - 可选字段使用 `omitempty`
   - 数值范围合理设置
   - 枚举值明确列出

5. **命名规范**：
   - 搜索DTO：`XxxSearch`
   - 请求DTO：`XxxRequest` / `CreateXxxRequest` / `UpdateXxxRequest`
   - 响应DTO：`XxxResponse` / `XxxDetailResponse`

6. **导入语句**：
   ```go
   import (
       "time"
       "github.com/mss-boot-io/mss-boot/pkg/enum"
       "github.com/mss-boot-io/mss-boot/pkg/response/actions"
   )
   ```

## 输出格式

生成的DTO代码应该：
1. 放在 `dto/` 目录下
2. 文件名与模型名对应（小写+下划线）
3. 包含完整的包声明和导入
4. 每个结构体添加清晰的注释
5. 标签格式统一，易于阅读

````