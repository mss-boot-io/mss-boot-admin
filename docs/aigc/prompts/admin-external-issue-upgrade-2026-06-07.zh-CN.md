# mss-boot-admin 外部 Issue 升级记录

日期：2026-06-07

## 范围

本轮只评估 `mss-boot-io/mss-boot-admin` 中外部用户提交的 open issues：

- `#126` 添加查看在线用户、强退功能
- `#105` 数据更新后接口查询数据未刷新
- `#89` migration 支持纯 SQL 脚本和回滚
- `#75` Bug Report

排除 maintainer 自己创建的 issue、bot issue、依赖升级 PR。

## 结论

- `#105` 有足够信号做一期修复：外部用户反馈启用 `queryCache` 后更新数据仍读到旧详情。
  当前 admin 启动时没有把 mss-boot query cache 初始化回调传入 `Cache.Init`，
  也没有把更新/删除后的 table tag 清理函数绑定到 mss-boot GORM actions。
- `#126` 是合理功能方向，但普通登录 JWT 当前不是服务端会话模型；仓库已有个人访问
  token 的撤销能力，仍不足以直接实现“在线用户 + 强退登录会话”。应单独设计会话模型、
  revoke 策略、审计与前端联动。
- `#89` 是迁移系统大功能，应先做 RFC：一期 forward SQL 脚本发现/排序/执行/版本记录，
  二期 rollback 命名、顺序、失败策略。不能混入小修 PR。
- `#75` 混合多个 bug 和一个功能建议；其中“删除模型后 MenuBar 残留生成菜单”
  已能从当前代码直接确认，适合独立小 PR 修复。其余问题仍需要拆分复现。

## 已推进

- 新建 PR：`mss-boot-admin#371`，标题 `fix: wire query cache invalidation`。
- 代码改动：
  - `config/config.go`：将 `cache.queryCache` 初始化接入 GORM query cache plugin。
  - `config/config.go`：绑定 `response/actions/gorm.CleanCacheFromTag`，更新/删除后按
    `gorm.cache:<table>` 清理关联缓存 key。
  - `config/query_cache_test.go`：覆盖 query cache 初始化和 tag 前缀清理。
- 新建 PR：`mss-boot-admin#372`，标题 `fix: clean generated menus after model delete`。
- 代码改动：
  - `apis/model.go`：为模型删除增加 after delete hook。
  - 删除模型后递归软删除 `/virtual/<model.path>` 菜单树。
  - 如果仍有其他 active 模型使用相同 `path`，跳过菜单清理，避免误删共享 path
    的仍在用菜单。
  - 删除这些菜单/API 路径对应的 Casbin policy，并重新加载 policy。
  - `apis/model_test.go`：覆盖生成菜单树和 policy 被清理、无关菜单和 policy 不受影响、
    active 同 path 模型保护。
- 社区治理：
  - `#105` 已回复进展并关联 `#371`，标签整理为 `bug`、`type/bug`、`area/backend`、
    `status/has-pr`。
  - `#75` 已回复进展并关联 `#372`，明确只处理其中一个可验证子问题。
  - 新建 RFC issue：`mss-boot-admin#373`，标题 `RFC: online sessions and force logout`。
  - 新建 RFC issue：`mss-boot-admin#374`，标题 `RFC: SQL migration scripts and rollback`。
  - `#126` 已用友好语气迁移到 `#373` 并关闭，关闭原因是原 issue 已被更聚焦的
    RFC 取代。
  - `#89` 已用友好语气迁移到 `#374` 并关闭，关闭原因是原 issue 已被更聚焦的
    RFC 取代。
  - `#75` 已标记 `status/has-pr`，并作为混合 issue 关闭；其中已确认的模型删除菜单
    残留由 `#372` 处理，其余问题需要贡献者按单项重新提交可复现 issue。
  - `#373`、`#374` 已从 `needs-triage` 移除，保留 `needs-rfc`、`help wanted` 和
    相关 area/type 标签，表示后续需要方案讨论而不是再次分诊。

## 验证

```text
go test ./config
go test ./config ./middleware ./apis ./models
go test ./apis
go test ./apis ./models ./middleware
```

GitHub PR 检查：

- `mss-boot-admin#371`：CI、CodeQL、Docs Drift、PR Guard、govulncheck 全部通过；
  当前阻塞点是 `REVIEW_REQUIRED`。
- `mss-boot-admin#372`：CI、CodeQL、Docs Drift、PR Guard、govulncheck 全部通过；
  当前阻塞点是 `REVIEW_REQUIRED`。

## 后续

- maintainer 人工 review 并合并 `#371` 后，再关闭 `#105` 或请外部用户基于合并版本复测。
- maintainer 人工 review 并合并 `#372` 后，不需要重新打开 `#75`；如果贡献者拆出新的
  单项复现 issue，再按单项处理。
- `#373` 应先确认服务端会话模型、撤销策略、审计、权限控制和前端联动，再拆 PR。
- `#374` 应先确认 SQL migration 文件规范、执行记录、数据库差异和 rollback 边界，
  再拆 PR。
- 不要绕过 `REVIEW_REQUIRED` 强行合并 `#371`、`#372`；这是当前开源治理的人工
  review 交接点。
