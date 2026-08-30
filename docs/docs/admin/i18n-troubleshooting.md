---
title: 国际化排障
order: 14
nav:
  order: 1
  title: admin
description: Thin Host 业务国际化缺失、键名漂移与运行时语言快照排障
keywords: [react-intl i18n missing message thin host]
---

本文档从生成的 Thin Host 根目录处理：

- `[React Intl] Missing message: "..."`
- `[React Intl] Cannot format message: "..."`

## 一、快速定位步骤

1. 复制完整 message id（例如 `menu.super-permission.appConfig`）
2. 在业务拥有的前端目录搜索该 key：

```sh
rg "menu.super-permission.appConfig" web/src/locales web/src/business
```

3. 若不存在：补充到 `zh-CN` 与 `en-US`
4. 若存在：检查是否被拼接成错误 key（如重复前缀）

## 二、菜单 key 相关问题

### 现象

出现类似：

- `menu.menu.welcome`
- `menu.origination.origination.user`

### 原因

菜单名称可能已包含部分前缀，而布局层再次拼接 `menu.` 或父级路径，导致重复。

### 修复建议

- 保证 `menu/authorize` 返回给前端布局的 `name` 为布局预期格式
- 前端手工 `formatMessage` 时避免盲目追加前缀，可先判断是否已有 `menu.`

## 三、页面文案 key 缺失

### 现象

例如任一仍受支持的业务页面出现未翻译 key：

- `pages.user.form.name`
- `pages.role.form.code`

### 修复建议

业务文案只在 Thin Host 拥有的语言文件中补齐：

- `web/src/locales/zh-CN.ts`
- `web/src/locales/en-US.ts`

并保持中英文 key 对齐。不要修改 `node_modules` 中的 Admin Web 包，也不要把
Foundation 的核心语言目录复制进业务仓库。由生成器拥有的
`web/src/generated/locales/*` 应通过对应业务规格和生成器更新。

## 四、数据库语言包缓存

`GET /admin/api/language/profile` 把所有数据库语言定义作为一个完整快照缓存，而不是按语言分别写入。
快照 key 带有 Redis generation；增删改或迁移只需原子递增 generation，旧快照会立即失效并在 5 分钟后
自然过期。Profile 在读库前后校验 generation，因此并发的旧请求不能把已经失效的内容重新发布为当前快照。

Redis 是优化层：读取失败时 Profile 回退数据库；语言 CRUD 已提交后若缓存失效暂时失败，服务记录错误但仍返回
数据库写入成功，避免客户端因错误的 500 响应重试并产生重复数据。每组语言缓存操作使用 500ms 上限，避免 Redis
黑洞网络长时间阻塞页面或写请求。失效失败属于最终一致场景：旧快照最多保留 5 分钟；恢复 Redis 后，也可由运维
递增 `language:profile:generation` 立即切换到新快照。

## 五、验证方式

1. 从 Thin Host 根目录运行当前变更验证：

```sh
mss verify --changed
```

2. 刷新页面后确认控制台无新增 Missing message
3. 切换语言（`zh-CN` / `en-US`）分别验证关键页面

## 六、实践建议

- 新页面开发时，先定义完整 key 清单再编码
- PR 中附带“新增/变更 i18n key 列表”
- 避免在多处用不同命名风格表示同一语义（如 `appConfig` 与 `app-config` 混用）
