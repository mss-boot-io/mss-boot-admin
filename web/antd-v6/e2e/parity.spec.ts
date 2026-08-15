import { expect, test } from '@playwright/test';
import { expectNoDocumentOverflow, login, setLocale } from './support/session';

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
    page.on('console', (message) => {
      if (message.type() === 'warning' || message.type() === 'error') {
        consoleViolations.push(`${message.type()}: ${message.text()}`);
      }
    });

    await setLocale(page, expected.locale);
    await login(page);

    await page.goto('/workplace');
    await expect(page.getByText(expected.workplace, { exact: true })).toBeVisible();
    await expect(page.getByText(expected.monitor, { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: expected.switchLanguage })).toBeVisible();
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
    expect(consoleViolations).toEqual([]);
  });
}
