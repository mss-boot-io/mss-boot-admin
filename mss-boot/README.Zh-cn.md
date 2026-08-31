# mss-boot Framework

[English](./README.md)

## v1.3.7 稳定组件状态

v1.3.7 已是当前稳定且可采用的完整 Admin Distribution。公开 Framework Module
`github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.7` 已使用 `GOWORK=off`，并与
匹配的 Admin、Admin Web、Root 工具和镜像一起从精确 merged-main 提交
`77b53d41092741eac62fa6418c0bdbf87413c7cd` 完成资格验证。

v1.3.5 与 v1.3.6 都是不可变部分发布。v1.3.6 Framework 组件已公开并绑定
`b1fe47a3a83209574e09d53526b122dd2cbc5277`，但协调列车从未完成。部分列车的公共 Go
组件只能作为不可变审计证据，不能与其他补丁版本、本地替换、源码检出或未发布包混合，
拼成完整 Admin Distribution。Docs 网站独立发布，不影响此 Framework Module 的稳定状态。

## Framework 边界

mss-boot 提供生命周期、配置、日志、缓存、队列、锁、存储、传输、迁移、响应、条件、
重试、幂等与协调等基础能力；不包含 Admin 实体、菜单、业务流程、React 代码或 Agent
编排。

v1.3.7 的受支持依赖方向是：

```text
Thin Host 业务代码 -> Admin -> mss-boot
```

大多数应用应依赖完整 Admin Module，并让 Go 传递解析匹配的 Framework。只有领域无关
基础设施扩展才直接依赖 Framework。Thin Host 必须锁定完整 v1.3.7 发行版，不能从
v1.3.5 或 v1.3.6 组件拼装安装路径。

公开 API、配置键、接口和持久化行为仍是兼容性表面。参见
[包发布状态](../docs/docs/getting-started/packages.md) 与
[`CHANGELOG.md`](./CHANGELOG.md)。

仓库源码测试命令是 source-only 贡献者合同，保留在 [`AGENTS.md`](./AGENTS.md)。
