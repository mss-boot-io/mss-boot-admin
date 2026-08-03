---
title: 移动端 H5 适配说明
order: 27
nav:
  order: 1
  title: admin
description: mss-boot-admin 移动端响应式适配实现说明
keywords: [admin mobile h5 responsive adaptation]
---

## 概述

mss-boot-admin 前端已完成移动端 H5 适配，支持在手机浏览器中正常使用所有管理功能。本文档说明适配方案、实现细节和使用指南。

## 适配范围

### 已适配模块

| 模块 | 移动端组件 | 说明 |
|------|-----------|------|
| 用户管理 | `MobileUserList` | 卡片式列表，显示头像、姓名、邮箱、手机、部门 |
| 角色管理 | `MobileRoleList` | 卡片式列表，支持编辑、授权、删除 |
| 部门管理 | `MobileDepartmentList` | 卡片式列表，支持树形结构 |
| 岗位管理 | `MobilePostList` | 卡片式列表，支持树形结构 |
| 菜单管理 | `MobileMenuList` | 卡片式列表，扁平化显示树形菜单 |
| 选项管理 | `MobileOptionList` | 卡片式列表 |
| 任务管理 | `MobileTaskList` | 卡片式列表 |
| 通知管理 | `MobileNoticeList` | 卡片式列表 |
| 语言管理 | `MobileLanguageList` | 卡片式列表 |
| 个人中心 | `MobileCenter` | 卡片式布局，头像居中，分组显示信息 |
| 设置页面 | `MobileSettings` | 顶部 Tabs 布局 |

### 核心功能

- ✅ 移动端菜单（Drawer 抽屉模式）
- ✅ 响应式宽度适配
- ✅ 深色/浅色主题自动切换
- ✅ 触摸友好的交互设计

---

## 技术实现

### 响应式检测

使用 Ant Design Grid 的 breakpoint 实现：

```typescript
// src/hooks/useResponsive.ts
import { Grid } from 'antd';

export const useResponsive = () => {
  const screens = Grid.useBreakpoint();
  
  return {
    screens,
    isMobile: !screens.md,    // < 768px
    isTablet: !!screens.md && !screens.lg,
    isDesktop: !!screens.lg,  // >= 992px
  };
};
```

### 条件渲染

页面组件根据屏幕尺寸选择渲染模式：

```typescript
// src/pages/User/index.tsx
import { useResponsive } from '@/hooks/useResponsive';
import MobileUserList from './Mobile/UserList';

const UserList: React.FC = () => {
  const { isMobile } = useResponsive();
  
  if (isMobile) {
    return <MobileUserList />;
  }
  
  return <ProTable ... />;
};
```

### 样式架构

```
src/styles/mobile.less       # 通用移动端样式
src/global.less              # 全局响应式样式
```

---

## 样式规范

### 通用移动端样式

```less
// src/styles/mobile.less
.mobileContainer {
  width: 100%;
  padding: 12px;
  background: var(--ant-color-bg-layout);
  
  .card {
    width: 100%;
    background: var(--ant-color-bg-container);
    border-radius: 8px;
  }
  
  .cardHeader {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  
  .cardBody .field {
    display: flex;
    justify-content: space-between;
    padding: 6px 0;
  }
}
```

### 主题变量

使用 Ant Design 5.x CSS 变量实现主题适配：

| 变量名 | 用途 |
|--------|------|
| `--ant-color-bg-container` | 容器背景色 |
| `--ant-color-bg-layout` | 布局背景色 |
| `--ant-color-text` | 主文本色 |
| `--ant-color-text-secondary` | 次要文本色 |
| `--ant-color-border` | 边框色 |
| `--ant-color-primary` | 主色 |
| `--ant-color-primary-bg` | 主色背景 |

---

## 移动端菜单

### 实现方式

移动端使用 Drawer 抽屉替代固定侧边栏：

```less
// src/global.less
@media (max-width: 768px) {
  // 隐藏固定侧边栏
  .ant-pro-sider.ant-pro-sider-fixed {
    display: none !important;
  }
  
  // Drawer 内的菜单
  .ant-drawer {
    .ant-pro-sider {
      display: block !important;
      width: 100% !important;
    }
  }
}
```

### 菜单触发

点击顶部导航栏的菜单图标打开 Drawer。

