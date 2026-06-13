# 2026-06-14 迁移版本碰撞修复

## 背景

- `mss-boot/pkg/migration.GetFilename` 取文件名前 13 字符并 `cast.ToInt`，用作迁移版本 map 的 key。
- `cmd/migrate/migration/system/20260607162057_user_sessions.go` 与
  `cmd/migrate/migration/system/20260607162058_session_menus.go` 前 13 字符均为
  `2026060716205` → 同一 key。
- `Migration.SetVersion` 用 map[int] 存储 handler，第二个 `init()` 覆盖第一个：
  按 Go 包初始化顺序，`session_menus` 注册晚，覆盖了 `user_sessions`。
- 已经在受影响版本跑过迁移的环境：
  - `mss_boot_user_sessions` 表未建。
  - `mss_boot_migrate_versions` 已记录 version=`2026060716205`（其实是
    `session_menus` 记的）。

## 决策

1. 文件改名 `20260607162058_session_menus.go` → `20260607162060_session_menus.go`
   解决碰撞（前 13 字符变为 `2026060716206`）。**新装库** 因此恢复正常。
2. 但 **已损坏的旧库** 仍有 `2026060716205` 这条记录，迁移器会把
   `user_sessions` 当作 Done 跳过 → 表仍缺失 → 会话登录仍 401。
3. 追加 idempotent 修复迁移
   `20260614120000_repair_user_sessions.go`：
   - 取一个全新的、不会再碰撞的 13 字符前缀 `2026061412000`。
   - 内部调用 `tx.Migrator().AutoMigrate(new(models.UserSession))`，已存在则
     no-op，缺失则补建。
   - 健康环境（无碰撞）也会跑一次，开销可忽略。
4. 不重写历史迁移，不删除 `2026060716205` 记录 —— 避免破坏其它依赖该 version
   表行为的脚本，也方便审计哪些库走过哪条路径。

## 验证目标

- `go test ./cmd/migrate/migration/system`：包内测试通过。
- SQLite 全量迁移：12 → 13 versions 全部 applied，`mss_boot_user_sessions` 表存在。
- MySQL 全量迁移：同上，session 列表、力撤销、自登出全部正常。
- 复现旧碰撞场景：手工把 SQLite 库回退到 PR #114 之前状态，在
  `mss_boot_migrate_versions` 留下 `2026060716205` 但不建 `mss_boot_user_sessions`；
  再跑本次迁移 → 表被补建，登录恢复。

## 兼容性说明

- 同时也修了 `20260403225953_enhance_options.go`：
  - Postgres 加 `current_schema()` 分支替代 MySQL-only `DATABASE()`。
  - 所有 `tx.Raw(...).Scan` / `tx.Exec` 加 `.Error` 检查。
  - 去掉 MySQL 不支持的 `CREATE INDEX IF NOT EXISTS`，改用
    `information_schema.statistics` 存在性探针 + 裸 `CREATE INDEX`。
- `1746193492486_migrate.go` 用 `quoteCol(dialect)` 让保留字 `group` 列在 MySQL
  下用反引号、其它方言用双引号。
