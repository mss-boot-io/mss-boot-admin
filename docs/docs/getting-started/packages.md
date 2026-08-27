---
title: v1.3.6 候选包发布状态
order: 2
description: v1.3.6 未发布候选包与 v1.3.5 永久停止组件边界
keywords: [v1.3.6 v1.3.5 v1.3.2 candidate go module npm admin web immutable partial]
---

# v1.3.6 候选包发布状态

:::warning
发布状态：v1.3.2 仍是当前稳定版；v1.3.5 已永久停止并保持不可变部分发布；v1.3.6 已选
为 release candidate，但 Go Module、Admin Web npmjs 与 Root 包面尚未发布。公共制品
对账前，v1.3.6 不可采用，也不开放安装或升级命令。
:::

## 已公开与缺失的身份

| 组件 | v1.3.5 身份 | 采用结论 |
| --- | --- | --- |
| Framework | `github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.5` | Go Module 已公开，但只代表 Framework 组件 |
| Admin | `github.com/mss-boot-io/mss-boot-admin/admin@v1.3.5` | Go Module 已公开，但只代表后端组件 |
| Admin Web | `@mss-boot-io/admin-web@1.3.5` | GitHub Release 与 GitHub Packages 已公开，npmjs 未发布 |
| Root | `v1.3.5` | 只有不可变 Tag；Root Release、工具和后端镜像未发布 |
| Docs | `docs/v1.3.5` | 未创建、未部署 |

这些身份可以用于审计已经公开的不可变组件，不能拼成受支持的 Thin Host。尤其不能把
GitHub Packages、Release 附件、本地 tarball、源码目录、分支或 `replace` 当作 npmjs 和
完整 Root 发布的替代品。

## 未来完整发行的导入边界

完整 Admin Distribution 仍遵循以下设计，但只有未来显式选定且完成公共对账的版本才能
提供可执行用法：

```text
业务后端模块 ──编译期注册──> Admin ──依赖──> mss-boot
业务前端路由 ──显式注册──> Admin Web 完整应用
```

- 普通应用只直接依赖 Admin；仅开发通用基础设施时才直接依赖 Framework；
- Admin Web 是一个完整前端发行单元，不拆出第二个 SPA；
- 下游必须使用公共 Go Module、官方 npmjs 包和冻结锁，不使用本地替换；
- 公共解析资格必须关闭 Foundation workspace，并在仓库外的干净使用方环境验证；
- 后端权限始终由服务端执行，前端菜单或控件隐藏不能替代授权。

## 业务扩展与升级所有权

未来 Thin Host 只拥有业务模块和组合胶水。普通 AdminModule 由确定性生成器负责；关系、
十进制定价、库存并发和状态机等复杂行为使用显式扩展接缝：

- `internal/modules/custom/modules.go` 注册手写 `business.Module`；
- `web/src/business/routes.config.ts` 与 `route-registrations.ts` 声明页面、菜单和权限；
- `web/src/business/locales/zh-CN.ts` 与 `en-US.ts` 同步业务文案；
- 受管组合层合并核心、生成和手写条目，并对重复路径失败关闭。

三方升级只能更新受管文件，必须原字节保留业务和未知文件。v1.3.5 没有完整公共工具、
npmjs 包和 Root 发布，因此不能用来创建、验证或升级这种 Thin Host。

完整证据见 [v1.3.5 不可变部分发布记录](/releases/v1-3-5)。
