---
title: v1.3.7 候选页面展示配置发布治理
order: 16
nav:
  order: 1
  title: Admin
description: v1.3.7 候选页面展示配置的草稿、发布、回滚、权限与恢复合同
keywords: [v1.3.7 v1.3.5 v1.3.2 admin presentation configuration publish rollback recovery etag]
---

# 页面展示配置发布治理

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 与 v1.3.6 已永久停止并保持不可变部分发布；v1.3.7 已选
为 release candidate，但尚未稳定且不可采用。候选发布面可能处于不同公开阶段，必须以
远端发布台账为准；完整 stable promotion 和最终 policy/Docs 对账完成前，本页不是安装、创建、
升级或部署指引。
:::

页面展示配置把“已经编译并授权的页面能力”与日常展示选择分开。维护者可以准备列顺序、搜索项、
表单布局、详情布局、文案、密度和动作位置等数据，但不能通过配置创建字段、接口、路由、权限、
组件实现或业务模型。

:::warning
P1 只提供受治理的草稿、发布、历史、回滚、有效层读取和管理控制台。生产能力注册表目前故意为空，
也没有业务页面读取有效展示层，因此本阶段不会改变任何现有业务页面。Supplier 等真实页面要等 P2
通过生成器投影能力定义并完成单独验收后才能接入。
:::

## 安全模型

展示文档始终是不可信数据。服务端使用编译进程序的能力定义进行严格结构和语义校验，文档只能引用
已注册的页面、字段、数据源、组件和动作标识。配置中不能包含 JavaScript、HTML、SQL、URL、HTTP
方法、请求头、动态导入、权限或可执行模板。

每个页面最多使用三层已发布文档，顺序固定为：

```text
编译默认值 < 应用层 < 当前角色层 < 当前用户层 < 可信权限交集
```

角色和用户身份来自已认证的服务端主体，浏览器不能通过查询参数选择别人。角色与用户 ID 是服务端已有的
不透明标识，只按长度和精确值处理，不要求满足页面键等语义名称格式。草稿永远不会进入有效层。
如果定义哈希过期、文档损坏、语义不再兼容或存储暂时不可用，服务端会舍弃相应展示层并让页面使用
编译默认值；展示配置不是页面可用性的前置依赖。

## 权限分离

迁移创建四项互不隐含的权限：

| 权限 | 能力 |
| --- | --- |
| `presentation:read` | 查看能力、配置、校验结果和不可变历史 |
| `presentation:draft-write` | 创建或替换未生效草稿 |
| `presentation:publish` | 发布当前有效草稿 |
| `presentation:rollback` | 将历史版本重新发布为一个新版本 |

前端按钮会按权限隐藏或禁用，但后端仍对每个接口独立授权。只授予发布权限不会自动获得草稿写入或
回滚权限。

## 管理流程

1. 在“系统管理 → 页面展示配置”选择一个已注册页面和应用、角色或用户范围。
2. 编辑严格 JSON 草稿并执行校验。结构安全但语义无效的草稿可以保存用于继续修复，不会生效。
3. 校验通过后保存草稿，再经明确确认发布。服务端在事务内按当前能力定义重新校验。
4. 在历史区分页查看全部不可变发布记录。回滚会把所选历史内容校验后发布成新的递增版本，不会移动或覆盖旧记录。

控制台自动使用强 ETag 和幂等键。两个操作者基于同一版本保存时只能有一个成功；另一方会看到当前
服务端版本，本地编辑内容保持不变，必须明确选择丢弃并重新加载，系统不会静默覆盖或重试。冲突响应
只返回不透明聚合 ID、当前版本和 ETag；草稿、历史、页面/范围信息仍须通过具备读取权限的接口重新获取。
同一操作者的相同发布或回滚重试返回当次原始结果，不会被后来保存的草稿或版本替换；幂等键不能跨操作者复用。

控制台的配置列表与历史列表均使用服务端分页，不再只加载固定的第一页。选择已有配置时，预览能力会
跟随该配置自己的页面键。即使能力注册表为空，恢复模式提示仍会显示，已存配置和历史也仍可用于诊断；
此时只禁用依赖新能力定义的创建和预览操作。

有未发布草稿时不能回滚，以免恢复动作丢弃另一位操作者的工作。发布和回滚审计只记录有界、脱敏的
资源元数据；展示文档、文案、条件值、原始主体标识和原始幂等键不会进入通用审计正文。

## 部署与升级

Foundation 源码可以按仓库 `.mss/commands.yaml` 验证幂等迁移与完整拓扑；这属于
source-only 贡献者合同，不能作为 v1.3.5 Thin Host 的本地启动路径。

生产部署必须复用同一个不可变 Thin Host 后端镜像和配置：先运行 `migrate` init job，
再运行一次 `server -a` 路由同步 job，两者成功后才启动长期 `server`。全新数据库
仅在 migrate job 中由 secret store 注入一次性 `MSS_ADMIN_INITIAL_PASSWORD`；同步
和长期服务环境不得保留该值。迁移是前向兼容的，只新增展示配置聚合、不可变历史及其
权限/菜单元数据，不修改已有业务表和配置行。`server -a` 不能由数据库迁移代替。

部署后至少确认：

- 无 `presentation:read` 的非 root 用户访问控制台和管理接口均被拒绝；
- 四项权限可以独立分配；
- 控制台能显示明确的空能力状态，而不是空白页；
- 能力注册表为空时，恢复模式提示和已有配置/历史仍然可见；
- 配置与历史超过一页时可以翻页，选择不同页面的配置会使用对应能力预览；
- 刷新控制台路由仍能正常进入；
- 浏览器控制台无应用错误，管理请求无意外失败；
- 生产注册表仍为空时，所有业务页面与升级前显示一致。

## 紧急恢复模式

恢复开关只存在于启动配置，不存放在数据库，也没有 Admin 写接口。发生错误配置、定义漂移或存储事故时，
在每个实例的受控配置中设置：

```yaml
presentation:
  recoveryMode: true
```

然后滚动重启全部 Admin 实例。恢复模式下，有效展示读取返回空层和明确的 fallback/recovery 诊断，所有
业务页面继续使用编译默认值；数据库中的草稿、已发布版本和历史不会被删除。确认页面恢复后再调查或修复
配置。只有在修复、权限和有效层检查通过后，才把 `recoveryMode` 恢复为 `false` 并再次滚动重启。

不要通过删除历史表、回退版本指针、临时放宽校验或给控制台本身增加展示配置来“恢复”。

## 设计与机器契约

- [P0：受治理的页面展示配置 ADR](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/adr/2026-08-24-governed-admin-presentation-configuration.md)
- [P1：发布工作流 ADR](https://github.com/mss-boot-io/mss-boot-admin/blob/main/docs/adr/2026-08-24-admin-presentation-publication-workflow.md)
- Foundation 设计合同：[展示配置 Feature](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/features/admin-presentation-configuration.yaml)、[发布工作流 Feature](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/features/admin-presentation-publication-workflow.yaml)
- Foundation 可移植 Schema：[admin-page-presentation.schema.json](https://github.com/mss-boot-io/mss-boot-admin/blob/main/.mss/schemas/admin-page-presentation.schema.json)

这些链接是 Foundation 贡献者与发布流程的设计证据，Thin Host 采用者无需在本地复制
或维护它们。
