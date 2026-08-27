---
title: 容器发布状态与部署合同
order: 6
description: v1.3.5 镜像部分发布事实，以及未来 Thin Host 的构建、配置、验证和回滚边界
---

# v1.3.5 容器发布状态与部署合同

当前稳定版本仍是 **v1.3.2**。v1.3.5 是不可变部分发布，镜像发布只完成了一部分：

| 镜像身份 | v1.3.5 结果 |
| --- | --- |
| `ghcr.io/mss-boot-io/mss-boot-admin:v1.3.5` | 后端 Root 镜像未发布 |
| `ghcr.io/mss-boot-io/mss-boot-admin-antd-v6:v1.3.5` | 前端镜像已发布并保持不可变 |

单个前端镜像不能证明完整 Admin Distribution。即使两个参考镜像未来都存在，它们也只
是 Foundation 发行证据，不包含采用者的业务模块、组合入口或业务前端路由，Thin Host
不能直接部署它们来代替自己的业务镜像。

## 构建 Thin Host 镜像

未来完整发行生成的 Dockerfile 从同一协调版本的公共依赖和冻结锁构建后端、Admin Web
与业务代码，并把 Go、Node 和最终运行时基础镜像固定到已核验的多架构 digest。配套
`.dockerignore` 会排除 Git 元数据、Agent 指令、运行报告/日志、数据库、环境文件、配置
覆盖和前端构建缓存；不得删除这些边界把本地 secret 带入构建上下文。应在业务仓库自己
的 CI 中构建、测试并推送不可变应用镜像，例如：

```sh
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag ghcr.io/acme/orders-admin:2026.08.25-1 \
  --push .
```

发布时记录业务镜像的 OCI index digest、源提交和锁文件身份；部署配置使用 digest，
而不是只依赖可移动标签。基础镜像 digest 需要升级时，通过新的应用 PR 更新并重跑构建
与浏览器验收。

发布前不要把候选或部分发布身份当作可用镜像。生产部署应记录实际业务镜像的不可变
digest、源提交、锁文件和平台。多架构标签对应 OCI index；部署证据应区分 index digest
与节点实际平台 manifest。v1.3.5 后端镜像不存在，因此本页不提供 Root 镜像拉取或运行
命令，也不建议用前端镜像拼装不完整部署。

## 配置与数据

- 配置以只读文件或部署平台注入；
- 数据库、上传与日志使用持久卷；
- secret 不写入镜像、Compose 文件或命令历史；
- 前后端 origin、CORS、Cookie Secure 和反向代理信任边界保持一致；
- 迁移前完成备份和恢复演练。

## 先迁移，后启动服务

Thin Host 后端不会在 `server` 启动时偷偷修改数据库。部署编排必须先用同一个业务镜像
digest 运行一次 `migrate` init job，成功后才允许启动或滚动更新 `server`：

- init job 和 server 使用同一套生产配置提供方与数据库连接；
- 全新数据库只在 init job 中由 secret store 映射 `MSS_ADMIN_INITIAL_PASSWORD`；
- 该值不写入命令参数、镜像、Compose、日志或长期 server 环境；
- 已记录首次迁移后，后续幂等迁移不再需要该值；
- 迁移失败必须阻止服务发布，不能让 server 以缺表状态继续运行。

镜像的入口程序同时支持 `migrate` 与默认的 `server` 子命令，因此 Kubernetes init
container、一次性 Job 或等价部署阶段都可以复用同一制品。不要在容器启动命令中拼接
明文密码。

## 启动后验证

至少检查：

1. 后端 `/healthz` 与 `/readyz` 的状态和正文；
2. 登录、刷新、退出和权限拒绝；
3. 前端静态资源、深链刷新和 API 代理；
4. 一个真实只读与一个授权写业务流程；
5. 容器重启计数、错误日志和浏览器控制台。

仅有 `running` 或 HTTP 200 不足以证明业务成功。

## 回滚

应用修复优先使用后续补丁版本。需要回退时同时恢复匹配的镜像、配置、数据库快照和
业务数据；不要只换二进制跨越不兼容迁移。公共标签和 digest 不移动、不覆盖。
