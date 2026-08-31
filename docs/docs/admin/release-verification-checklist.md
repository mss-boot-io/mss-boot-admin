---
title: Admin 发布与部署验证清单
order: 28
nav:
  order: 1
  title: admin
description: 完整 Thin Host 业务发行前后的最小验证清单
keywords: [admin release checklist smoke regression]
---

## 使用方式

:::warning
v1.3.5 与 v1.3.6 都已永久停止为不可变部分发布；本清单不能授权补发其缺失制品。
v1.3.7 是尚未完成 Distribution 公共对账的候选，本清单也不能单独把它变成可安装或可升级版本。
当前稳定与回退资料仍以 [v1.3.2 稳定记录](/releases/archive/v1-3-2) 为准。
Docs 是异步、非阻断的网站发布；`docs/v*` 只标识网站部署，其缺失或失败不影响组件、npm、
GitHub Latest、current stable 或采用状态。
:::

本清单区分未来完整版本或业务 Thin Host 的代码门禁与生产部署巡检。Foundation 发布方先
在 PR Head 上完成 `mss verify --changed`、影响计划选中的聚焦检查与受影响页面的内置浏览器验收，
并通过精简后的 PR 必需检查；合并后，只在精确且 tracked-clean 的 merged-main commit 上执行一次
`mss verify --all --release-evidence --expect-commit <full-sha>` 并固定自绑定完整报告。唯一 Root preview 只生成和核验
将被发布的归档、包、SBOM、校验和、portable delivery、Thin Host 组成与多架构镜像，再依次发布组件 Tag；
Root Tag 触发 Root Release 与后端镜像，单独受审的 stable promotion 从该精确 Root Tag 发布
官方 npm 并随后推进 GitHub Latest。Docs 从独立文档 Tag 候补发布且不加入上述依赖图。正式
Distribution Tag 不重复昂贵验证，也不接受 promotion、readiness run ID 或人工环境审批。

## 一、发布前检查

### 构建检查

- [ ] PR Head 执行 `mss verify --changed`、影响计划选中的聚焦检查与受影响页面内置浏览器验收；PR 阶段不重复完整发布资格套件
- [ ] Foundation 根目录在精确 merged-main commit 上执行 `mss verify --all --release-evidence --expect-commit <full-sha>` 成功，报告中的 commit、trackedCleanBefore 和 trackedCleanAfter 均精确
- [ ] 保存真实外部 Thin Host 检查输出的系统临时证据目录路径，并核验其中脱敏的 `evidence-manifest.json`；需要长期留存时记录目录归档哈希
- [ ] 完整验证包含 Agent 构建、Admin/Framework race、coverage、vet 与模块元数据、独立 next-Foundation 生成和升级、CLI/MCP/doctor 身份一致性、确定性冲突、第二次升级零变更、eval、依赖审计、release workflow 合同、delivery smoke 与 Playwright E2E
- [ ] PR、审查记录或仓库外发布台账记录完整 commit、`trackedCleanBefore: true`、`trackedCleanAfter: true`、精确命令、verify report 哈希或摘要及退出结果；本地报告单独存在不授权发布
- [ ] Codex 内置浏览器在同一精确 commit 完成桌面、窄屏、深链、刷新、控制台和失败网络请求验收
- [ ] PR 必需的治理、govulncheck、CodeQL 和普通 Admin 单测全部通过；Framework 变更另跑一个普通独立单测；race、coverage、vet/tidy、前端/Thin Host/容器/多数据库重型矩阵不作为 PR 合并门禁
- [ ] 合并后的 Root preview 仅核验同一 frozen commit 的精确发布制品，且六平台二进制、portable delivery、候选 Thin Host、SBOM、校验和和双架构 OCI 均完整
- [ ] Admin Tag 流水线在 Framework 已公开后，以 GOWORK=off 和公共 Go Proxy 解析精确 Framework 并通过 app/business 组合测试
- [ ] 后端和 Admin Web 依赖都精确为同一个已完成公共对账的协调版本，锁文件无漂移
- [ ] 面向 Distribution 使用者的源文档如有更新，版本与采用状态不误导；公开 Docs 网站部署状态单独记录，不作为本节通过条件

### 核心能力检查

- [ ] 登录成功且首次登录状态正常
- [ ] Welcome 页监控卡片与趋势图正常
- [ ] 日志页面能看到登录日志、审计日志、运行时日志
- [ ] 仅在显式 Local 开发 Delivery 中执行头像上传 smoke，并确认 `/public/uploads/{uuid}` 内容一致；该结果不作为生产 Provider 证据
- [ ] 当 Local/S3-compatible 仍为 Legacy / Blocked 时，生产 ingress 已阻断两个上传路径，且未授予通用 `storage:upload` 权限作为纵深防御；头像入口没有独立 Casbin permission
- [ ] 告警规则可以保存
- [ ] 已启用任务状态正常
- [ ] 国际化接口可返回有效语言资源

## 二、发布后冒烟检查

- [ ] `/healthz` 正常
- [ ] 前端首页可打开
- [ ] WebSocket 在线状态正常
- [ ] `/public/` 未被当作生产对象 Delivery 代理，两个上传入口仍被部署侧阻断
- [ ] 后端日志无持续性 fatal 错误

## 三、异常归属建议

| 现象 | 优先排查 |
|------|----------|
| 前端空白页 | 前端构建、代理、接口鉴权 |
| 监控图表无数据 | `/admin/api/monitor` 的 `collectedAt`/`stale`/`instanceId`、内置系统作业日志、登录态 |
| 头像入口被禁用 | 当前 Provider / Delivery evidence 状态、ingress 与权限门禁；不要通过临时开放 `/public/` 绕过 |
| 告警不发送 | 通知配置、规则状态、日志输出 |
| 用户任务未执行 | `task.enable`、cron 表达式、任务状态、TaskRun |
| 内置监控/会话清理未执行 | task server 启动日志和系统作业错误；不要以 Task/TaskRun 无记录判定失败 |

## 三-A、Docs 网站异步候补（不阻断发布）

- [ ] 仅在准备发布网站时检查 `docs/v*` 身份；该 Tag 只表示网站发布，不是 Distribution Tag
- [ ] 初始 Docs Tag 精确绑定已存在的 Root Release commit；后续修订保持 Root 祖先关系并使用最低未占用 `+docs.N`
- [ ] Docs 源来自受审 merged-main，受保护部署、不可变 Release、归档校验和和 `release.json` 身份一致
- [ ] 在内置浏览器刷新首页与嵌套路由，检查可见版本、控制台和失败网络请求
- [ ] 若任一项失败，记录“网站部署待办”并只修复 Docs；不得阻止或回退 Framework、Admin、Admin Web、Root、镜像、npm、GitHub Latest、`currentStableVersion` 或采用路径

## 四、回滚前确认

- [ ] 问题是否来自配置而非代码
- [ ] 数据库结构是否发生变化
- [ ] 若开发环境曾显式启用 Local，已有 opaque-key 对象是否已保留；不要回滚到解析后限流或 user/filename key
- [ ] 日志目录是否需要保留
- [ ] 通知渠道凭据是否改动

## 推荐阅读

- [集成测试指南](/admin/integration-test-guide)
- [v1.3.5 不可变部分发布记录](/releases/v1-3-5)
- [v1.3.6 不可变部分发布记录](/releases/v1-3-6)
- [v1.3.7 发布候选说明](/releases/v1-3-7)
- [当前采用状态](/getting-started)
