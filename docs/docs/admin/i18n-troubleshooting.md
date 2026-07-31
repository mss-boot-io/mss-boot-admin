---
title: 国际化排障
order: 14
nav:
  order: 1
  title: admin
description: React Intl Missing message 常见问题与修复方法
keywords: [react-intl, i18n, missing message, mss-boot-admin]
---

本文档基于当前仓库常见问题整理，重点处理：

- `[React Intl] Missing message: "..."`
- `[React Intl] Cannot format message: "..."`

## 一、快速定位步骤

1. 复制完整 message id（例如 `menu.super-permission.appConfig`）
2. 在前端仓库搜索该 key：

```bash
grep -R "menu.super-permission.appConfig" src/locales
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

例如 `Generate` 页面：

- `pages.generator.steps.params.tooltip`
- `pages.generator.repo`
- `pages.generator.service`
- `pages.generator.email`

### 修复建议

优先在聚合语言文件中补齐：

- `src/locales/zh-CN.ts`
- `src/locales/en-US.ts`

并保持中英文 key 对齐，避免仅修复单语种后另一个语种继续告警。

## 四、验证方式

1. 前端执行类型检查：

```bash
pnpm -s tsc --noEmit
```

2. 刷新页面后确认控制台无新增 Missing message
3. 切换语言（`zh-CN` / `en-US`）分别验证关键页面

## 五、实践建议

- 新页面开发时，先定义完整 key 清单再编码
- PR 中附带“新增/变更 i18n key 列表”
- 避免在多处用不同命名风格表示同一语义（如 `appConfig` 与 `app-config` 混用）
