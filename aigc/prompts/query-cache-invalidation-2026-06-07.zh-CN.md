# Query Cache Invalidation

日期：2026-06-07

## 背景

外部 issue `#105` 反馈：启用 `cache.queryCache` 后，更新数据后列表显示已更新，
再次打开编辑/详情仍可能读到旧数据。

## 决策

- admin 启动时必须把 mss-boot 的 query cache 初始化回调传给 `Cache.Init`。
- 初始化 query cache 后，必须绑定 `response/actions/gorm.CleanCacheFromTag`。
- mss-boot GORM update/delete actions 会调用 `CleanCacheFromTag(tableName)`；
  admin 侧需要把表名转换为 mss-boot cache 实际使用的 tag：`gorm.cache:<table>`。
- 社区 review 后确认仅 admin 接入还不完整：mss-boot 核心需要在 cache miss 写入新缓存后
  同步把 key 加入 table tag set，并在 create 成功后清理 table tag，避免新增数据后
  list/search 缓存继续返回旧结果。

## 本轮变更

- `config/config.go`
  - 接入 `cache.queryCache` 的 GORM plugin 初始化。
  - 注册基于 table tag 的缓存清理函数。
  - 当 query cache 开启但没有可用 cache adapter 时输出 warning，避免静默失效。
  - 记录 `Cache.Init` 中 set callback 先于 queryCache callback 的顺序契约。
- `config/query_cache_test.go`
  - 使用真实 SQLite `*gorm.DB` 覆盖 query cache 初始化和 GORM query callback 路径。
  - 覆盖 `gorm.cache:` tag 前缀清理。
  - 覆盖缺失 cache adapter 和 nil `*gorm.DB` 的安全行为。
- `go.mod` / `go.sum`
  - 临时指向 `github.com/mss-boot-io/mss-boot v0.7.3-0.20260607064058-ea17965d7546`，
    对应核心修复 PR `mss-boot#380`。

## 验证

```text
go test ./config
go test ./config ./middleware ./apis ./models
```

## 后续

- 若 `#105` 仍可复现，需要用户补充版本、缓存配置、更新接口、读取接口和复现步骤。
- admin PR `#371` 合并前需要确认 mss-boot PR `#380` 已通过 review，或保留 pseudo-version
  依赖并在 dev 环境验证候选镜像。
- dev 验证必须启用 `cache.queryCache: true`，预热 get/list 缓存后执行 create/update/delete，
  再确认 get/list 不返回旧数据。
