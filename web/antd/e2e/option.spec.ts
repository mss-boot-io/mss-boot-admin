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

test.describe('Option Management - PC View', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);
  });

  test('should display option list page', async ({ page }) => {
    await page.goto('/options');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('.ant-pro-table, .ant-table')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'test-results/option-list-pc.png' });
  });

  test('should create a new option', async ({ page }) => {
    await page.goto('/options');
    await page.waitForLoadState('networkidle');

    const addButton = page.locator('button:has-text("新建"), button:has-text("Add")').first();
    if (await addButton.isVisible({ timeout: 3000 }).catch(() => false)) {
      await addButton.click();
      await page.waitForTimeout(500);

      const modal = page.locator('.ant-modal-content');
      await expect(modal).toBeVisible({ timeout: 5000 });

      const keyInput = modal.locator('input[id*="key"], input[id*="name"]').first();
      if (await keyInput.isVisible()) {
        await keyInput.fill('test_option_e2e');
      }

      await page.screenshot({ path: 'test-results/option-create-modal.png' });
      
      const submitBtn = modal.locator('button:has-text("确定"), button:has-text("OK")').first();
      if (await submitBtn.isVisible()) {
        await submitBtn.click();
        await page.waitForTimeout(1000);
      }
    }
  });

  test('should search options', async ({ page }) => {
    await page.goto('/options');
    await page.waitForLoadState('networkidle');

    const searchInput = page.locator('.ant-pro-table-search input, input[placeholder*="搜索"], input[placeholder*="Search"]').first();
    if (await searchInput.isVisible({ timeout: 3000 }).catch(() => false)) {
      await searchInput.fill('site');
      await page.keyboard.press('Enter');
      await page.waitForTimeout(1000);
    }

    await page.screenshot({ path: 'test-results/option-search.png' });
  });
});

test.describe('Option Management - Mobile View', () => {
  test.use({
    viewport: { width: 375, height: 667 },
    isMobile: true,
    hasTouch: true,
  });

  test('should display option list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/options');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/option-list-mobile.png', fullPage: true });
  });
});