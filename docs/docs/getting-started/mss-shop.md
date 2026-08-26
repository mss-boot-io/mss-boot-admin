---
title: mss-shop 最佳范本
order: 4
description: 使用公开 v1.3.5 工具和包构建通用单租户商城管理系统的参考边界
keywords: [mss-shop v1.3.5 single tenant reference r1shop]
---

# mss-shop 最佳范本

[mss-boot-io/mss-shop](https://github.com/mss-boot-io/mss-shop) 是计划在 v1.3.5
公开对账完成后创建的外部参考应用。完成状态只由该仓库的生成基线、业务提交和 CI
证明；本页不把尚未执行的发布后工作描述成既成事实。

## 可复现基线

公开 v1.3.5 完成对账后，基线从空目录生成：

```sh
mss new app mss-shop --module github.com/mss-boot-io/mss-shop --destination ./mss-shop --write --git-init
```

生成结果未经业务修改先单独提交。后续商城能力以独立提交进入，便于审查哪些文件来自
Blueprint、哪些属于业务。

## 复杂业务的扩展接缝

mss-shop 的关系模型、十进制定价、库存事务和订单状态机不会伪装成普通 CRUD 生成能力。
每个复杂边界用 `.mss/features/` 记录规格，并以手写 `business.Module` 实现；唯一后端入口是
`internal/modules/custom/modules.go`。该注册表显式 import 并排序模块，Foundation 组合根再把
它与 `.mss/modules/` 的生成模块合并。禁止包初始化发现、运行时装载和 `AutoMigrate`。
Admin 的受保护路由组负责会话认证而不推断商城权限；每个手写 handler 必须使用注入的
principal 与数据库执行后端 permission 检查，并用正、负授权用例证明允许与拒绝路径。

业务页面放在 `web/src/business/`，由 `routes.config.ts` 提供页面路由，由
`route-registrations.ts` 提供菜单、服务端路径和权限元数据。两者都通过 Thin Host 的固定
组合胶水与生成结果合并；重复页面路径或服务端路径会在构建或启动时失败，不能静默覆盖。
这些业务自有文件在 Admin Distribution 三方升级中保持原字节，业务代码无需修改生成入口。

商城手写页面的文案同时写入 `web/src/business/locales/zh-CN.ts` 与 `en-US.ts`。受管
locale facade 按 Admin core → AdminModule 生成词典 → 商城手写词典合并，两份业务词典与
上述三条注册接缝一样由三方升级保留；禁止直接修改受管或生成 locale 文件。

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
- 金额在数据库使用 `DECIMAL(19,4)`，API 使用十进制字符串，浏览器提交的合计不作为权威值；
- 库存预留、释放、实扣和不可变流水在同一事务中完成，并使用版本和幂等键防止超卖与重复执行；
- 前端覆盖加载、空、错误、拒绝和移动布局；
- 中英文业务文案同步；
- 生产密钥和环境地址不进入仓库。

如果未来需要多租户，应通过新的规格、迁移与权限模型扩展，不能在查询层临时加
`tenant_id` 假装完成隔离。

## 验收

```sh
mss doctor --strict
mss setup
mss verify --all
```

全新范本第一次交互式 `mss setup` 会安全提示初始管理员密码；非交互自动化必须只
通过一次性 `MSS_ADMIN_INITIAL_PASSWORD` 环境变量提供。首次迁移完成后不再需要。

除自动检查外，完整交付还需要 Codex 内置浏览器验证桌面和窄屏关键流程、深链刷新、
权限拒绝、网络错误以及控制台零异常。仓库中的实际提交和 CI 是完成状态的权威来源。
