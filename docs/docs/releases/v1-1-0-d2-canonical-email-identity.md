---
title: D2 Canonical Email Identity 内部 checkpoint
order: 11
description: v1.1.0 D2 邮箱身份唯一迁移、写入边界、验证证据与冻结前剩余门禁
keywords: [v1.1.0 D2 email identity migration mysql postgresql sqlite security]
---

# D2 Canonical Email Identity 内部 checkpoint

本文记录累积进 `v1.1.0` 的 `D2-contract-substrate` 邮箱身份开发检查点。实现分别落在
`9ccb1d2`、`eb1277c`、`3710aca` 和 `8b361ce`；这些提交和开发环境数据库运行都不是 feature-freeze SHA、
Git tag、GitHub Release 或发布授权。当前 capability 仍是 Planned。

## 已落地的身份合同

- 非空邮箱在 model boundary 解析为不超过 100 字节的 ASCII full address，去除首尾空白并对整地址
  case-fold；非法值返回 typed validation error。空邮箱继续表示没有邮箱身份。
- 唯一所有权只约束 `deleted_at IS NULL` 且非空的 identity。soft-deleted 历史行不占用 identity，
  后续 active 用户可安全复用。
- SQLite 和 PostgreSQL 使用命名的 partial expression unique index；MySQL 使用只为 active/non-empty
  行生成值的 nullable stored `VARBINARY(100)` key，再建立同名 unique index。迁移会核对实际 metadata，
  不能用“索引名称存在”代替表达式、predicate、列类型、顺序和唯一性检查。
- 迁移在任何 backfill 或 DDL 前读取全部存量 active identity，汇总 invalid/conflict 数量；错误和日志
  不输出邮箱、用户 ID、SQL 或 driver detail。预检失败不修改数据、不创建索引、不记录 migration version。
- backfill 使用原始邮箱值参与 compare-and-swap；并发变化会固定失败，不覆盖新值。完整迁移成功后才记录
  full migration identifier，重复运行保持幂等。
- 邮件注册和首次 OAuth provisioning 都在事务内建立新 user；provider email 永远不会把新 OAuth identity
  合并到既有本地账户。legacy `varchar(20)` username 使用有碰撞重试的 bounded opaque 值，不再复制邮箱。
- 只有命名的 canonical-email constraint 可归类为 identity conflict。Admin create/update 对非法 identity
  固定返回 `422 INVALID_EMAIL_IDENTITY`，对已占用或歧义 identity 固定返回
  `409 EMAIL_IDENTITY_UNAVAILABLE`；其他数据库错误走通用 redacted fallback。自助邮箱修改仍关闭。
- migration 结束后与 Admin server 启动时共用一个 fixed/redacted schema verifier；只有 migration marker、
  canonical data、dialect-specific generated column/index/predicate 全部精确匹配，server 才挂载业务路由。
  缺失、漂移、数据库失败或敏感冲突固定 fail closed，不把原始错误复制到日志或 readiness 响应。

## 开发 checkpoint evidence

机器合同在
`.mss/features/foundation-v1-1-0-generator-blueprint.yaml` 中精确列出了六个
`mss test evidence` 命令，全部指定 `--race --go-work off`、每一个 required test 和非缓存计数：

1. SQLite fresh/repeat、upgrade/backfill、preflight no-mutation、CAS、soft-delete reuse、并发 claim、
   data/DDL metadata 验证；
2. model canonicalization、constraint-specific normalization、邮件注册、GitHub/Lark OAuth、bounded opaque
   username、no-email-merge、歧义 fail-closed 和敏感 SQL 抑制；
3. 实际 Admin User Controller 的 422/409、root guard、无关 unique error redacted fallback。
4. schemahealth 的 ready、缺失/错误 SQLite index、精确 migration version、敏感数据漂移、数据库失败，
   以及错误 PostgreSQL/MySQL metadata shape 六项正负验证；
5. migrate 完成所有 migrations 后调用同一个 runtime schema verifier；
6. server 只在 canonical-email schema readiness 成功后挂载业务路由。

这些命令是可重跑的开发哨兵。它们不证明 MySQL/PostgreSQL provider 行为，也不能复用为未来冻结 SHA
的 evidence。

## 冻结前仍未通过的 required gates

以下任一项未完成时，不能把邮件注册或首次 OAuth provisioning 宣称为生产 ready，也不能完成 D2 或
选择 `v1.1.0` feature-freeze SHA：

1. server startup/readiness 的六项 schemahealth 正负测试和 migrate/server 两项 composition 测试已在
   `8b361ce` 开发 checkpoint 完成；选择最终 feature-freeze SHA 后必须从该提交全部重跑并保持
   required `run/pass`、zero-skip，不能复用当前结果。
2. 冻结提交必须同时提供 `MSS_EMAIL_IDENTITY_TEST_MYSQL_DSN` 和
   `MSS_EMAIL_IDENTITY_TEST_POSTGRES_DSN`。两个 DSN 都只能指向 allowlisted
   `mss_email_identity_test` 数据库。
3. 在该冻结提交上运行 exact integration evidence：

   ```shell
   go run ./cmd/mss test evidence --directory admin --package ./cmd/migrate/migration/system \
     --run '^TestCanonicalEmailIdentityMigration(MySQL|Postgres)Integration$' \
     --count 1 --race --go-work off \
     --require TestCanonicalEmailIdentityMigrationMySQLIntegration \
     --require TestCanonicalEmailIdentityMigrationPostgresIntegration
   ```

   evidence runner 必须记录两个 required test 都 `run=1/pass=1/skip=0`。任一 DSN 缺失会触发 test skip，
   因而整个门禁失败；当前一次手工运行不能替代这个 exact-freeze 结果。
4. 该 SHA 仍需通过完整三数据库 release migration matrix、权限、安全 redaction、`verify --all`、
   `eval run --all` 和其余 phase-aware readiness；本 checkpoint 没有提前启动这些发布检查。

## 升级、失败恢复与回滚

部署新 Admin 前先备份数据库并执行 forward migration。若预检报告 invalid/conflict 计数，保持旧版本运行，
由授权运维人员在不导出身份值到普通日志的受控流程中解决数据，再重跑迁移；不得自动合并账户、选择第一行、
手工插入 migration marker 或绕过唯一约束。

迁移成功后，旧 Admin 仍可读取规范化后的邮箱；新增索引只限制 active/non-empty 重复值，空值和 soft-deleted
历史记录保持兼容。不要以删除索引或 generated column 作为应用回滚，因为那会重新打开并发双 owner 风险。
若新版本启动时 readiness 失败，停止流量并关闭邮件注册/首次 OAuth 建号，保留已安装约束和数据，使用前向修复
恢复精确 metadata 后再启动。已经公开的版本只做 forward repair，不移动或重写 tag。

本切片不开放自助邮箱变更，也不改变 ChallengeStore 的 provider maturity；Challenge 的 Redis/SMTP readiness
不能替代数据库 identity readiness。
