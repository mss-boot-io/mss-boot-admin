# mss-boot Framework

[English](./README.md)

`github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.4` 是完整 Admin 发行版中
可复用、领域无关的 Go Framework。

```sh
go get github.com/mss-boot-io/mss-boot-admin/mss-boot@v1.3.4
```

它提供生命周期、配置、日志、缓存、队列、锁、存储、传输、迁移、响应、条件、重试、
幂等与协调等基础能力；不包含 Admin 实体、菜单、业务流程、React 代码或 Agent 编排。

受支持的依赖方向是：

```text
Thin Host 业务代码 -> Admin -> mss-boot
```

大多数应用应导入完整 Admin Module，并让 Go 传递解析匹配的 Framework。只有在扩展
领域无关基础设施时才直接导入 `mss-boot`，并精确固定 v1.3.4。

公开 API、配置键、接口和持久化行为都是兼容性表面。参见
[包合同](../docs/docs/getting-started/packages.md) 与
[`CHANGELOG.md`](./CHANGELOG.md)。

仓库源码测试命令仅面向贡献者，保留在 [`AGENTS.md`](./AGENTS.md)。
