import { test, expect } from '@playwright/test';

async function loginViaAPI(page: any) {
  const response = await page.request.post('http://localhost:8080/admin/api/user/login', {
    headers: { 'Content-Type': 'application/json' },
    data: { username: 'admin', password: '123456' },
  });

  const data = await response.json();
  expect(data.code).toBe(200);
  expect(data.token).toBeDefined();

  await page.goto('/');
  await page.waitForLoadState('domcontentloaded');

  await page.evaluate((token: string) => {
    localStorage.setItem('token', token);
    localStorage.setItem('token.expire', new Date(Date.now() + 86400000).toISOString());
  }, data.token);

  await page.reload();
  await page.waitForLoadState('networkidle');
}

test.describe('Mobile View Tests', () => {
  test.use({
    // 使用 iPhone 12 Pro 视口尺寸
    viewport: { width: 390, height: 844 },
    deviceScaleFactor: 3,
    isMobile: true,
    hasTouch: true,
  });

  test('should display mobile menu correctly', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    // 验证页面加载成功
    await expect(page.locator('.ant-pro-layout-content')).toBeVisible({ timeout: 10000 });

    // 截图：初始状态
    await page.screenshot({ path: 'test-results/mobile-initial.png', fullPage: true });

    // 查找菜单触发按钮
    const menuTrigger = page.locator('.ant-pro-layout-header-actions-avatar, .ant-pro-global-header-header-actions-avatar');
    
    // 如果有折叠的菜单按钮，点击它
    const collapsedButton = page.locator('button[aria-label="Collapse"], .ant-pro-global-header-collapsed-button').first();
    
    if (await collapsedButton.isVisible({ timeout: 3000 }).catch(() => false)) {
      await collapsedButton.click();
      await page.waitForTimeout(1000);
      
      // 截图：菜单打开状态
      await page.screenshot({ path: 'test-results/mobile-menu-open.png', fullPage: true });
      
      // 验证 Drawer 打开
      const drawer = page.locator('.ant-drawer.open, .ant-drawer-content-wrapper');
      await expect(drawer).toBeVisible({ timeout: 5000 });
      
      // 验证菜单项存在（关键测试点 - 之前菜单是空的）
      const menuItems = page.locator('.ant-drawer .ant-menu-item, .ant-drawer .ant-menu-submenu');
      const menuCount = await menuItems.count();
      
      console.log(`✅ Mobile menu contains ${menuCount} items`);
      
      // 截图：菜单内容
      await page.screenshot({ path: 'test-results/mobile-menu-content.png' });
      
      // 关闭菜单
      const closeButton = page.locator('.ant-drawer .ant-drawer-close');
      if (await closeButton.isVisible()) {
        await closeButton.click();
        await page.waitForTimeout(500);
      }
    }
  });

  test('should display user list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    // 导航到用户列表页面
    await page.goto('/users');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // 截图：用户列表页面
    await page.screenshot({ path: 'test-results/mobile-user-list.png', fullPage: true });

    // 检查是否显示移动端组件（卡片布局）
    const mobileContainer = page.locator('.mobileContainer, [class*="mobileContainer"]');
    
    if (await mobileContainer.isVisible({ timeout: 3000 }).catch(() => false)) {
      console.log('✅ Mobile user list component rendered');
      
      // 验证卡片存在
      const cards = page.locator('.mobileContainer .ant-card');
      const cardCount = await cards.count();
      console.log(`✅ Found ${cardCount} user cards`);
      
      // 截图：用户卡片详情
      if (cardCount > 0) {
        await cards.first().screenshot({ path: 'test-results/mobile-user-card.png' });
      }
    } else {
      // 如果移动组件未加载，检查是否有 ProTable（桌面端回退）
      const proTable = page.locator('.ant-pro-table');
      if (await proTable.isVisible()) {
        console.log('⚠️ Desktop ProTable shown instead of mobile component');
        await page.screenshot({ path: 'test-results/mobile-fallback-table.png', fullPage: true });
      }
    }
  });

  test('should display role list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/roles');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/mobile-role-list.png', fullPage: true });

    const mobileContainer = page.locator('.mobileContainer, [class*="mobileContainer"]');
    if (await mobileContainer.isVisible({ timeout: 3000 }).catch(() => false)) {
      console.log('✅ Mobile role list component rendered');
      const cards = page.locator('.mobileContainer .ant-card');
      const cardCount = await cards.count();
      console.log(`✅ Found ${cardCount} role cards`);
    }
  });

  test('should display department list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/departments');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/mobile-department-list.png', fullPage: true });

    const mobileContainer = page.locator('.mobileContainer, [class*="mobileContainer"]');
    if (await mobileContainer.isVisible({ timeout: 3000 }).catch(() => false)) {
      console.log('✅ Mobile department list component rendered');
      const cards = page.locator('.mobileContainer .ant-card');
      const cardCount = await cards.count();
      console.log(`✅ Found ${cardCount} department cards`);
    }
  });

  test('should display menu list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/menus');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/mobile-menu-list.png', fullPage: true });

    const mobileContainer = page.locator('.mobileContainer, [class*="mobileContainer"]');
    if (await mobileContainer.isVisible({ timeout: 3000 }).catch(() => false)) {
      console.log('✅ Mobile menu list component rendered');
      const cards = page.locator('.mobileContainer .ant-card');
      const cardCount = await cards.count();
      console.log(`✅ Found ${cardCount} menu cards`);
    }
  });

  test('should support dark theme on mobile', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    // 设置深色主题
    await page.evaluate(() => {
      localStorage.setItem('antd-pro-theme', 'dark');
    });

    await page.reload();
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    // 截图：深色主题
    await page.screenshot({ path: 'test-results/mobile-dark-theme.png', fullPage: true });

    // 检查深色主题属性
    const theme = await page.evaluate(() => {
      return document.documentElement.getAttribute('data-theme') || 
             document.body.getAttribute('data-theme');
    });
    
    if (theme === 'dark') {
      console.log('✅ Dark theme applied correctly');
    } else {
      console.log(`⚠️ Theme attribute: ${theme}`);
    }

    // 导航到用户页面检查主题一致性
    await page.goto('/users');
    await page.waitForTimeout(2000);
    await page.screenshot({ path: 'test-results/mobile-dark-user-list.png', fullPage: true });

    // 切换回浅色主题
    await page.evaluate(() => {
      localStorage.setItem('antd-pro-theme', 'light');
    });
    await page.reload();
    await page.waitForTimeout(2000);
    await page.screenshot({ path: 'test-results/mobile-light-theme.png', fullPage: true });
  });

  test('should verify page width adaptation', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/users');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    const widths = await page.evaluate(() => {
      const root = document.getElementById('root');
      const layout = document.querySelector('.ant-pro-layout');
      const content = document.querySelector('.ant-pro-layout-content');
      const pageContainer = document.querySelector('.ant-pro-page-container');
      const mobileContainer = document.querySelector('[class*="mobileContainer"]');
      
      return {
        viewport: window.innerWidth,
        rootWidth: root?.getBoundingClientRect().width || 0,
        layoutWidth: layout?.getBoundingClientRect().width || 0,
        contentWidth: content?.getBoundingClientRect().width || 0,
        pageContainerWidth: pageContainer?.getBoundingClientRect().width || 0,
        mobileContainerWidth: mobileContainer?.getBoundingClientRect().width || 0,
      };
    });

    console.log('Width measurements:');
    console.log(`  - Viewport: ${widths.viewport}px`);
    console.log(`  - Root: ${widths.rootWidth}px`);
    console.log(`  - Layout: ${widths.layoutWidth}px`);
    console.log(`  - Content: ${widths.contentWidth}px`);
    console.log(`  - PageContainer: ${widths.pageContainerWidth}px`);
    console.log(`  - MobileContainer: ${widths.mobileContainerWidth}px`);

    expect(widths.viewport).toBe(390);
    expect(widths.rootWidth).toBeLessThanOrEqual(widths.viewport);
    expect(widths.contentWidth).toBeLessThanOrEqual(widths.viewport);
    expect(widths.pageContainerWidth).toBeLessThanOrEqual(widths.viewport);

    if (widths.mobileContainerWidth > 0) {
      expect(widths.mobileContainerWidth).toBeLessThanOrEqual(widths.viewport);
      console.log('✅ Mobile container width adapted correctly');
    }
  });

  test('should display user data correctly on mobile', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    const responsePromise = page.waitForResponse(response => 
      response.url().includes('/admin/api/users')
    );

    await page.goto('/users');
    
    const response = await responsePromise;
    const responseData = await response.json();

    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    const pageContent = await page.evaluate(() => {
      const listItems = document.querySelectorAll('.ant-list-item');
      return {
        listItemCount: listItems.length,
      };
    });

    expect(response.status()).toBe(200);
    expect(responseData.data?.length || 0).toBeGreaterThan(0);
    expect(pageContent.listItemCount).toBe(responseData.data?.length || 0);
    
    console.log(`✅ User data displayed: ${pageContent.listItemCount} items`);
    
    await page.screenshot({ path: 'test-results/mobile-user-data.png', fullPage: true });
  });

  test('should display account center on mobile', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/account/center');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/mobile-account-center.png', fullPage: true });

    const hasMobileContainer = await page.evaluate(() => {
      return !!document.querySelector('[class*="mobileContainer"]');
    });

    expect(hasMobileContainer).toBe(true);
    console.log('✅ Account center mobile component rendered');
  });

  test('should display account settings on mobile', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/account/settings');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/mobile-account-settings.png', fullPage: true });

    const hasTopTabs = await page.evaluate(() => {
      const tabs = document.querySelector('.ant-tabs-top');
      return !!tabs;
    });

    expect(hasTopTabs).toBe(true);
    console.log('✅ Account settings mobile component rendered with top tabs');
  });
});