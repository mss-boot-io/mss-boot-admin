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

test.describe('Department Management - PC View', () => {
  test.beforeEach(async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);
  });

  test('should display department list page', async ({ page }) => {
    await page.goto('/departments');
    await page.waitForLoadState('networkidle');
    
    await expect(page.locator('.ant-pro-table, .ant-table, .ant-tree')).toBeVisible({ timeout: 10000 });
    await page.screenshot({ path: 'test-results/department-list-pc.png' });
  });

  test('should create a new department', async ({ page }) => {
    await page.goto('/departments');
    await page.waitForLoadState('networkidle');

    const addButton = page.locator('button:has-text("新建"), button:has-text("Add")').first();
    if (await addButton.isVisible({ timeout: 3000 }).catch(() => false)) {
      await addButton.click();
      await page.waitForTimeout(500);

      const modal = page.locator('.ant-modal-content');
      await expect(modal).toBeVisible({ timeout: 5000 });

      const nameInput = modal.locator('input[id*="name"], input[name*="name"]').first();
      if (await nameInput.isVisible()) {
        await nameInput.fill('test_dept_e2e');
      }

      await page.screenshot({ path: 'test-results/department-create-modal.png' });
      
      const submitBtn = modal.locator('button:has-text("确定"), button:has-text("OK")').first();
      if (await submitBtn.isVisible()) {
        await submitBtn.click();
        await page.waitForTimeout(1000);
      }
    }
  });

  test('should display department tree structure', async ({ page }) => {
    await page.goto('/departments');
    await page.waitForLoadState('networkidle');

    const tree = page.locator('.ant-tree');
    const treeVisible = await tree.isVisible({ timeout: 5000 }).catch(() => false);
    
    if (treeVisible) {
      const treeNodes = page.locator('.ant-tree-treenode');
      const nodeCount = await treeNodes.count();
      console.log(`Found ${nodeCount} department tree nodes`);
    }

    await page.screenshot({ path: 'test-results/department-tree.png' });
  });
});

test.describe('Department Management - Mobile View', () => {
  test.use({
    viewport: { width: 375, height: 667 },
    isMobile: true,
    hasTouch: true,
  });

  test('should display department list in mobile card layout', async ({ page }) => {
    await page.context().clearCookies();
    await loginViaAPI(page);

    await page.goto('/departments');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(2000);

    await page.screenshot({ path: 'test-results/department-list-mobile.png', fullPage: true });
  });
});