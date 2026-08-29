---
title: 页面展示配置完整设计
order: 17
nav:
  order: 1
  title: Admin
description: 从 P0/P1 治理基础到生成器、真实页面运行时、可视化编辑、升级漂移与恢复的完整方案
keywords: [admin presentation configuration generator visual editor drift recovery thin host]
---

# 页面展示配置完整设计

:::warning
当前状态：D0 设计已获得维护者确认，当前开发分支正在实现并验收真实页面运行时。Supplier 是首个
生成业务页面；维护者又明确授权 `user.list` 作为第二个、单独审查的 Foundation Core Page
Presentation 试点。本文记录当前实现目标，不把尚未执行或尚未基于最终精确 Head 的命令、内置浏览器、
Thin Host 或安全检查写成已通过证据。
:::

## 目标

页面展示配置解决的是日常展示调整，而不是“无代码创建业务系统”。

允许配置的内容包括：

- 页面标题、字段标题、占位说明和帮助文案；
- 列表列、搜索项、表单项、详情项的显示、顺序、宽度和栅格；
- 已注册组件之间的选择；
- 列表密度、分页大小和默认排序；
- 已注册动作的显示、顺序、位置和确认文案；
- 对已注册字段使用有界、强类型条件控制可见性；
- 应用、当前角色、当前用户三层继承与覆盖。

不允许通过展示配置创建或改变：

- 数据库表、字段、实体、模型或迁移；
- API、URL、HTTP 方法、请求头、查询逻辑或凭据；
- 路由、菜单实现、权限、工作流或动作处理器；
- React 组件实现、导入路径、远程模块或插件；
- JavaScript、HTML、SQL、表达式、模板或其他可执行内容；
- 登录、认证、授权、应用配置、发布、恢复和展示治理页面。

一句话边界：

> 已经编译、生成并授权的页面能力可以调整展示；生成业务页通过 AdminModule，单独审查的 Foundation
> 核心页通过专用机器规格进入同一确定性生成链；新增业务能力仍必须经过代码评审、迁移、测试和发布。

## 当前基础

### P0：安全展示合同

P0 已经完成：

- 严格的 `AdminPagePresentation` JSON Schema；
- 可信页面能力定义；
- `编译默认值 < 应用层 < 角色层 < 用户层` 的确定性解析；
- 定义哈希漂移检测；
- 整层拒绝和低层回退；
- 权限最后求交；
- 仅用于测试的 Supplier 原型。

P0 不包含数据库、API、发布流程和生产页面接入。

### P1：发布治理控制面

P1 已经完成：

- 应用、角色、用户范围的配置聚合；
- 未生效草稿；
- 严格结构校验和能力语义校验；
- 强 ETag、条件写入和冲突保护；
- 幂等发布；
- 不可变历史；
- 通过重新发布历史版本实现回滚；
- 读取、草稿写入、发布、回滚四项独立权限；
- 脱敏审计；
- 当前主体有效层接口；
- 启动级恢复模式；
- 静态编译的治理控制台。

P1 故意保持生产能力注册表为空，因此不会改变任何业务页面。

## 完整架构

```text
.mss/modules/<module>.yaml 或经单独审查的 .mss/core-pages/<page>.yaml
        │
        │ AdminModule 结构与语义校验
        ▼
标准化展示能力清单 + 唯一定义哈希
        │
        ├───────────────┐
        ▼               ▼
Go 可信定义         TypeScript 可信定义 + 页面适配器
        │               │
业务模块显式注册     前端静态注册表
        │               │
P1 校验/发布/有效层 ──► 浏览器解析器与渲染器
        │               │
写库与不可变历史     编译期 API/查询/动作/组件
        └───────┬───────┘
                ▼
       disabled / shadow / active
                +
          recoveryMode 强制回退
```

完整方案分为四个平面。

### 一、规格与生成平面

`AdminModule` 是生成业务页面的唯一源头。模块可以选择性增加 `spec.presentation`。
没有该段时，当前静态页面保持原样，不注册展示能力，完全向后兼容。