---

## 测试覆盖

### E2E 测试

测试文件：`e2e/mobile.spec.ts`

```typescript
// 使用 iPhone 12 Pro 视口
test.use({
  viewport: { width: 390, height: 844 },
  isMobile: true,
  hasTouch: true,
});

test('should display mobile menu correctly', async ({ page }) => {
  // 验证菜单包含项目
});

test('should display user data correctly on mobile', async ({ page }) => {
  // 验证用户数据渲染
});
```

### 测试结果

```
Running 10 tests using 6 workers

✅ Mobile menu contains 7 items
✅ Mobile container width adapted correctly
✅ User data displayed: 2 items
✅ Account center mobile component rendered
✅ Account settings mobile component rendered with top tabs

10 passed (25.7s)
```

---

## 文件结构

```
src/
├── hooks/
│   └── useResponsive.ts          # 响应式检测 Hook
├── styles/
│   └── mobile.less               # 通用移动端样式
├── global.less                   # 全局响应式样式
└── pages/
    ├── User/Mobile/
    │   └── UserList.tsx
    ├── Role/Mobile/
    │   └── RoleList.tsx
    ├── Department/Mobile/
    │   └── DepartmentList.tsx
    ├── Post/Mobile/
    │   └── PostList.tsx
    ├── Menu/Mobile/
    │   └── MenuList.tsx
    ├── Option/Mobile/
    │   └── OptionList.tsx
    ├── Task/Mobile/
    │   └── TaskList.tsx
    ├── Notice/Mobile/
    │   └── NoticeList.tsx
    ├── Language/Mobile/
    │   └── LanguageList.tsx
    └── Account/
        ├── Center/Mobile/
        │   └── index.tsx
        └── Settings/Mobile/
            └── index.tsx

e2e/
└── mobile.spec.ts                # 移动端测试用例
```

---

## 使用指南

### 开发调试

1. 启动前端服务：
```bash
cd mss-boot-admin-antd
pnpm dev
```

2. 打开浏览器开发者工具（F12）

3. 切换到移动设备模式（Ctrl+Shift+M）

4. 选择设备或自定义视口尺寸

### 推荐测试设备

| 设备 | 视口尺寸 | 说明 |
|------|----------|------|
| iPhone SE | 375 × 667 | 小屏手机 |
| iPhone 12 Pro | 390 × 844 | 主流 iPhone |
| iPhone 14 Pro Max | 430 × 932 | 大屏 iPhone |
| Pixel 5 | 393 × 851 | Android 主流 |
| Samsung Galaxy S20 | 360 × 800 | Android 小屏 |

---

## 注意事项

### 数据共享原则

移动端与桌面端共享数据源，只改变渲染方式：

```typescript
// 正确：共享数据，props 传递
const Center: React.FC = () => {
  const [userInfo, setUserInfo] = useState<API.User>();
  
  // 数据获取逻辑...
  
  if (isMobile) {
    return <MobileCenter userInfo={userInfo} />;
  }
  
  return <DesktopView userInfo={userInfo} />;
};

// 错误：移动端独立获取数据
const MobileCenter: React.FC = () => {
  const [userInfo, setUserInfo] = useState<API.User>();
  // 移动端自己获取数据会导致与桌面端不一致
};
```

### 样式覆盖

移动端样式使用 `!important` 时需谨慎，避免影响主题切换：

```less
// 推荐：使用 CSS 变量
.mobileContainer {
  background: var(--ant-color-bg-layout);
}

// 避免：硬编码颜色
.mobileContainer {
  background: #f5f5f5 !important;  // 深色主题下会出错
}
```

---

## 后续优化方向

| 方向 | 说明 | 优先级 |
|------|------|--------|
| 下拉刷新 | 移动端下拉刷新列表数据 | 中 |
| 无限滚动 | 列表分页无限滚动加载 | 中 |
| 手势操作 | 滑动删除、左滑菜单等 | 低 |
| PWA 支持 | 离线访问、添加到桌面 | 低 |
| 性能优化 | 懒加载、图片压缩 | 中 |

---

## 推荐阅读

- [五期路线图](/admin/phase-5-roadmap)
- [当前功能总览](/admin/current-capabilities)
- [集成测试指南](/admin/integration-test-guide)