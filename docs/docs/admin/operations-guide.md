---
title: v1.3.3 运行与运营指南
order: 19
nav:
  order: 1
  title: admin
description: Thin Host 的安全运行、变更、任务、通知、监控与故障处置入口
keywords: [v1.3.3 admin operations task notice monitoring rollback]
---

# v1.3.3 运行与运营指南

本文面向通过公开 v1.3.3 工具和包生成的 Thin Host。应用仓库只维护业务模块、组合胶水
和非敏感配置；不要复制 Foundation 源码、旧版配置文件或历史部署脚本。

## 日常操作入口

在应用仓库根目录使用已安装的 `mss`：

```sh
mss doctor --strict
mss dev status
mss verify --changed
```

`doctor` 检查工具、Distribution、锁文件和 Thin Host 合同；`dev status` 只报告由当前
项目管理的开发进程；`verify` 运行与改动范围相符的检查。进程存在或容器健康不等于业务
成功，仍应验证 `/healthz`、`/readyz`、登录、权限拒绝和关键写操作。

开发环境需要查看日志或停止进程时：

```sh
mss dev logs backend
mss dev logs admin-web --follow
mss dev stop
```

只停止属于当前 Thin Host 的进程。端口冲突时先确认监听进程身份，不要批量终止其他服务。

## 配置与密钥

仓库内配置只保存可以公开审查的默认值。环境差异和密钥由环境变量或部署平台 Secret
注入，不在 YAML、JSON、命令参数、迁移记录、日志或验证报告中写入明文值。

```yaml
runtime:
  resources:
    main:
      provider:
        kind: redis
        redis:
          credentials:
            kind: password
            password:
              passwordRef: env://MSS_RUNTIME_REDIS_PASSWORD
```

邮件、Webhook、OAuth、数据库和对象存储使用同一原则：仓库只保存开关、非敏感地址和
SecretRef。配置变更通过受保护的 Admin API、部署配置或可审查迁移完成；不要直接向业务
表执行临时写入，也不要复制退役的单体应用配置文件。

完整来源优先级和生产检查见[配置指南](/admin/configuration-guide)，最低安全要求见
[安全基线](/admin/security-baseline)。

## 初始化与迁移

全新应用第一次交互式初始化：

```sh
mss setup
```

工具通过隐藏输入询问初始管理员密码。非交互自动化只为该 setup 进程注入一次性
`MSS_ADMIN_INITIAL_PASSWORD`；不要把密码放在命令行。首次迁移成功后，重复执行 setup
不再需要初始密码。

本地首次登录打开 `http://127.0.0.1:8001`，使用用户名 `admin` 和本次 setup 提供的
密码；系统没有默认密码。

生产变更前必须：

1. 在空库验证完整迁移；
2. 从当前受支持版本验证升级路径；
3. 备份并实际演练恢复；
4. 验证唯一约束、外键、时间和 JSON 语义；
5. 把多写步骤放入显式事务；
6. 先完成代码和迁移回滚决策，再进入变更窗口。

不要依赖运行时 `AutoMigrate` 代替版本化迁移，也不要把手工数据库修改当作永久配置。

## 通知与会话

通知列表、详情、已读状态和 WebSocket 连接都以已验证会话为边界。请求中的用户字段不能
扩大可见范围；前端隐藏不替代后端授权。通知正文和其他敏感详情响应应禁止缓存，浏览器
关闭详情或编辑器后应清除对应查询缓存。

排查实时通知时依次确认：

1. 登录会话和一次性 WebSocket ticket 有效；
2. 反向代理允许升级连接并使用正确 origin；
3. 在线连接数和心跳变化正常；
4. 启用集群广播时 Redis 可用且日志已脱敏；
5. 跨用户读取和写入的拒绝测试仍通过。

长期访问令牌不得放入 URL、localStorage 或 WebSocket 查询参数。

## 用户任务与内置作业

系统区分两类调度对象：

| 类型 | 所有权 | 变更方式 |
| --- | --- | --- |
| 用户任务 | 业务配置，可启停并产生运行记录 | 通过受保护的 Task API 或对应 Admin 页面 |
| 内置系统作业 | Admin 运行时，不属于业务数据 | 随服务显式注册，用户 CRUD 不可覆盖 |

任务端点、请求体和元数据都按敏感配置处理。启用外部调用前限制目标地址、方法、超时、
并发和重试，防止 SSRF、重复写入和无限重放。失败时检查最近一次运行记录和脱敏错误，不要
通过直接修改任务表绕过 API 校验。

内置监控采样、会话清理等作业即使用户任务关闭也应继续运行；它们不会伪装成用户任务，
也不能被同名业务记录替换。

## 监控、日志与告警

最小观测面包括：

- HTTP 状态、延迟、认证与授权拒绝；
- 数据库、缓存、队列和外部集成的就绪状态；
- 用户任务执行与内置作业失败；
- WebSocket 在线连接和心跳；
- CPU、内存、Goroutine、GC 与磁盘；
- 迁移、配置和高风险管理操作的审计事件。

日志不得包含密码、Cookie、Authorization、token、完整 DSN、通知正文或敏感请求体。
指标端点和 pprof 只暴露在受控网络。告警通道默认关闭；启用时使用受限凭据的 SecretRef，
并验证失败不会终止无关功能。

详细排查顺序见[性能与可观测性指南](/admin/observability-guide)。

## 发布、升级与回滚

升级前先备份仓库、配置和数据库，安装目标版本的 `mss` 与 `mss-mcp`，并确认仓库保留
`.mss/blueprint-manifest.json`。手工拼装或丢失 manifest 的仓库必须先生成新基线再迁入
业务所有文件，不能直接三方升级。

先查看只读计划：

```sh
mss upgrade admin v1.3.3
```

只有确认无冲突后才应用：

```sh
mss upgrade admin v1.3.3 --apply --yes
mss doctor --strict
mss verify --all
mss upgrade admin v1.3.3
```

升级只管理 Blueprint 声明的 Thin Host 文件，业务所有和未知文件必须保留。应用后验证
最后一次计划为空，并验证迁移、登录、菜单/API 绑定、权限允许与拒绝、关键业务写操作、
前端深链刷新和控制台。

回滚不能移动已发布标签或覆盖制品。恢复时使用匹配版本的工具、Admin、Framework、
Admin Web、数据库备份和 Blueprint 快照；若已产生不可逆数据写入，停止自动回退并按迁移
方案处置。

## 事件处置顺序

1. 确认受影响应用、版本、提交、区域和时间窗口；
2. 先读取真实 API、日志、就绪和依赖证据，不先扩大负载；
3. 区分直接失败、触发条件、下游影响和仍未知项；
4. 只变更已确认属于该 Thin Host 的最小范围；
5. 复核业务合同，而不只看进程、端口或容器状态；
6. 记录执行过的命令、结果、跳过项和恢复路径。

如果需要源码级修复，按照贡献者流程通过 PR 合入 Foundation 或业务仓库；使用方运行手册
不应回退到 Foundation 源码内的工具入口或本地模块替换。
