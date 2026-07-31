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

test.describe('Login', () => {
  test('should login successfully with admin account via API', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await expect(page).not.toHaveURL(/\/user\/login/, { timeout: 10000 });
    await expect(page.locator('.ant-pro-layout-content')).toBeVisible({ timeout: 10000 });
  });

  test('should login successfully via UI form', async ({ page }) => {
    await page.context().clearCookies();

    await page.goto('/user/login');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    const usernameInput = page.locator('.ant-input').first();
    await usernameInput.waitFor({ state: 'visible', timeout: 10000 });
    await usernameInput.click();
    await usernameInput.fill('admin');

    const passwordInput = page
      .locator('.ant-input-password .ant-input, input[type="password"]')
      .first();
    await passwordInput.click();
    await passwordInput.fill('123456');

    const loginButton = page.locator('button.ant-btn-primary').first();
    await loginButton.click();

    await expect(page).not.toHaveURL(/\/user\/login/, { timeout: 15000 });
    await expect(page.locator('.ant-pro-layout-content')).toBeVisible({ timeout: 10000 });
  });

  test('should show error with wrong credentials', async ({ page }) => {
    await page.context().clearCookies();

    const response = await page.request.post('http://localhost:8080/admin/api/user/login', {
      headers: { 'Content-Type': 'application/json' },
      data: { username: 'wronguser', password: 'wrongpass' },
    });

    expect(response.status()).toBe(401);
  });
});
