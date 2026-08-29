# mss-boot Framework

[English](./README.md)

## v1.3.7 候选组件状态

v1.3.7 已选为完整 package-first 候选，但
`github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.7` 尚未公开。候选 preview
可以从精确仓库 workspace 资格验证它；正式 Admin 发布随后必须用 `GOWORK=off` 解析
同一公共 Framework。

v1.3.5 与 v1.3.6 都是不可变部分发布。v1.3.6 Framework 组件已公开并绑定
`b1fe47a3a83209574e09d53526b122dd2cbc5277`，但协调列车缺少 Root 镜像、官方 npmjs
包、Docs 和完整 Thin Host 资格。

发布策略仍将 **v1.3.2** 定义为当前稳定发行版。部分列车的公共 Go 组件只能作为不可变
审计证据，不能与其他补丁版本、本地替换、源码检出或未发布包混合，拼成
完整 Admin Distribution。

## Framework 边界

mss-boot 提供生命周期、配置、日志、缓存、队列、锁、存储、传输、迁移、响应、条件、
重试、幂等与协调等基础能力；不包含 Admin 实体、菜单、业务流程、React 代码或 Agent
编排。

v1.3.7 候选的受支持依赖方向是：

```text
Thin Host 业务代码 -> Admin -> mss-boot
```

大多数应用应依赖完整 Admin Module，并让 Go 传递解析匹配的 Framework。只有领域无关
基础设施扩展才直接依赖 Framework。这是正在资格验证的完整发行合同，不是公共 v1.3.7
或部分列车 Thin Host 安装路径。

公开 API、配置键、接口和持久化行为仍是兼容性表面。参见
[包发布状态](../docs/docs/getting-started/packages.md) 与
[`CHANGELOG.md`](./CHANGELOG.md)。

仓库源码测试命令是 source-only 贡献者合同，保留在 [`AGENTS.md`](./AGENTS.md)。
