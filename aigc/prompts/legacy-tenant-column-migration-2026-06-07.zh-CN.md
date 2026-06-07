# 2026-06-07 dev 验证阻塞：遗留 tenant_id 非空列兼容迁移

## 背景

- 当前 dev 环境 `mss-boot-dev` 使用 PostgreSQL/TimescaleDB 旧库。
- 代码中的 `models.ModelGormTenant` 已经收敛为兼容空壳，不再映射 `tenant_id` 字段。
- dev 库中仍有多张旧表保留 `tenant_id NOT NULL`，导致新镜像 init migration 在 `1772445829126` 写入 `mss_boot_menus` 时失败。

## 决策

- 不恢复租户字段，不在业务插入点硬编码默认租户。
- 在 `1772445829126` 开始处释放遗留 `tenant_id NOT NULL`，保证卡在该版本前的旧库可继续迁移。
- 新增 `20260607072000_relax_legacy_tenant_columns`，保证已经跑过旧迁移的环境也能补齐同样 schema 兼容。
- PostgreSQL 使用 `ALTER COLUMN tenant_id DROP NOT NULL`；MySQL 保留 `information_schema.columns.column_type` 后 `MODIFY COLUMN ... NULL`；SQLite 暂不处理。

## 验证目标

- `go test ./cmd/migrate/migration/system`。
- dev 环境新镜像 init migration 不再因 `tenant_id` 非空失败。
- 后续 #371/#372 的 dev 验证必须基于包含该迁移兼容修复的验证镜像。