任意手写 Foundation 核心页默认仍然排除。当前阶段唯一例外是维护者明确授权并单独审查的
`user.list`：其唯一源头为 `.mss/core-pages/user-list.yaml`。它不能伪装成 `AdminModule`，因为现有用户
路由、服务、DTO、权限和安全规则仍属于 Foundation 核心代码；它也不能分别手写 Go 与 TypeScript
定义。该机器规格进入同一个标准化与规范哈希流水线，由生成器产出 Go 定义、TypeScript 定义、唯一
哈希、标准化 manifest 和两端注册表条目。

下面只展示 Supplier 的结构片段。P2A 提交的 AdminModule 必须完整列出与当前生产页面等价的默认项，
本片段中省略的字段不代表存在隐式默认值。

```yaml
spec:
  presentation:
    pageKey: supplier.list
    definitionVersion: "2"
    title:
      zh-CN: 供应商管理
      en-US: Suppliers
    dataSource: list
    list:
      density: large
      pageSize: 20
      defaultSort: []
      fields:
        - field: code
          component: text
          order: 10
          width: 180
        - field: name
          component: text
          order: 20
          width: 240
        - field: creditLevel
          component: tag
          order: 60
        - field: enabled
          component: boolean
          order: 70
    search:
      collapsedByDefault: false
      fields:
        - field: code
          component: input
          order: 10
        - field: creditLevel
          component: select
          order: 50
    form:
      columns: 1
      fields:
        - field: code
          component: input
          order: 10
          span: 24
        - field: enabled
          component: switch
          order: 70
          span: 24
    detail:
      columns: 1
      fields:
        - field: code
          component: text
          order: 10
          span: 24
        - field: enabled
          component: boolean
          order: 70
          span: 24
    actions:
      - action: export
        placement: toolbar
        order: 10
      - action: create
        placement: toolbar
        order: 20
      - action: read
        placement: row
        order: 30
      - action: update
        placement: row
        order: 40
      - action: delete
        placement: row
        order: 50
```

关键规则：

- `pageKey` 必须显式声明、全局唯一并长期稳定，不从路由、文件名、表名或显示名称推导；
- `definitionVersion` 只在定义合同不兼容时升级；首个生产生成合同使用 `2`，因为测试用 P0
  的 `1` 未把完整默认展示纳入哈希，不能静默沿用；
- 字段引用必须来自当前模块实体；
- 数据源来自当前模块已生成的 API 和查询适配器；
- 动作权限来自当前模块已生成的权限合同；
- 组件和字段类型/页面表面兼容关系来自 Foundation 内嵌的
  `.mss/admin-presentation-catalog.yaml` 机器合同；
- 模块只能缩小组件可选范围，不能用字符串注册新的组件实现；
- 必填、可搜索、可排序、可过滤、表单/详情可用性等事实继续来自现有字段定义；
- `spec.presentation` 不接受权限字符串、URL、方法、请求头、SQL、导入路径或处理函数。

展示源只接受未限定的本地引用：`dataSource: list` 标准化为 `<module>.list`，
`action: create` 标准化为 `<module>.create`。数据源和动作源值只要包含 `.` 就拒绝，避免出现两种限定
方式。字段 ID 保持页面本地，组件 ID 保持 Foundation 全局，`pageKey` 始终显式声明、绝不由该规则推导。
生成的数据源能力还必须携带编译 API/查询适配器已经执行的分页选项、最大分页和最大排序字段数；
profile 只能在这些范围内缩小行为，不能提供查询参数名或编码方式。

版本 `2` 的 catalog 只开放 Supplier 首闭环已经需要的最小矩阵，其他组合一律拒绝：

| 组件 | 兼容值与页面表面 |
| --- | --- |
| `text` | 字符串；列表/详情 |
| `input` | 字符串；搜索/表单 |
| `email-input` | email 格式字符串；表单 |
| `tag` | 枚举；列表/详情 |
| `select` | 枚举；搜索/表单 |
| `boolean` | 布尔；列表/详情 |
| `boolean-filter` | 布尔；搜索，使用编译期 all/true/false 编码 |
| `switch` | 布尔；表单 |
| `copyable-code` | 生成的标识字段；详情 |
| `date-time` | 生成的时间戳字段；详情 |

