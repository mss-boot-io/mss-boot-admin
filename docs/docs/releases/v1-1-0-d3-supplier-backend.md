---
title: D3 Supplier Backend 内部 checkpoint
order: 15
description: v1.1.0 D3 Supplier 后端生成、显式三方言迁移、授权边界、精确开发证据与后续门禁
keywords: [v1.1.0 D3 supplier generator backend migration authorization]
---

# D3 Supplier Backend 内部 checkpoint

本文记录累计进 `v1.1.0` 的 Generator/Blueprint 旗舰轴 Supplier 后端检查点。
实现提交为 `5a60ad606fd8f17ed686aba29d44d8703cdceddf`，模板 revision 为
`1.1.0-backend.2`。这是未打 tag 的开发 checkpoint，不是完整 Supplier 模块、
feature-freeze SHA 或发布授权；`agent.fullstack-module-generator` 继续保持 Planned。

## 已落地的窄边界

`.mss/modules/example-supplier.yaml` 仍是唯一源合同，生成器现在把它确定性投影到
`admin/modules/supplier`：

- `20260810160000` 是完整、lossless 的显式 forward migration ID。生成 DDL 分别覆盖
  SQLite、MySQL 和 PostgreSQL，不以 `AutoMigrate` 代替生产迁移；migration runner 在任何
  数据库访问前统一验证注册错误和 duplicate ID。
- schema readiness 精确验证 required/nullability、唯一性、索引列顺序与唯一属性，以及
  `credit_level` CHECK 的实际表达式。相同名称但错误列、错误唯一属性或过宽 CHECK 都不能
  写入 migration version truth。
- model、Create/Update DTO、字段 validation、CRUD、过滤、排序、分页、CSV export 与 soft delete
  都来自同一 spec projection。SQLite、MySQL 和 PostgreSQL 在 GORM `TranslateError=false` 时仍把
  provider-native 唯一冲突归一为固定 `ErrSupplierConflict`；搜索中的 `%`、`_` 与 escape 字符不会
  被误当作 wildcard。
- `procurement.supplier.created` 与 `procurement.supplier.updated` 是 typed events，只在权威事务
  commit 后收集；失败事务不发事件。空事件声明仍生成可编译代码，非法事件名或同一 trigger
  的歧义声明在首次写文件前被拒绝。
- 六个声明操作生成 DTO/service/API/OpenAPI annotations 与 permission code。`RegisterRoutes`
  必须收到非 nil backend authorizer，否则拒绝挂载；descriptor 或 blank import 不会推断并自动
  挂载路由。
- 生成器会在写入前规划删除带正确 generated marker 的旧 auto-mounted controller/search 输出，
  但遇到 user-owned managed path 或 user-owned obsolete path 会在首次写入前整体失败。

## `complete=false` 是强制真值

当前 dry-run 必须保持以下机器状态：

```text
phase: backend-checkpoint
templateRevision: 1.1.0-backend.2
complete: false
managed changes: 15 unchanged
deferred projections: 26
```

26 项 deferred projection 包括字段 UI presentation、enum label/color、`generation.frontend`、
`generation.docs`、menu persistence/visibility、六个 permission 的 `defaultRoles` 与 display metadata、
permission matrix、browser E2E 和完整 UI。后端 authorizer injection 只证明“每个请求必须经过一个
明确 permission code”，不证明 admin/procurement/finance 的 policy seed 或默认角色授权已经落库。
因此本 checkpoint 不创建 menu，不暴露 UI，也不声称 typed frontend client、审计矩阵或 E2E 已完成。

## Exact development evidence

2026-08-11 在实现 checkpoint 上运行了以下结构、生成与 hermetic 后端证据。命令使用单包、完全
锚定的 `--run`、逐项 `--require`、`GOWORK=off`、race detector 和非缓存执行；缺失、skip、失败、
cached-only 或零命中都会失败。

```shell
go run ./cmd/mss spec validate .mss/modules/example-supplier.yaml --format json
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --format json
go run ./cmd/mss module generate .mss/modules/example-supplier.yaml --check --format json
```

```shell
go run ./cmd/mss test evidence --directory . --package ./internal/mss/generator \
  --run '^(TestGenerateDryRunWriteCheckAndDrift|TestGenerateRejectsUnsupportedBackendProjectionBeforeWriting|TestGenerateReportsDeferredSurfacesWithoutWritingFrontendOrDocs|TestGenerateRejectsUserOwnedObsoletePathBeforeAnyWrite|TestGeneratedUpgradeRemovesLegacyAutoMountedController|TestGenerateEventShapesCompileOrRejectBeforeWrite)$' \
  --count 1 --race --go-work off \
  --require TestGenerateDryRunWriteCheckAndDrift \
  --require TestGenerateRejectsUnsupportedBackendProjectionBeforeWriting \
  --require TestGenerateReportsDeferredSurfacesWithoutWritingFrontendOrDocs \
  --require TestGenerateRejectsUserOwnedObsoletePathBeforeAnyWrite \
  --require TestGeneratedUpgradeRemovesLegacyAutoMountedController \
  --require TestGenerateEventShapesCompileOrRejectBeforeWrite
```

```shell
go run ./cmd/mss test evidence --directory . --package ./internal/mss/spec \
  --run '^(TestLoadExampleSupplierModule|TestSupplierSourceSpecMatchesFeatureAccessContract|TestModuleValidationRejectsUnsafeOrAmbiguousEvents)$' \
  --count 1 --race --go-work off \
  --require TestLoadExampleSupplierModule \
  --require TestSupplierSourceSpecMatchesFeatureAccessContract \
  --require TestModuleValidationRejectsUnsafeOrAmbiguousEvents
```

