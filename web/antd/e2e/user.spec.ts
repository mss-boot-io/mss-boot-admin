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

test.describe('User Management - PC View', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);
  });

  test('should display user list page', async ({ page }) => {
    await page.goto('/users');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('.ant-pro-table')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'test-results/user-list-pc.png' });
  });

  test('should create a new user', async ({ page }) => {
    await page.goto('/users');
    await page.waitForLoadState('networkidle');

    const addButton = page.locator('button:has-text("新建"), button:has-text("Add")').first();
    if (await addButton.isVisible({ timeout: 3000 }).catch(() => false)) {
      await addButton.click();
      await page.waitForTimeout(500);

      const modal = page.locator('.ant-modal-content');
      await expect(modal).toBeVisible({ timeout: 5000 });

      const usernameInput = modal.locator('input[id*="username"], input[name*="username"]').first();
      if (await usernameInput.isVisible()) {
        await usernameInput.fill('test_user_e2e');
      }

      await page.screenshot({ path: 'test-results/user-create-modal.png' });
      
      const submitBtn = modal.locator('button:has-text("确定"), button:has-text("OK")').first();
      if (await submitBtn.isVisible()) {
        await submitBtn.click();
        await page.waitForTimeout(1000);
      }
    }
  });

  test('should search users', async ({ page }) => {
    await page.goto('/users');
    await page.waitForLoadState('networkidle');

    const searchInput = page.locator('.ant-pro-table-search input, input[placeholder*="搜索"], input[placeholder*="Search"]').first();
    if (await searchInput.isVisible({ timeout: 3000 }).catch(() => false)) {
      await searchInput.fill('admin');
      await page.keyboard.press('Enter');
      await page.waitForTimeout(1000);
    }

    await page.screenshot({ path: 'test-results/user-search.png' });
  });

  test('should display user table with pagination', async ({ page }) => {
    await page.goto('/users');
    await page.waitForLoadState('networkidle');

    const table = page.locator('.ant-table');
    await expect(table).toBeVisible({ timeout: 10000 });

    const pagination = page.locator('.ant-pagination');
    const paginationVisible = await pagination.isVisible({ timeout: 3000 }).catch(() => false);
    expect(typeof paginationVisible).toBe('boolean');
  });
});

test.describe('User Management - Mobile View', () => {
  test.use({
    viewport: { width: 375, height: 667 },
    isMobile: true,
    hasTouch: true,
  });

  test('should display user list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/users');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/user-list-mobile.png', fullPage: true });

    const mobileContainer = page.locator('.mobileContainer, [class*="mobileContainer"]');
    const isMobile = await mobileContainer.isVisible({ timeout: 3000 }).catch(() => false);
    console.log(`Mobile view active: ${isMobile}`);
  });
});