新增组件 ID、兼容项或 React 实现都属于独立 Foundation 变更。catalog 随 `mss` 分发内嵌；Schema
只校验结构，Go 语义校验读取 catalog，前端构建测试证明每个 catalog ID 都有且只有一个静态实现。

生成器先构造一个标准化内存清单，再从同一清单输出 Go 和 TypeScript。定义哈希只计算一次：

```text
sha256(canonical-json(normalized-capability-without-hash))
```

哈希输入包含页面、字段、表面、组件、数据源、动作、可信权限要求、分页/排序限制和完整默认展示；
不包含生成文件头、时间、路径和哈希字段自身。规范 JSON 固定使用 ASCII 属性名排序、按声明顺序再按
稳定 ID 排列数组（排序优先级除外）、不做 HTML 专用转义的 UTF-8 JSON 字符串，并只允许有界整数。
Go 与 TypeScript 必须嵌入同一个哈希，并用 Unicode、HTML 特殊字符、键序、数组序和空集合金丝雀验证。

计划生成的所有权边界：

| 产物 | 所有权 | 作用 |
| --- | --- | --- |
| `admin/modules/<module>/presentation_generated.go` | 生成器 | 后端可信能力定义 |
| `web/antd-v6/src/generated/modules/<module>/presentation.generated.ts` | 生成器 | 前端可信可序列化定义 |
| `presentation.adapter.generated.tsx` | 生成器 | 编译期查询、字段、组件和动作适配 |
| `admin/presentation/core/definitions_generated.go` | 生成器 | Foundation 核心页可信定义，在业务模块前显式注入 |
| `admin/presentation/core/manifest.generated.json` | 生成器 | 核心页标准化 manifest 与哈希一致性证据；运行时不读取 |
| `web/antd-v6/src/generated/core-presentation-registry.generated.ts` | 生成器 | 静态导入的 Foundation 核心页注册表 |
| 后端/前端应用注册表索引 | 生成器 | 显式列出启用模块 |
| 每模块标准化 manifest 快照 | 生成器 | 双端一致性证据和以后升级前后结构化 diff 输入；运行时不读取 |
| 模块 `custom` 扩展文件 | 业务代码 | 通过强类型入口提供编译期自定义实现 |

生成文件不手改。生成器必须支持 dry-run、路径限制、稳定排序、旧生成文件清理和连续两次运行零差异。

Foundation 与 Thin Host 的所有权不对称：只有 Foundation 生成过程读取
`.mss/core-pages/user-list.yaml`；Thin Host 从固定版本的 Admin Go 与 npm 包消费生成后的核心定义和
静态注册表，不复制该 YAML，也不把 Foundation 核心产物重新生成到业务自有路径。

### 二、发布与控制平面

P1 的发布模型保持不变：

```text
草稿保存 -> 校验 -> 明确发布 -> 不可变历史
                         │
                         └-> 回滚 = 把历史内容重新发布成新版本
```

后端变化只在能力来源和组合方式：

- 应用组合根先显式注入生成的 Foundation 核心页定义，再组合业务模块；两类来源页面键冲突时启动前失败；
- 每个业务模块在一个注册事务中同时暂存描述、迁移、就绪检查、路由和展示能力；
- 任一部分失败，整个模块注册都不可见；
- 注册表冻结后再挂载业务路由和启动监听；
- 服务通过依赖注入获得应用级不可变注册表；
- 禁止包 `init`、数据库行、请求参数、目录扫描或插件修改注册表；
- 重复页面键、保护页面键、错误哈希和不完整默认值会在启动前失败。

治理控制台即使在展示应用关闭时也能读取可信能力、创建草稿和走发布流程。数据库中的文档不能定义
或修改校验自己的能力。

### 三、真实页面运行时

前端注册表是构建时生成的静态映射：

```ts
pageKey -> {
  definition,
  compiledAdapter,
  staticallyImportedComponents
}
```