```shell
go run ./cmd/mss test evidence --directory admin --package ./modules/supplier \
  --run '^(TestSupplierServiceCRUDQueryExportAndPostCommitEvents|TestSupplierForwardMigrationFreshRepeatAndConstraints|TestSupplierForwardMigrationPreservesUpgradeData|TestSupplierForwardMigrationRecoversAfterInterruptionWithoutPartialTruth|TestSupplierForwardMigrationRejectsExistingColumnShapeDriftWithoutVersionTruth|TestSupplierForwardMigrationRejectsNullableRequiredColumnsWithoutVersionTruth|TestSupplierForwardMigrationRejectsIndexShapeDriftWithoutVersionTruth|TestSupplierGeneratedMigrationDuplicateRegistrationFailsPreflightWithoutDatabase|TestSupplierForwardMigrationRejectsExistingCheckBodyDriftWithoutVersionTruth|TestSupplierTableName|TestSupplierMigrationIdentityIsComplete|TestSupplierPermissionCodesAreUnique|TestSupplierOperationContractMatchesSpecification|TestSupplierDescriptorNeverInfersMigrationOrRoutes|TestSupplierRegisterRoutesFailsClosedWithoutAuthorizer|TestSupplierHTTPAuthorizationValidationAndDeclaredOperations)$' \
  --count 1 --race --go-work off \
  --require TestSupplierServiceCRUDQueryExportAndPostCommitEvents \
  --require TestSupplierForwardMigrationFreshRepeatAndConstraints \
  --require TestSupplierForwardMigrationPreservesUpgradeData \
  --require TestSupplierForwardMigrationRecoversAfterInterruptionWithoutPartialTruth \
  --require TestSupplierForwardMigrationRejectsExistingColumnShapeDriftWithoutVersionTruth \
  --require TestSupplierForwardMigrationRejectsNullableRequiredColumnsWithoutVersionTruth \
  --require TestSupplierForwardMigrationRejectsIndexShapeDriftWithoutVersionTruth \
  --require TestSupplierGeneratedMigrationDuplicateRegistrationFailsPreflightWithoutDatabase \
  --require TestSupplierForwardMigrationRejectsExistingCheckBodyDriftWithoutVersionTruth \
  --require TestSupplierTableName \
  --require TestSupplierMigrationIdentityIsComplete \
  --require TestSupplierPermissionCodesAreUnique \
  --require TestSupplierOperationContractMatchesSpecification \
  --require TestSupplierDescriptorNeverInfersMigrationOrRoutes \
  --require TestSupplierRegisterRoutesFailsClosedWithoutAuthorizer \
  --require TestSupplierHTTPAuthorizationValidationAndDeclaredOperations
```

同一实现 checkpoint 还从 `admin/` 以 `GOWORK=off go test -race -count=1 ./modules/supplier` 在临时
MySQL 8.4 与 PostgreSQL 17 容器上运行了四个真实顶级测试，两条 DSN 均非空且 zero-skip。
冻结阶段必须改用以下 exact runner 形状，防止缺失、skip 或零命中被普通 package 结果掩盖：

```shell
go run ./cmd/mss test evidence --directory admin --package ./modules/supplier \
  --run '^TestSupplierMigration(MySQLIntegration|PostgresIntegration|MySQLRejectsCheckAndIndexDrift|PostgresRejectsCheckAndIndexDrift)$' \
  --count 1 --race --go-work off \
  --require TestSupplierMigrationMySQLIntegration \
  --require TestSupplierMigrationPostgresIntegration \
  --require TestSupplierMigrationMySQLRejectsCheckAndIndexDrift \
  --require TestSupplierMigrationPostgresRejectsCheckAndIndexDrift
```

Provider suite 覆盖 fresh/repeat/constraints、interrupted recovery、关闭 GORM error translation 后的
create/update conflict、escaped wildcard search、同名错误 index shape 和 permissive CHECK body rejection。
SQLite hermetic suite 另有旧表数据保留 upgrade。该开发运行不绑定未来冻结 SHA，也不能替代完整发布门禁；
冻结后必须以 allowlisted `mss_supplier_test` 数据库、同一候选提交、两个 DSN 和 zero-skip 重跑。

## 兼容、安全与迁移

这是 additive vertical module 和 generator/template 扩展，不删除 v1.0 exported Go API。生成模块通过
blank import 注册 descriptor，但不会自动挂载 HTTP；没有 composition-root authorizer 时固定失败，
也没有 legacy global permission fallback。错误响应不携带 SQL、constraint 名称、provider error 或
冲突值。

采用该模块的数据库需要按顺序应用 migration `20260810160000`。迁移只新增
`biz_suppliers`、indexes 与 CHECK，不提供 destructive down migration。若部署后需要撤回，先停止 route/menu
组合和 permission grants，保留表及业务数据，再通过新的 idempotent forward migration 修复；不得手工删除
migration truth 或重建无关表。生成器删除 obsolete 文件时只处理带预期 generated marker 的路径，手写内容
始终 fail closed。

## 后续门禁

D4 必须生成并验证 default-role/policy/menu persistence、admin/procurement/finance 正负授权、拒绝路径审计，
并保持 backend permission 为唯一授权源。D5 必须生成 typed client、list/form/detail/export、双语 locale、
loading/empty/error/permission-denied/conflict 等 UI 状态、browser E2E、模块文档，以及 Blueprint
0.1→0.2 定制保留和第二次空升级。

完成这些开发项后，generation plan 才能从 `complete=false` 变为 `complete=true`。随后才能选择单一
feature-freeze SHA，重跑三数据库、权限、browser、generated drift、external downstream、upgrade、
`verify --all` 与 `eval run --all`；本 checkpoint 的绿色结果不授予 tag、Release 或 package 发布权。
