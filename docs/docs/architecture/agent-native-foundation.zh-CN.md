---
title: Agent-native Foundation
order: 1
description: v1.3.5 人机协作、机器合同、确定性工具与运行时产品的整体架构
---

# Agent-native Foundation

:::warning
v1.3.5 是不可变部分发布，Root 工具、官方 npmjs、Docs 与完整 Thin Host 路径未发布；
当前稳定版本仍是 v1.3.2。本页描述 Foundation 源码架构和未来完整发行合同，不是
v1.3.5 采用指引。
:::

## 目标

Foundation 让人和 Agent 在同一套可审查合同上开发管理系统。Agent 可以发现能力、
规划、生成、验证和升级，但不能绕过安全边界、文件所有权或发布治理。

## 四个平面

```text
意图平面        AGENTS.md、Feature、AdminModule、ADR
合同平面        .mss/project、capabilities、commands、lock、schemas
执行平面        mss、mss-mcp、Skills、generator、verifier、upgrader
产品平面        Thin Host -> Admin + Admin Web -> mss-boot
```

### 意图平面

人或 Agent 把中大型变化写成 Feature，把垂直业务写成 AdminModule。规格定义目标、
非目标、需求、约束、验收、风险、迁移和回滚。ADR 记录长期架构选择及状态。

### 合同平面

`.mss/` 提供机器可执行事实：

- `project.yaml`：版本、布局、工具链和组件；
- `capabilities.yaml`：已有能力与成熟度；
- `commands.yaml`：工作目录、参数和验证命令；
- `lock.yaml`：Distribution、Blueprint 与升级快照；
- schemas：规格结构和生成约束。

散落在文档中的版本或命令不得与这些合同冲突。

### 执行平面

源码执行平面包含 CLI、MCP 适配、Skills、generator、verifier 和 upgrader。未来完整
Root Release 只把 `mss` 与 `mss-mcp` 作为公共工具；二者复用相同实现和权限边界，
写操作默认 dry-run、路径受限、拒绝未知覆盖、输出稳定且不发送遥测。v1.3.5 没有发布
这些公共工具，源码能力不能替代 Release 二进制。

### 产品平面

```text
业务后端 ──显式编译期模块──> 完整 Admin ──依赖──> mss-boot
业务前端 ──菜单与路由注册──> 完整 Admin Web
```

Agent 工具不进入 Admin 运行时。Framework 只提供领域无关基础设施；Admin 统一拥有
身份、授权、迁移、配置和应用壳；Thin Host 只拥有组合和业务。

## 未来 package-first 采用路径

```text
完整 Root Release 工具
        │ 内置同源 Blueprint
        ▼
空目录 ──生成──> Thin Host
        │
        ├── Go: 同版本 Admin / Framework
        └── npm: 同版本 Admin Web
```

未来采用者不需要 Foundation checkout、`go.work` 或本地 `replace`。公共包资格必须
在空目录、`GOWORK=off` 和匿名 npmjs 安装条件下证明。v1.3.5 缺少这条完整证据链。

## 变更闭环

1. 读取最近 `AGENTS.md` 和 `.mss` 上下文；
2. 查找已有能力；
3. 更新规格；
4. dry-run 并审查变更；
5. 写入最小一致实现；
6. 运行 focused 检查，再按风险扩大；
7. 记录迁移、安全、兼容和未验证项；
8. 通过 PR 合入 `main`。

生成器必须支持 schema 验证、稳定排序、路径限制、golden 测试和两次运行幂等。

## 安全不变量

- 后端授权权威，UI 只做体验守卫；
- 状态变化不使用 GET；
- secret 不进入代码、日志、报告和生成物；
- 未知与业务所有文件不被升级器覆盖；
- 安装、生成和验证不接触生产凭据；
- 不引入浏览器运行时建模、远程代码加载或采用者追踪。

## 发布

协调版本按 Framework、Admin、Admin Web、Root、Docs、官方 npmjs 顺序发布。每个组件
必须来自同一个已合入 `main` 的干净提交，公开标签和摘要不可移动。发布后的修复使用
下一补丁版本。

外部 [mss-shop](/getting-started/mss-shop) 必须等待维护者显式选择的未使用完整版本，
再验证 package-first 路径和单租户业务扩展；它不能从 v1.3.5 或本地 Foundation 制品
生成。