页面适配器包含：

- 查询参数构造；
- API 请求和变更；
- 字段值编解码；
- 枚举选项；
- 展示格式化；
- 表单校验；
- 动作回调；
- 静态组件映射。

这些是编译代码，不进入数据库，也不通过有效层接口下发。

真实页面只调用一次共享 Hook，服务端根据当前认证主体选择有效层：

```text
编译默认值
  < 已发布应用层
  < 已发布当前角色层
  < 已发布当前用户层
  < 可信前端权限交集
```

浏览器不能指定另一个用户或角色，草稿不会进入页面。未来即使支持多角色，角色选择顺序也必须由
服务端认证策略决定，浏览器不能自行合并无序角色集合。

展示读取不是业务页面依赖：

- 页面外壳和业务数据可并行加载；
- 有效层请求必须有界；
- 超时、取消、网络错误、数据库回退、格式错误、定义漂移、未知字段、组件不兼容、必填字段被隐藏
  或恢复模式都会回退；
- 一个无效层整层丢弃，低层和编译默认值继续可用；
- 无配置时的页面行为必须与接入前一致。

允许改变的是展示模型，不允许改变业务适配器。比如将 Supplier 的 `creditLevel` 列隐藏或改成已注册
的 `tag` 样式，不会改变列表 API；隐藏“删除”按钮也不会改变删除接口权限；配置不能把任意 URL
变成数据源。

条件的上下文按页面表面固定。首个生产合同不允许列表列和 toolbar 动作使用条件，因为它们没有唯一
记录上下文；搜索字段读取当前过滤草稿；表单字段和 form 动作读取表单草稿；详情字段和 detail 动作
读取已加载记录；row 动作读取当前行。只能读取编译 adapter 为该上下文显式开放的字段。missing 与
`null` 不同，属性存在且值为 `null` 时 `exists=true`；missing 上的比较为 false；相等和集合判断不做
类型转换；有序比较只允许已登记的 number/date/date-time。条件只影响渲染，不删除查询/提交值、不改
handler，也不授予权限。

### 四、运行控制与恢复

注册能力与真正应用配置必须分开。启动配置增加：

```yaml
presentation:
  recoveryMode: false
  adoptionMode: disabled # disabled | shadow | active
  activePages: []        # 精确 pageKey 白名单
```

三种采用模式和恢复优先级：

| 状态 | 治理能力 | 后台解析 | 业务页面 |
| --- | --- | --- | --- |
| `disabled` | 可看能力、写草稿、发布 | 返回关闭诊断 | 仅编译默认值 |
| `shadow` | 同上 | 读取并校验已发布层，生成对比指标 | 仍仅编译默认值 |
| `active`，页面不在白名单 | 同上 | 返回未授权采用诊断 | 仅编译默认值 |
| `active`，页面在白名单 | 同上 | 返回有效层或回退诊断 | 应用合法展示配置 |
| `recoveryMode: true` | 控制台、历史和诊断仍可访问 | 强制空层 | 所有页面仅编译默认值 |

`recoveryMode` 的优先级最高。数据库文档不能关闭恢复模式，也不能将自己加入白名单。

Shadow 模式只解析展示结构，不执行另一份查询，不触发动作，不做隐藏业务写入。它用于提前发现：

- 定义哈希过期；
- 字段或动作引用不合法；
- 解析后字段/动作数量异常；
- 后端与前端定义不一致；
- 有效层接口错误和回退比例。

日志、指标和审计只记录页面键、范围、哈希、模式和稳定结果码，不记录文案、条件值、用户/角色原值、
业务记录、密钥或原始幂等键。

## 可视化编辑器

P3 不创建第二套产品，而是在现有“页面展示配置”治理控制台增加可视化模式。

建议布局：

