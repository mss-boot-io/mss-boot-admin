# mss-boot-admin 外部 Issue 修复与 dev 验证收尾

日期：2026-06-07

## 背景

本轮延续 `mss-boot-admin#371`、`mss-boot-admin#372` 的外部 issue 修复工作。
外部 review 用户名为 `12ain`，review 语境是中文，回复也应使用中文并明确感谢、
列出修复项和验证证据。

本轮遵循用户要求：

- 后端可发布到 `mss-boot-dev` 做验证。
- 前端不高频发布 Cloudflare，本轮未发布前端。
- 必须在 dev 环境充分验证后再考虑合并。
- 不自合并需要 review 的 PR。

## 已完成的修复 PR

### mss-boot-admin#371

标题：`fix: wire query cache invalidation`

目的：修复 `#105` 中启用 `queryCache` 后更新数据仍读到旧数据的问题。

补充修复：

- 缺少 cache adapter 时增加 warning，不再静默 no-op。
- query cache 初始化测试改为真实 GORM + Redis/miniredis 路径。
- 增加 nil tx、空 tag、callback 顺序契约等边界处理或说明。
- 依赖核心联动 PR `mss-boot#380`。

状态：

- CI、CodeQL、Docs Drift、PR Guard、govulncheck 全绿。
- `REVIEW_REQUIRED`，不得自合并。

### mss-boot-admin#372

标题：`fix: clean generated menus after model delete`

目的：修复 `#75` 中“删除模型后生成菜单残留”的可验证子问题。

补充修复：

- 菜单 soft-delete 与 Casbin policy 删除纳入 DB transaction。
- `LoadPolicy()` 失败降级为 warning，不再把已清理成功的数据状态上报 500。
- Casbin policy 删除改为精确匹配 `(ptype, v1, v2, v3)`。
- 只清理 `GeneratedData=true` 的模型，并限制根菜单 `type = MENU`。
- 增加事务回滚、LoadPolicy 降级、同 path/不同 type 或 method 保留、手工菜单保留等测试。

状态：

- CI、CodeQL、Docs Drift、PR Guard、govulncheck 全绿。
- `REVIEW_REQUIRED`，不得自合并。

### mss-boot#380

标题：`fix: keep query cache invalidation complete`

目的：补齐核心 query cache tag 记录和 create 后清理。

核心点：

- cache miss 首次写入时也记录 tag set。
- `RemoveFromTag` 删除缓存 key 时同时删除 tag set 本身。
- create 成功后按 table tag 调用 `CleanCacheFromTag`。

状态：

- CI、lint、CodeQL、Docs Drift、PR Guard、govulncheck 全绿。
- `REVIEW_REQUIRED`，不得自合并。

### mss-boot-admin#375

标题：`fix: keep PostgreSQL migrations compatible after tenant cleanup`

来源：在 #371/#372 组合验证镜像部署到 PostgreSQL/TimescaleDB dev 环境时暴露。

修复内容：

- 对遗留 `tenant_id NOT NULL` 列做兼容迁移，适配当前 `ModelGormTenant` 不再映射
  `tenant_id` 的代码状态。
- 在旧迁移 `1772445829126` 前置执行兼容处理，避免卡在该版本前的旧库继续失败。
- 新增 `20260607072000_relax_legacy_tenant_columns`，让已经跑过旧迁移的库也能补齐兼容。
- 将 `Menu.AfterCreate` 和 `1691847581348` 迁移中的 MySQL 反引号 `` `default` ``
  改为 GORM `clause.Eq`，由 GORM 按方言 quote 标识符。

状态：

- CI、CodeQL、Docs Drift、PR Guard、govulncheck 全绿。
- `REVIEW_REQUIRED`，不得自合并。

## dev 验证记录

命名空间：`mss-boot-dev`

验证镜像：

```text
ghcr.io/mss-boot-io/mss-boot-admin:015cf4d5dd895d65328cb6dfea4416b14f19612a
```

部署结果：

- `mss-boot-admin` deployment 1/1 ready。
- init migration 完成。
- dev 数据库遗留 `tenant_id` 列均已确认 nullable。
- migration 版本推进到 `2026060707200`。
- `queryCache: true`。
- 启动日志确认 GORM `gorm:query` callback 被替换。

API 冒烟：

- `/healthz` 返回 200。
- `admin/123456` 登录成功。
- `/admin/api/user/userInfo`、`/admin/api/options`、`/admin/api/models`、`/admin/api/menus`
  鉴权访问均返回 200。

query cache 验证：

- 临时 Option create/get/update/delete：
  - create 201
  - get 200
  - update 200
  - delete 204
  - delete 后再次 get 返回 404，未从 query cache 读回旧对象。

generated model menu cleanup 验证：

- 创建一次性模型并执行 `generate-data`。
- 删除前：
  - active menu = 2
  - policy = 2
- 删除模型后：
  - active menu = 0
  - policy = 0
- 临时生成表已 drop，未保留测试表。

## 社区沟通

已回复：

- `mss-boot-admin#371`：回复 `12ain`，说明 review blocking 项已处理、dev 验证已完成、#375
  是独立迁移兼容阻塞。
- `mss-boot-admin#372`：回复 `12ain`，说明事务、LoadPolicy、policy 精确删除和 dev 验证证据。
- `mss-boot-admin#375`：补充 dev 验证记录。
- `mss-boot#380`：补充 core 联动和 dev 验证记录。

当前没有新的外部人类反馈需要进一步互动。

## 合并顺序建议

建议人工 review 后按以下顺序处理：

1. `mss-boot#380`：核心 query cache invalidation 完整性。
2. `mss-boot-admin#375`：PostgreSQL/legacy migration 兼容，保证后续 dev/开源安装不再卡迁移。
3. `mss-boot-admin#371`：query cache 接入和 admin 侧清理绑定。
4. `mss-boot-admin#372`：generated model menu cleanup。

如必须调整顺序：

- `#372` 语义上可独立于 `#371/#380`，但 dev 验证依赖 `#375` 完成迁移。
- `#371` 必须等 `mss-boot#380` 通过 review 后再考虑合并，否则 admin 侧只能调用不完整的核心缓存清理能力。

## 注意事项

- 不要绕过 `REVIEW_REQUIRED`。
- 不要把 `codex/admin-dev-validation-20260607` 当成正式合并分支；该分支是组合验证用，包含
  #371、#372、#375 和 dev 镜像触发变更。
- 正式合并应使用各自 PR 分支。
- `make test` 会在本地改动 `testdata/test.tar.gz`、`testdata/test.zip`，这些是测试副作用，
  不应纳入提交。
