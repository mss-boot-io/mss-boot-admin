import { randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';
import {
  API_BASE_URL,
  csrfHeaders,
  expectNoDocumentOverflow,
  login,
  readJSON,
  setLocale,
} from './support/session';

test('@operations bounded operational pages work through the browser contract', async ({
  page,
  request,
}) => {
  test.setTimeout(120_000);
  const suffix = randomUUID().replaceAll('-', '').slice(0, 10);
  const taskName = `E2E health ${suffix}`;
  const noticeTitle = `E2E notice ${suffix}`;
  const configName = `e2e-${suffix}`;
  const configContent = `{"feature":"${suffix}"}`;
  const created: { config?: string; notice?: string; task?: string } = {};
  let cleanupHeaders: Record<string, string> | undefined;
  const consoleViolations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'warning' || message.type() === 'error') {
      consoleViolations.push(`${message.type()}: ${message.text()}`);
    }
  });

  await setLocale(page, 'en-US');
  await login(page);

  try {
    const headers = await csrfHeaders(page);
    cleanupHeaders = {
      ...headers,
      Cookie: (await page.context().cookies())
        .map((cookie) => `${cookie.name}=${cookie.value}`)
        .join('; '),
    };
    const taskResponse = await page.request.post(`${API_BASE_URL}/tasks`, {
      data: {
        name: taskName,
        provider: 'default',
        spec: '0 */15 * * * *',
        protocol: 'https',
        endpoint: 'example.test/health',
        method: 'GET',
        timeout: 30,
        remark: 'browser qualification',
      },
      headers,
    });
    expect(taskResponse.status()).toBe(201);
    const task = await readJSON(taskResponse);
    created.task = typeof task.id === 'string' ? task.id : undefined;
    expect(created.task).toBeTruthy();
    expect(task.status).toBe('disabled');

    const noticeResponse = await page.request.post(`${API_BASE_URL}/notices`, {
      data: {
        title: noticeTitle,
        description: 'Owner-scoped browser qualification notice',
        read: true,
        type: 'message',
        userID: 'forged-owner',
      },
      headers,
    });
    expect(noticeResponse.status()).toBe(201);
    const notice = await readJSON(noticeResponse);
    created.notice = typeof notice.id === 'string' ? notice.id : undefined;
    expect(created.notice).toBeTruthy();
    expect(notice.read).toBe(false);
    expect(notice.userID).not.toBe('forged-owner');

    const configResponse = await page.request.post(`${API_BASE_URL}/system-configs`, {
      data: { name: configName, ext: 'json', content: configContent, remark: 'E2E only' },
      headers,
    });
    expect(configResponse.status()).toBe(201);
    const config = await readJSON(configResponse);
    created.config = typeof config.id === 'string' ? config.id : undefined;
    expect(created.config).toBeTruthy();

    const taskList = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === '/admin/api/tasks',
    );
    await page.goto('/task');
    expect((await taskList).ok()).toBe(true);
    await expect(
      page.getByRole('heading', { level: 1, name: 'Task scheduler', exact: true }),
    ).toBeVisible();
    await expect(page.getByText(taskName, { exact: true })).toBeVisible();
    await expectNoDocumentOverflow(page);

    const noticeList = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === '/admin/api/notices',
    );
    await page.goto('/notice');
    expect((await noticeList).ok()).toBe(true);
    const noticeRecord =
      (page.viewportSize()?.width ?? Number.POSITIVE_INFINITY) < 768
        ? page.getByRole('listitem').filter({ hasText: noticeTitle })
        : page.getByRole('row').filter({ hasText: noticeTitle });
    await expect(noticeRecord).toBeVisible();
    const markRead = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' &&
        new URL(response.url()).pathname === `/admin/api/notice/read/${created.notice}`,
    );
    await noticeRecord.getByRole('button', { name: /Mark read$/ }).click();
    expect((await markRead).ok()).toBe(true);
    await expect(noticeRecord.getByRole('button', { name: /Mark read$/ })).toHaveCount(0);
    await expectNoDocumentOverflow(page);

    const loginLogs = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === '/admin/api/audit-logs/login',
    );
    await page.goto('/log');
    expect((await loginLogs).ok()).toBe(true);
    await expect(page.getByText('Logs', { exact: true }).first()).toBeVisible();
    const runtimeLogs = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === '/admin/api/logs',
    );
    await page.getByRole('tab', { name: 'Runtime logs' }).click();
    expect((await runtimeLogs).ok()).toBe(true);
    await expectNoDocumentOverflow(page);

    const configList = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === '/admin/api/system-configs',
    );
    await page.goto('/system-config');
    expect((await configList).ok()).toBe(true);
    await expect(page.getByText(configName, { exact: true })).toBeVisible();
    const configDetail = page.waitForResponse(
      (response) =>
        response.request().method() === 'GET' &&
        new URL(response.url()).pathname === `/admin/api/system-configs/${created.config}`,
    );
    await page.getByRole('button', { name: configName, exact: true }).click();
    expect((await configDetail).ok()).toBe(true);
    await expect(page.getByText(configContent, { exact: true })).toBeVisible();
    await expectNoDocumentOverflow(page);

    expect(consoleViolations).toEqual([]);
  } finally {
    for (const [resource, id] of [
      ['tasks', created.task],
      ['notices', created.notice],
      ['system-configs', created.config],
    ] as const) {
      if (id && cleanupHeaders) {
        const response = await request.delete(`${API_BASE_URL}/${resource}/${id}`, {
          headers: cleanupHeaders,
        });
        expect(response.status(), `cleanup ${resource}/${id}`).toBe(204);
      }
    }
  }
});