```text
┌──────────────── 页面 / 范围 / 主体 / 继承状态 ────────────────┐
├───────────┬─────────────────────────┬───────────────────────┤
│ 能力与表面 │ 画布 / 合成数据预览      │ 属性检查器             │
│ 列表       │ 拖拽顺序                 │ 继承/显示/组件/宽度     │
│ 搜索       │ 可见性与布局             │ 文案/排序/条件/确认     │
│ 表单       │                         │                       │
│ 详情       │                         │                       │
│ 动作       │                         │                       │
└───────────┴─────────────────────────┴───────────────────────┘
      可视化模式  <------ 同一个类型化 AST ------>  原始 JSON 模式
```

支持的操作：

- 拖动排序；
- 显示或隐藏；
- 恢复继承；
- 修改中英文纯文本；
- 在字段允许清单中切换组件；
- 设置列宽、栅格和布局列数；
- 设置表格密度、分页大小和默认排序；
- 设置动作位置、顺序、显示和确认文案；
- 使用字段类型允许的条件运算符构建有界条件；
- 查看属性来自编译默认值、应用、角色还是用户。

可视化和 JSON 必须共享同一个类型化 AST：

- 省略表示继承，不能擅自补默认值；
- `false` 是明确值，不能被当成未设置；
- 条件树不能在切换模式时丢失；
- 保存、冲突、刷新和预览不能改变文档语义；
- Canonical JSON 仍通过现有 P1 API 保存和发布。

生成 adapter 在类型上拆成纯 `PresentationViewAdapter` 和有副作用的 `BusinessPageAdapter`。前者只含
静态组件、格式化、codec、校验和合成预览绑定；后者才含 query、mutation 和动作回调。治理预览只能
导入纯 view adapter，构建测试必须拒绝其依赖图出现网络 client、查询、变更或动作回调。

预览默认使用按字段类型和枚举生成的合成数据和纯 view adapter，不为了预览读取生产业务记录，也不执行任何动作。
结构安全但语义错误的草稿可以保存继续修复，发布仍必须通过当前定义校验。

现有四项权限保持独立：

| 权限 | 可做的事 |
| --- | --- |
| `presentation:read` | 看能力、配置、校验和历史 |
| `presentation:draft-write` | 编辑和保存草稿 |
| `presentation:publish` | 发布当前有效草稿 |
| `presentation:rollback` | 将历史重新发布为新版本 |

展示权限不会自动授予用户目录或角色目录读取能力。

## 定义漂移和升级

任何标准化能力变化都会产生新哈希。旧草稿和旧发布版本因此变为 stale。即使变化看起来只是新增字段，
第一版合同仍选择保守回退，不静默猜测兼容性。

运行时固定行为：

1. 丢弃整个过期层；
2. 保留低层；
3. 最终使用编译默认值；
4. 输出有界诊断；
5. 不修改数据库草稿和不可变历史。

治理控制台需要明确区分：

- 当前；
- 定义过期；
- 页面已不再注册；
- 文档损坏；
- 语义错误；
- 采用关闭；
- Shadow；
- Active；
- Recovery。

“迁移到当前定义”只能生成未发布草稿。版本 `2` 只展示保留、新增、移除、变更和不兼容项，不根据
名称相似度猜测重命名；以后只有增加显式、源码受控的 rename hint 合同后才可报告“重命名”。它不能
只改哈希，不能静默删除未知引用，更不能自动发布。

`mss upgrade admin` 的计划阶段根据旧、新生成快照报告：

- 页面键和定义哈希变化；
- 字段、动作、组件、表面和默认值变化；
- 后端和前端注册表是否一致；
- 哪些页面键可能导致持久化配置过期。

升级工具不连接生产数据库，因此不能声称影响了多少真实配置。实际存储状态由治理控制台判断。

Thin Host 升级继续使用受管理快照和三方比较：

- 只改生成文件和受管理文件；
- 保留未知和业务自有文件；
- Go 与 npm 作为同一个 Admin Distribution 版本升级；
- 不允许后端和前端展示定义混用不同版本；
- 持久化配置和历史不删除，是否可应用由新哈希判断。

## Supplier 首个生产试点

