import { expect, test } from '@playwright/test';
import { APP_BASE_URL, expectNoDocumentOverflow, login, setLocale } from './support/session';

const localeExpectations = [
  {
    locale: 'en-US' as const,
    workplace: 'Welcome, admin',
    monitor: 'Service monitor',
    account: 'Account center',
    settings: 'Personal settings',
    suppliers: 'Suppliers list',
    switchLanguage: 'Switch language',
    systemMenu: 'System settings',
    platformMenu: 'Platform administration',
  },
  {
    locale: 'zh-CN' as const,
    workplace: '欢迎，admin',
    monitor: '服务监控',
    account: '账户中心',
    settings: '个人设置',
    suppliers: '供应商管理列表',
    switchLanguage: '切换语言',
    systemMenu: '系统设置',
    platformMenu: '平台管理',
  },
];

for (const expected of localeExpectations) {
  test(`@parity ${expected.locale} retained pages are responsive and deprecation-free`, async ({
    page,
  }, testInfo) => {
    test.setTimeout(90_000);
    const consoleViolations: string[] = [];
    const responseViolations: string[] = [];
    page.on('console', (message) => {
      if (message.type() === 'warning' || message.type() === 'error') {
        const location = message.location();
        const source = location.url
          ? ` (${location.url}:${location.lineNumber}:${location.columnNumber})`
          : '';
        consoleViolations.push(`${message.type()}: ${message.text()}${source}`);
      }
    });
    page.on('response', (response) => {
      if (response.status() >= 400) {
        responseViolations.push(
          `${response.status()} ${response.request().method()} ${response.url()}`,
        );
      }
    });

    await setLocale(page, expected.locale);
    await login(page);

    await page.goto('/workplace');
    await expect(page.getByText(expected.workplace, { exact: true })).toBeVisible();
    await expect(page.getByText(expected.monitor, { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: expected.switchLanguage })).toBeVisible();
    const logo = page.locator('img[src="/logo.svg"]').first();
    await expect(logo).toBeVisible();
    await expect
      .poll(() => logo.evaluate((image: HTMLImageElement) => image.naturalWidth))
      .toBeGreaterThan(0);
    if (testInfo.project.name === 'chromium-desktop') {
      const systemMenu = page.getByRole('menuitem', { name: new RegExp(expected.systemMenu) });
      const platformMenu = page.getByRole('menuitem', { name: new RegExp(expected.platformMenu) });
      await expect(systemMenu).toBeVisible();
      await expect(systemMenu.getByRole('img', { name: 'setting' })).toBeVisible();
      await expect(platformMenu).toBeVisible();
      await expect(platformMenu.getByRole('img', { name: 'audit' })).toBeVisible();
    }
    await expectNoDocumentOverflow(page);

    await page.goto('/account/center');
    await expect(page.getByText(expected.account, { exact: true }).first()).toBeVisible();
    await expectNoDocumentOverflow(page);

    await page.goto('/account/settings');
    await expect(page.getByText(expected.settings, { exact: true }).first()).toBeVisible();
    await expectNoDocumentOverflow(page);

    const supplierList = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === '/admin/api/suppliers',
    );
    await page.goto('/suppliers');
    expect((await supplierList).ok()).toBe(true);
    await expect(page.getByText(expected.suppliers, { exact: true })).toBeVisible();
    await expectNoDocumentOverflow(page);

    await page.keyboard.press('Tab');
    await expect(page.locator(':focus')).not.toHaveJSProperty('tagName', 'BODY');
    expect(responseViolations).toEqual([]);
    expect(consoleViolations).toEqual([]);
  });
}

test('@parity OAuth login preserves an attempt-bound safe deep link', async ({
  page,
}, testInfo) => {
  test.skip(
    testInfo.project.name !== 'chromium-desktop',
    'one browser covers the redirect contract',
  );
  const attemptID = 'attempt-deep-link';
  const redirectKey = `mss.antd-v6.auth.oauth-redirect.v1:${attemptID}`;

  await page.route('**/admin/api/app-configs/profile', (route) =>
    route.fulfill({
      json: {
        base: { websiteName: 'MSS' },
        security: { githubEnabled: true },
        theme: { _meta: { v: 1, scope: 'application', revision: '3' } },
      },
    }),
  );
  await page.route('**/admin/api/user/auth-cookie/clear', (route) =>
    route.fulfill({ status: 204 }),
  );
  await page.route('**/admin/api/user/session/oauth2/authorize', (route) =>
    route.fulfill({
      json: {
        authorizeURL: `${APP_BASE_URL}/oauth-provider-placeholder`,
        attemptID,
        expiresAt: '2099-08-16T00:05:00Z',
      },
    }),
  );
  await page.route('**/oauth-provider-placeholder', (route) =>
    route.fulfill({ contentType: 'text/html', body: '<main>provider placeholder</main>' }),
  );
  await page.route('**/admin/api/user/session/github/callback', (route) =>
    route.fulfill({
      status: 201,
      json: {
        code: 200,
        provider: 'github',
        intent: 'login',
        attemptID,
        expire: '2099-08-16T12:00:00Z',
      },
    }),
  );
  await page.route('**/admin/api/user/userInfo', (route) =>
    route.fulfill({ json: { id: 'oauth-user', username: 'oauth-user' } }),
  );
  await page.route(/\/users(?:\?.*)?$/, (route) =>
    route.fulfill({ contentType: 'text/html', body: '<main>deep-link target</main>' }),
  );

  await page.goto('/user/login?redirect=%2Fusers%3Fpage%3D2%23details');
  await page.getByRole('button', { name: /GitHub/ }).click();
  await page.waitForURL('**/oauth-provider-placeholder');
  await expect
    .poll(() => page.evaluate((key) => window.sessionStorage.getItem(key), redirectKey))
    .toContain('/users?page=2#details');

  await page.goto('/user/callback/github?code=provider-code&state=opaque-state');
  await page.waitForURL('**/users?page=2#details');
  expect(await page.evaluate((key) => window.sessionStorage.getItem(key), redirectKey)).toBeNull();
  const sessionMetadata = await page.evaluate(() =>
    window.localStorage.getItem('mss.antd-v6.auth.session.v1'),
  );
  expect(sessionMetadata).toContain('expiresAt');
  expect(sessionMetadata).not.toContain('token');
});
