import { randomUUID } from 'node:crypto';
import { expect, type Page, test } from '@playwright/test';
import {
  API_BASE_URL,
  APP_BASE_URL,
  BACKEND_API_BASE_URL,
  csrfHeaders,
  login,
  readJSON,
  setLocale,
} from './support/session';

function menuContainsPath(value: unknown, expectedPath: string): boolean {
  if (!Array.isArray(value)) return false;
  return value.some((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return false;
    const item = entry as Record<string, unknown>;
    return item.path === expectedPath || menuContainsPath(item.children, expectedPath);
  });
}

async function createFinanceUser(rootPage: Page) {
  const roles = await readJSON(
    await rootPage.request.get(`${BACKEND_API_BASE_URL}/roles?current=1&pageSize=100`),
  );
  const financeRole = (Array.isArray(roles.data) ? roles.data : []).find(
    (entry) =>
      entry &&
      typeof entry === 'object' &&
      !Array.isArray(entry) &&
      (entry as Record<string, unknown>).name === 'finance',
  ) as Record<string, unknown> | undefined;
  const roleID = typeof financeRole?.id === 'string' ? financeRole.id : '';
  expect(roleID, 'the Supplier migration must seed the finance role').not.toBe('');

  const suffix = randomUUID().replaceAll('-', '').slice(0, 10);
  const credentials = {
    username: `e2efin${suffix}`,
    password: 'E2eFinance123!',
  };
  const response = await rootPage.request.post(`${BACKEND_API_BASE_URL}/users`, {
    data: {
      email: `${credentials.username}@example.test`,
      name: 'E2E Finance',
      password: credentials.password,
      roleID,
      status: 'enabled',
      username: credentials.username,
    },
    headers: await csrfHeaders(rootPage),
  });
  expect(response.status()).toBe(201);
  const user = await readJSON(response);
  expect(typeof user.id === 'string' ? user.id : '').not.toBe('');
  return credentials;
}

test.describe('V6 authorization boundaries', () => {
  test('@permission anonymous direct navigation requires a server session', async ({ page }) => {
    await setLocale(page, 'en-US');
    await page.goto('/suppliers');
    await expect(page.getByPlaceholder('Username')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByPlaceholder('Password')).toBeVisible();
    await expect(page).toHaveURL(/\/user\/login\?redirect=/);
  });

  test('@permission retired Supplier example stays absent from finance menu, route, and API', async ({
    browser,
    page,
  }) => {
    test.setTimeout(90_000);
    const rootContext = await browser.newContext();
    const rootPage = await rootContext.newPage();
    await login(rootPage);
    const credentials = await createFinanceUser(rootPage);

    try {
      await setLocale(page, 'en-US');
      await login(page, credentials);

      const identity = await readJSON(await page.request.get(`${API_BASE_URL}/user/userInfo`));
      expect((identity.role as Record<string, unknown> | undefined)?.root).toBe(false);

      const menu = await page.request.get(`${API_BASE_URL}/menu/authorize`);
      const authorizedMenu = await menu.json();
      expect(menu.ok()).toBe(true);
      expect(menuContainsPath(authorizedMenu, '/suppliers')).toBe(false);
      expect(menuContainsPath(authorizedMenu, '/security/online-sessions')).toBe(false);

      await page.goto('/suppliers');
      await expect(page.getByText('403', { exact: true })).toBeVisible();
      expect(page.url().startsWith(APP_BASE_URL)).toBe(true);

      const deniedList = await page.request.get(`${API_BASE_URL}/suppliers`);
      expect(deniedList.status()).toBe(403);

      const deniedCreate = await page.request.post(`${API_BASE_URL}/suppliers`, {
        data: {
          code: `DENIED_${randomUUID().replaceAll('-', '').slice(0, 8).toUpperCase()}`,
          contactEmail: 'denied@example.test',
          contactName: 'Denied',
          country: 'CN',
          creditLevel: 'normal',
          enabled: true,
          name: 'Denied supplier',
        },
        headers: await csrfHeaders(page),
      });
      expect(deniedCreate.status()).toBe(403);

      const deniedSessions = await page.request.get(`${API_BASE_URL}/online-sessions`);
      expect(deniedSessions.status()).toBe(403);

      await page.goto('/security/online-sessions');
      await expect(page.getByText('403', { exact: true })).toBeVisible();
      expect(page.url().startsWith(APP_BASE_URL)).toBe(true);
    } finally {
      await page.close();
      // The backend process owns an isolated SQLite database that is deleted
      // before every E2E run. Avoid the legacy User AfterDelete hook here: it
      // writes through a second connection while the delete transaction holds
      // SQLite's writer lock and can poison later browser-project logins.
      await rootContext.close();
    }
  });
});