Supplier 保留稳定页面键 `supplier.list`。当前手写 P0 原型只证明过 resolver，不是生产等价默认值；
它会被版本 `2` 生成器产物替代，源头移动到 `.mss/modules/example-supplier.yaml`。生成默认必须匹配
当前页面：large 密度、20 条分页且选项为 `[20, 50, 100]`、初始不排序、搜索始终展开并包含关键词/
国家或地区/信用等级/三态启用状态、表单和详情单列、详情包含只读 ID 与创建/更新时间、toolbar 先导出
后新建。关键词展示字段由编译 adapter 固定执行 `code -> q` 绑定，profile 永远看不到传输参数 `q`。

接入不能改变已有：

- 路由；
- API；
- 查询参数；
- CRUD；
- 导出；
- 表单校验；
- 权限；
- 中英文文案合同；
- 加载、空、错误、无权限和冲突状态。

验收顺序：

1. **Disabled：** 能力已生成并注册，页面仍完全使用当前默认代码。
2. **Shadow：** 发布的应用/角色/用户层被解析和观测，页面仍显示默认值。
3. **Active 白名单：** 只有 `supplier.list` 应用配置。
4. **正常矩阵：** 列表、搜索、分页、排序、表单、详情、动作、导出、条件和多语言。
5. **异常矩阵：** 无配置、错误草稿、旧哈希、未知引用、数据库异常、请求超时和恢复模式。
6. **权限矩阵：** 前端按钮与后端直接 API 正负权限。
7. **Thin Host：** 外部生成、构建、启动、升级、幂等和自有文件保留。

Supplier 的证据不能自动授权其他页面。维护者已经明确授权 `user.list` 作为下一个隔离试点，但它必须
独立完成自己的合同、生成、权限边界和内置浏览器验收。

## 用户管理 Foundation 核心页试点

`user.list` 是当前开发分支获准接入的第二个页面，也是本阶段唯一允许配置的手写 Foundation 核心页。
“核心页”只表示所有权，不表示可以绕过生成：`.mss/core-pages/user-list.yaml` 是唯一源头，生成器必须
从它产出一致的 Go 定义、TypeScript 定义、规范定义哈希、标准化 manifest 和两端注册表条目。不得为
现有用户管理伪造 `AdminModule`，也不得分别手写 Go/TypeScript 能力定义。

首版只开放以下展示面：

- 中英文页面标题；
- 生成定义明确列出的安全列表展示字段；
- 仅 `name` 与 `status` 两个搜索项；
- 表格密度；
- 分页大小；
- `maxSortFields: 0`，因此编译默认值与 profile 都不能开启排序。

`status` 必须由后端执行精确过滤，并且值域由编译合同封闭。它不是只在浏览器中过滤当前页，也不能由
profile 指定查询参数名、编码方式、模糊匹配或任意新搜索字段。

以下内容从核心页 manifest、合成预览、有效层与可视化编辑器中结构性排除：

- 密码、确认密码与任何凭证；
- OAuth 信息、token 和 session；
- 角色、部门、岗位及其他关系 ID；
- root、特权、所有者或等价授权标记；
- form 与 detail；
- 包括新建、编辑、删除、重置密码在内的全部 actions；
- 权限、路由、HTTP 方法、端点、传输参数和 API 选择。

现有用户路由、列表 API、权限判断、root-only 控制，以及当前用户与目标用户保护全部保持编译态并继续
作为唯一权威。展示配置可以隐藏或调整已批准显示字段、搜索项，不能暴露敏感事实、创建 mutation、
放宽 root 检查或改变 handler 正在保护的目标。

后端在业务模块之前显式注入生成的 Foundation 核心注册表并冻结组合结果；前端只使用静态导入的 core
registry。采用顺序必须按页面隔离执行：先 `disabled`，再进入不改变渲染且不产生隐藏业务操作的独立
`shadow`，最后只有在自己的测试和内置浏览器证据完成后，才允许精确白名单中的 `user.list` 进入
`active`。`recoveryMode` 始终优先。Thin Host 只消费 Admin Go/npm 包内的核心能力，不复制源规格或产物。

当前只表示实现与验收正在进行。最终 Head 的生成器、后端、前端、安全、Thin Host、刷新、控制台和
失败网络请求检查尚未实际记录前，都不能写成已通过。

