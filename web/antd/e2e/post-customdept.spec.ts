import { test, expect } from '@playwright/test';

async function loginViaAPI(page: any) {
  const response = await page.request.post('http://localhost:8080/admin/api/user/login', {
    headers: { 'Content-Type': 'application/json' },
    data: { username: 'admin', password: '123456' },
  });

  const data = await response.json();
  if (data.code !== 200 || !data.token) {
    throw new Error('Login failed: ' + JSON.stringify(data));
  }

  await page.goto('/');
  await page.waitForLoadState('domcontentloaded');

  await page.evaluate((token: string) => {
    localStorage.setItem('token', token);
    localStorage.setItem('token.expire', new Date(Date.now() + 86400000).toISOString());
  }, data.token);

  await page.reload();
  await page.waitForLoadState('networkidle');
}

test.describe('Post Management', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);
  });

  test('should create post with all data scope', async ({ page }) => {
    await page.goto('/posts/create');
    await page.waitForLoadState('networkidle');

    const inputs = page.locator('.ant-input');
    await inputs.first().waitFor({ state: 'visible', timeout: 10000 });

    const timestamp = Date.now();
    await inputs.nth(0).fill(`Test Post ${timestamp}`);
    await inputs.nth(1).fill(`test_post_${timestamp}`);

    const dataScopeSelect = page.locator('.ant-select').nth(2);
    await dataScopeSelect.click();
    await page.waitForTimeout(500);

    await page.click('.ant-select-item:has-text("all")');

    await page.click('button:has-text("Submit")');

    await expect(page).toHaveURL(/post/, { timeout: 10000 });
  });

  test('should create post with customDept data scope', async ({ page }) => {
    await page.goto('/posts/create');
    await page.waitForLoadState('networkidle');

    const inputs = page.locator('.ant-input');
    await inputs.first().waitFor({ state: 'visible', timeout: 10000 });

    const timestamp = Date.now();
    await inputs.nth(0).fill(`CustomDept Post ${timestamp}`);
    await inputs.nth(1).fill(`custom_dept_post_${timestamp}`);

    const dataScopeSelect = page.locator('.ant-select').nth(2);
    await dataScopeSelect.click();
    await page.waitForTimeout(500);

    await page.click('.ant-select-item:has-text("custom department")');

    await page.click('button:has-text("Submit")');

    await expect(page).toHaveURL(/post/, { timeout: 10000 });
  });
});
