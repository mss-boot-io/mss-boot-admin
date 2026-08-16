import { randomUUID } from 'node:crypto';
import { expect, type Page, test } from '@playwright/test';
import {
  API_BASE_URL,
  APP_BASE_URL,
  BACKEND_API_BASE_URL,
  csrfHeaders,
  expectNoDocumentOverflow,
  login,
  readJSON,
  setLocale,
} from './support/session';

async function createCredentialSelfServiceUser(rootPage: Page) {
  const roles = await readJSON(
    await rootPage.request.get(`${BACKEND_API_BASE_URL}/roles?current=1&pageSize=100`),
  );
  const role = (Array.isArray(roles.data) ? roles.data : []).find(
    (entry) =>
      entry &&
      typeof entry === 'object' &&
      !Array.isArray(entry) &&
      (entry as Record<string, unknown>).name === 'user' &&
      (entry as Record<string, unknown>).root !== true,
  ) as Record<string, unknown> | undefined;
  const roleID = typeof role?.id === 'string' ? role.id : '';
  expect(roleID, 'the migration must seed the non-root self-service role').not.toBe('');

  const suffix = randomUUID().replaceAll('-', '').slice(0, 10);
  const credentials = {
    username: `e2esecurity${suffix}`,
    password: 'E2eSecurity123!',
  };
  const response = await rootPage.request.post(`${BACKEND_API_BASE_URL}/users`, {
    data: {
      email: `${credentials.username}@example.test`,
      name: 'E2E Credential Owner',
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

test('@settings application settings expose every migrated V6 section without warnings', async ({
  page,
}) => {
  test.setTimeout(90_000);
  const consoleViolations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'warning' || message.type() === 'error') {
      consoleViolations.push(`${message.type()}: ${message.text()}`);
    }
  });

  await setLocale(page, 'en-US');
  await login(page);
  await page.goto('/app-config');
  await expect(page.getByText('Application settings', { exact: true }).first()).toBeVisible();
  await expect(page.getByRole('tab', { name: 'Basics', exact: true })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  await expectNoDocumentOverflow(page);

  for (const [name, key] of [
    ['Security and sign-in', 'security'],
    ['Upload policy', 'storage'],
    ['Email service', 'email'],
    ['Theme settings', 'theme'],
  ] as const) {
    // Every section is query-owned and independently deep-linkable. Direct
    // navigation also covers tabs clipped by Ant Design's mobile overflow.
    await page.goto(`/app-config?tab=${key}`);
    await expect(page).toHaveURL(new RegExp(`[?&]tab=${key}(?:&|$)`));
    await expect(page.getByRole('tab', { name, exact: true })).toHaveAttribute(
      'aria-selected',
      'true',
    );
    await expectNoDocumentOverflow(page);
  }

  expect(consoleViolations).toEqual([]);
});

test('@settings password rotation is user-owned and OAuth disconnect stays proof-gated', async ({
  browser,
  page,
  request,
}) => {
  test.setTimeout(120_000);
  const rootContext = await browser.newContext();
  const rootPage = await rootContext.newPage();
  const consoleViolations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'warning' || message.type() === 'error') {
      consoleViolations.push(`${message.type()}: ${message.text()}`);
    }
  });

  await setLocale(rootPage, 'en-US');
  await login(rootPage);
  const credentials = await createCredentialSelfServiceUser(rootPage);
  const replacementPassword = 'E2eRotated456!';

  try {
    await setLocale(page, 'en-US');
    await login(page, credentials);
    await page.goto('/account/settings?tab=connections');
    await expect(
      page.getByText('Disconnect requires recent identity proof', { exact: true }).first(),
    ).toBeVisible();
    await expect(
      page.getByText('No external account providers are currently enabled.', { exact: true }),
    ).toBeVisible();

    // Ant Design intentionally clips and scrolls the tab strip on narrow
    // viewports. Exercise the query-owned deep link directly so mobile does
    // not depend on a partially off-screen tab hit target.
    await page.goto('/account/settings?tab=security');
    await expect(page.getByText('Your password is managed by you', { exact: true })).toBeVisible();
    await expect(page.getByText('Proof active', { exact: true })).toBeVisible();
    await expect(
      page.getByText('A password change signs out every device', { exact: true }),
    ).toBeVisible();
    await page.getByRole('textbox', { name: /New password/ }).fill(replacementPassword);
    await page.getByRole('textbox', { name: /Confirm new password/ }).fill(replacementPassword);
    await page.getByRole('button', { name: /Change password and sign out everywhere/ }).click();
    await expect(page).toHaveURL(/\/user\/login\?passwordChanged=success$/);
    await expect(page.getByPlaceholder('Username')).toBeVisible();
    expect(
      (await page.context().cookies()).some((cookie) => cookie.name === 'mss_admin_session'),
    ).toBe(false);

    const oldPassword = await request.post(`${API_BASE_URL}/user/session/login`, {
      data: credentials,
      headers: { Origin: APP_BASE_URL },
    });
    expect(oldPassword.status()).toBe(401);
    const newPassword = await request.post(`${API_BASE_URL}/user/session/login`, {
      data: { ...credentials, password: replacementPassword },
      headers: { Origin: APP_BASE_URL },
    });
    expect(newPassword.status()).toBe(200);
    const session = await readJSON(newPassword);
    expect(session.code).toBe(200);
    expect(session).not.toHaveProperty('token');
    expect(consoleViolations).toEqual([]);
  } finally {
    await rootContext.close();
  }
});
