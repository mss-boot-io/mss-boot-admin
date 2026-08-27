---
title: mss-shop 范本采用状态
order: 4
description: 通用单租户商城范本的设计边界，以及 v1.3.5 不可作为生成基线的原因
keywords: [mss-shop v1.3.5 v1.3.2 single tenant reference r1shop]
---

# mss-shop 范本采用状态

[mss-boot-io/mss-shop](https://github.com/mss-boot-io/mss-shop) 原计划在完整 Admin
Distribution 公开对账后创建。v1.3.5 已停止为不可变部分发布，缺少 Root 工具、官方
npmjs 包和 Docs，因此不能成为范本的生成、安装或升级基线。当前稳定版本仍是 v1.3.2，
但它也不是本轮 mss-shop 重构所声明的生成基线。

实施必须等待维护者显式选择一个未使用版本，并由该版本的完整公共证据证明工具、Go
Module、npmjs、镜像、Docs 与外部使用方路径全部可用。本页只保留业务和架构设计，不提供
候选命令。

## 可复现基线要求

未来范本应从空目录使用一个完整公开版本生成，并把未经业务修改的生成结果单独提交。
后续商城能力以独立提交进入，从历史中清楚区分 Blueprint 管理文件和业务自有文件。

资格证据必须来自 mss-shop 仓库自身的精确生成清单、业务提交与 CI；Foundation 源码、
本地工具、本地包或历史候选命令不能替代这条证据链。

## 复杂业务的扩展接缝

mss-shop 的关系模型、十进制定价、库存事务和订单状态机不会伪装成普通 CRUD 生成能力。
每个复杂边界用 `.mss/features/` 记录规格，并以手写 `business.Module` 实现；唯一后端
入口是 `internal/modules/custom/modules.go`。该注册表显式导入并排序模块，Foundation
组合根再把它与 `.mss/modules/` 的生成模块合并。禁止包初始化发现、运行时装载和
`AutoMigrate`。

Admin 的受保护路由组负责会话认证而不推断商城权限；每个手写 handler 必须使用注入的
principal 与数据库执行后端 permission 检查，并用正、负授权用例证明允许与拒绝路径。

业务页面放在 `web/src/business/`，由 `routes.config.ts` 提供页面路由，由
`route-registrations.ts` 提供菜单、服务端路径和权限元数据。两者都通过 Thin Host 的
固定组合层与生成结果合并；重复页面路径或服务端路径必须失败关闭，不能静默覆盖。

商城手写页面的文案同时写入 `web/src/business/locales/zh-CN.ts` 与 `en-US.ts`。
受管 locale facade 按 Admin core、AdminModule 生成词典、商城手写词典的顺序合并；业务
文件在三方升级中保持原字节。

## 参考来源与重构原则

mss-shop 会先盘点而后选择性重构以下项目的业务能力：

- [shop-r1/shop-go](https://github.com/shop-r1/shop-go)：后端领域与 API；
- [shop-r1/shop-admin-ui](https://github.com/shop-r1/shop-admin-ui)：后台交互；
- [shop-r1/shop-m-cli](https://github.com/shop-r1/shop-m-cli)：消费端能力边界。

它不是三仓库的机械合并。品牌、租户、部署、密钥、旧框架和环境耦合会被移除；认证、
授权、迁移、API、页面状态和升级遵守 MSS 合同。

## 单租户边界

- 一个部署服务一个业务组织；
- 数据模型不携带伪多租户分支或隐式租户过滤；
- 后端对每个状态变更执行权限检查；
- API 写操作不使用 GET，并明确事务、幂等和并发语义；
- 金额在数据库使用 `DECIMAL(19,4)`，API 使用十进制字符串，浏览器合计不作为权威值；
- 库存预留、释放、实扣和不可变流水在同一事务中完成，并防止超卖与重复执行；
- 前端覆盖加载、空、错误、拒绝和移动布局；
- 中英文业务文案同步；
- 生产密钥和环境地址不进入仓库。

如果未来需要多租户，应通过新的规格、迁移与权限模型扩展，不能在查询层临时添加
`tenant_id` 假装完成隔离。

## 未来验收边界

完整交付需要后端、前端、生成漂移、权限正反例、升级保留和外部使用方检查全部通过，
并用 Codex 内置浏览器验证桌面和窄屏关键流程、深链刷新、权限拒绝、网络错误与控制台
零异常。在完整发行版本被显式选择前，这些只是验收合同，不表示 mss-shop 已可创建。

当前版本状态见 [v1.3.5 不可变部分发布记录](/releases/v1-3-5)；稳定历史见
[v1.3.2 稳定记录](/releases/archive/v1-3-2)。