## 开发检查点

:::info
每个阶段都是独立、完整、可审查的检查点：先检查，完成后提交并立即推送，确认远端 SHA，再执行并
报告实际验证。推送成功不等于验证通过。禁止 rebase、force-push 或隐藏修复历史。
:::

| 阶段 | 开发内容 | 关键退出条件 |
| --- | --- | --- |
| D0 | 本文、FeatureSpec、ADR | 远端提交确认；规格校验；文档构建；维护者确认 |
| P2A | AdminModule Schema、语义校验、标准化清单、哈希、Go/TS 生成 | Schema、golden、跨语言一致、清理、路径限制、两次生成零差异 |
| P2B | 后端模块原子注册、应用级注册表、采用配置、有效层诊断 | 后端、权限、并发、异常、恢复测试 |
| P2C | 前端静态注册表、共享 Hook/解析/渲染、Supplier 适配、Disabled/Shadow | lint、tsc、单测、集成、默认行为一致 |
| P2D | Supplier Active 白名单 | 内置浏览器、API 权限、漂移、故障、恢复、Thin Host 全矩阵 |
| P3 | 可视化编辑器 | AST 无损往返、冲突、发布、历史、回滚、浏览器验收 |
| P4 | 先接入单独审查并生成的 `user.list` Foundation 核心页，再逐页接入其他生成业务页 | 核心单源生成/哈希/注册表一致；敏感面排除；status 精确过滤；独立 disabled/shadow/active 浏览器证据；每个后续页面另行验收 |

## 验收原则

完整功能最终必须证明：

- Go/TypeScript 页面键、清单和哈希完全一致；
- 无配置和所有失败路径均保持编译默认行为；
- 配置无法新增 API、动作、权限或组件；
- 角色和用户来源于服务端当前主体；
- 必填表单字段不能被隐藏；
- 无权限动作无法通过直接 API 调用；
- 视觉编辑与 JSON 无损往返；
- 定义漂移不会自动改写或自动发布；
- Recovery 能在数据库配置错误时恢复全部页面；
- Thin Host 升级不覆盖业务自有代码；
- `user.list` 只来自 `.mss/core-pages/user-list.yaml`，两端投影和哈希一致，Thin Host 不复制核心产物；
- 用户页 status 由后端精确过滤，敏感字段、form/detail/actions、权限/route/API 不进入展示合同；
- 用户页 root 与目标用户保护保持编译态，disabled、独立 shadow、精确 active 白名单和 Recovery 顺序有证据；
- 所有命令、测试和浏览器证据基于精确远端 Head；
- 未执行的验证必须明确标注，不能把设计、编写、推送和验证混为一谈。

## 当前实现与证据边界

D0 曾经只落盘设计；该历史检查点已经结束。当前开发分支正在实现并验收生成器、运行时、可视化编辑器
以及 `user.list` 核心页试点。本文描述目标合同，不等于实现已经完成。

每项证据必须写明实际执行的命令、结果和精确 Head。内置浏览器证据必须真实访问运行中的页面、刷新、
检查可见内容、控制台和失败网络请求，并保存关键截图；Thin Host 必须证明消费包内核心能力而不是复制。
若最终 Head 变化，之前的证据不能用于 PASS。本文更新本身不创建 PR、不合并 `main`、不打标签或发布
制品。

## 设计证据

- 机器合同：`.mss/features/admin-presentation-complete-design.yaml`
- 完整 ADR：`docs/adr/2026-08-28-admin-presentation-complete-design.md`
- P0 ADR：`docs/adr/2026-08-24-governed-admin-presentation-configuration.md`
- P1 ADR：`docs/adr/2026-08-24-admin-presentation-publication-workflow.md`
- 现有发布治理说明：`docs/docs/admin/presentation-configuration.md`

当前下一项是完成 `user.list` 单源生成、后端精确 status 过滤、前后端静态组合、安全排除测试和独立
disabled/shadow/active 内置浏览器验收；在这些证据完成前，不能宣称该第二页面已经验收通过。
