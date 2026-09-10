---
title: mss-shop 范本边界
order: 4
description: 以 v1.3.7 正式稳定版建立通用单租户商城范本的范围、所有权与验收合同
keywords: [mss-shop v1.3.7 stable single tenant reference r1shop]
---

# mss-shop 范本边界

[mss-boot-io/mss-shop](https://github.com/mss-boot-io/mss-shop) 应以当前稳定版 v1.3.7
作为新的可复现生成基线。v1.3.5 与 v1.3.6 是永久停止的不可变部分发布，不得用它们的
部分组件生成或补齐范本。Docs 网站通过独立 Tag 异步候补，不影响 v1.3.7 工具、包和
范本基线的有效性。

本页定义范本应满足的产品边界，不把 Foundation 源码工作区的成功当成 mss-shop 已经
生成、合并或部署的证明。最终资格必须来自 mss-shop 仓库自己的精确生成清单、业务提交、
CI 和浏览器证据。

## 建立可复现基线

在空目录安装 v1.3.7 Root Release 的 `mss` 与 `mss-mcp`，核对版本与源提交后，用
`mss new app` 先查看只读计划，再显式生成 `github.com/mss-boot-io/mss-shop`。未经业务
修改的生成结果应单独提交，后续商城能力以独立提交进入，从 Git 历史清楚区分 Blueprint
管理文件和业务自有文件。

生成基线必须固定 v1.3.7 Admin Distribution 与公开 npmjs 包，保留
`.mss/blueprint-manifest.json` 和冻结锁，并在仓库外解析依赖。不得使用 Foundation
checkout、本地 Go `replace`、本地 npm tarball 或历史候选命令替代公共发布身份。

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

商城手写页面的文案同时写入 `web/src/business/locales/zh-CN.ts` 与 `en-US.ts`。受管
locale facade 按 Admin core、AdminModule 生成词典、商城手写词典的顺序合并；业务文件
在三方升级中保持原字节。

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

## 验收完成定义

完整交付需要后端、前端、生成漂移、权限正反例、升级保留和外部使用方检查全部通过，
并用 Codex 内置浏览器验证桌面和窄屏关键流程、深链刷新、权限拒绝、网络错误与控制台
零异常。验证报告必须绑定 mss-shop 精确提交和 v1.3.7 Distribution 身份；Docs 网站部署
状态只作为文档自己的发布证据，不得阻断或替代这些检查。
