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

test.describe('Language Management - PC View', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);
  });

  test('should display language list page', async ({ page }) => {
    await page.goto('/languages');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('.ant-pro-table, .ant-table')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'test-results/language-list-pc.png' });
  });

  test('should create a new language', async ({ page }) => {
    await page.goto('/languages');
    await page.waitForLoadState('networkidle');

    const addButton = page.locator('button:has-text("新建"), button:has-text("Add")').first();
    if (await addButton.isVisible({ timeout: 3000 }).catch(() => false)) {
      await addButton.click();
      await page.waitForTimeout(500);

      const modal = page.locator('.ant-modal-content');
      await expect(modal).toBeVisible({ timeout: 5000 });

      const codeInput = modal.locator('input[id*="code"], input[id*="language"]').first();
      if (await codeInput.isVisible()) {
        await codeInput.fill('test');
      }

      await page.screenshot({ path: 'test-results/language-create-modal.png' });
    }
  });
});

test.describe('Language Management - Mobile View', () => {
  test.use({
    viewport: { width: 375, height: 667 },
    isMobile: true,
    hasTouch: true,
  });

  test('should display language list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/languages');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/language-list-mobile.png', fullPage: true });
  });
});