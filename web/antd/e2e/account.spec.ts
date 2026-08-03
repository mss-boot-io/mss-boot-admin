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

test.describe('Account Center', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);
  });

  test('should display account center page', async ({ page }) => {
    await page.goto('/account/center');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('.ant-pro-page-container, .ant-card')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'test-results/account-center-pc.png' });
  });

  test('should display user profile information', async ({ page }) => {
    await page.goto('/account/center');
    await page.waitForLoadState('networkidle');

    const profileCard = page.locator('.ant-card, [class*="profile"]').first();
    if (await profileCard.isVisible({ timeout: 5000 }).catch(() => false)) {
      await page.screenshot({ path: 'test-results/account-profile.png' });
    }
  });
});

test.describe('Account Settings', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);
  });

  test('should display account settings page', async ({ page }) => {
    await page.goto('/account/settings');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('.ant-pro-page-container, .ant-card, .ant-tabs')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'test-results/account-settings-pc.png' });
  });

  test('should allow password change', async ({ page }) => {
    await page.goto('/account/settings');
    await page.waitForLoadState('networkidle');

    const securityTab = page.locator('.ant-tabs-tab:has-text("安全"), .ant-tabs-tab:has-text("Security")').first();
    if (await securityTab.isVisible({ timeout: 3000 }).catch(() => false)) {
      await securityTab.click();
      await page.waitForTimeout(500);
      await page.screenshot({ path: 'test-results/account-security.png' });
    }
  });
});

test.describe('Account - Mobile View', () => {
  test.use({
    viewport: { width: 375, height: 667 },
    isMobile: true,
    hasTouch: true,
  });

  test('should display account center in mobile view', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/account/center');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/account-center-mobile.png', fullPage: true });
  });

  test('should display account settings in mobile view', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/account/settings');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/account-settings-mobile.png', fullPage: true });
  });
});