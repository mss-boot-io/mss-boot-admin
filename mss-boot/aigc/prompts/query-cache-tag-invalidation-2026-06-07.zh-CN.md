# Query Cache Tag Invalidation

日期：2026-06-07

## 背景

`mss-boot-admin#105` 反馈启用 `cache.queryCache` 后，数据更新后详情/编辑接口可能读到旧缓存。
admin PR `mss-boot-admin#371` 已接入 query cache 初始化和 table tag 清理，但社区 review
指出核心层仍有两个缺口：

- Redis query cache 只在缓存命中时把 key 加入 tag set；首次 cache miss 后写入的新缓存
  没有关联到 `gorm.cache:<table>`，后续 `RemoveFromTag` 可能清不到它。
- GORM create action 成功后没有触发 `CleanCacheFromTag(table)`，已有 list/search 缓存会
  等 TTL 才刷新。

## 决策

- query cache miss 后成功 `SaveCache` 时，也必须调用 `SaveTagKey(ctx, tag, key)`。
- `RemoveFromTag` 删除被 tag 关联的 cache keys 后，也删除 tag set 本身；空 tag set
  应安全返回，不发出无 key 的 Redis DEL。
- GORM create action 成功提交事务后，调用 `CleanCacheFromTag(c, m.TableName())`，与
  update/delete 的缓存失效语义保持一致。
- 不改变公开 API；本次不处理 `Cache.Init` callback 显式传 adapter 这类较大 API 优化。

## 本轮变更

- `pkg/config/storage/cache/redis.go`
  - cache miss 保存缓存后写入 tag set。
  - `RemoveFromTag` 同时删除缓存 key 和 tag set，空集合安全处理。
- `pkg/response/actions/gorm/control.go`
  - create 成功后清理 table tag。
- `pkg/config/storage/cache/redis_test.go`
  - 使用 `miniredis` 覆盖首次查询写入缓存后 tag set 包含 key。
  - 覆盖 `RemoveFromTag` 删除 key 和 tag set。
- `pkg/response/actions/gorm/control_test.go`
  - 使用 Gin + SQLite 覆盖 create action 成功后调用 `CleanCacheFromTag`。

## 验证

```text
go test ./pkg/config/storage/cache ./pkg/response/actions/gorm
go test ./...
```

## 后续

- admin PR `mss-boot-admin#371` 合并前需要使用包含本修复的 mss-boot 版本或 pseudo-version
  构建候选镜像，并在 `mss-boot-dev` 验证 create/update/delete 后 get/list 不返回旧缓存。
- 若后续要消除 `Cache.Init(set, queryCache)` 的隐式顺序契约，可以单独设计 API：
  让 queryCache callback 显式接收已初始化的 cache adapter。
