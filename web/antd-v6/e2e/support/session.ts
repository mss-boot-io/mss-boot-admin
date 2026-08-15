import { type APIResponse, expect, type Page } from '@playwright/test';

export const APP_BASE_URL = process.env.MSS_V6_BASE_URL ?? 'http://127.0.0.1:8001';
export const API_BASE_URL = (process.env.MSS_E2E_API_URL ?? `${APP_BASE_URL}/admin/api`).replace(
  /\/$/,
  '',
);
export const BACKEND_API_BASE_URL = (
  process.env.MSS_E2E_BACKEND_API_URL ?? 'http://127.0.0.1:18080/admin/api'
).replace(/\/$/, '');
export const ROOT_USERNAME = process.env.MSS_E2E_USERNAME ?? 'admin';
export const ROOT_PASSWORD = process.env.MSS_E2E_PASSWORD ?? '123456';

export interface SessionCredentials {
  username: string;
  password: string;
}

export async function readJSON(response: APIResponse): Promise<Record<string, unknown>> {
  if (!response.ok()) {
    throw new Error(`${response.url()} returned ${response.status()}: ${await response.text()}`);
  }
  return response.json() as Promise<Record<string, unknown>>;
}

export async function login(
  page: Page,
  credentials: SessionCredentials = { username: ROOT_USERNAME, password: ROOT_PASSWORD },
) {
  const response = await page.request.post(`${API_BASE_URL}/user/session/login`, {
    data: credentials,
    headers: { Origin: APP_BASE_URL },
  });
  const body = await readJSON(response);
  expect(body.code).toBe(200);
  expect(body).not.toHaveProperty('token');
  expect(body).not.toHaveProperty('accessToken');
  const cookies = await page.context().cookies();
  expect(cookies.find((cookie) => cookie.name === 'mss_admin_session')?.httpOnly).toBe(true);
  expect(cookies.find((cookie) => cookie.name === 'mss_csrf')?.value).toBeTruthy();
}

export async function csrfHeaders(page: Page) {
  const cookies = await page.context().cookies();
  const csrf = cookies.find((cookie) => cookie.name === 'mss_csrf')?.value;
  if (!csrf) throw new Error('V6 CSRF cookie is missing');
  return { Origin: APP_BASE_URL, 'X-CSRF-Token': csrf };
}

export async function setLocale(page: Page, locale: 'en-US' | 'zh-CN') {
  await page.addInitScript((value) => window.localStorage.setItem('umi_locale', value), locale);
}

export async function expectNoDocumentOverflow(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      })),
    )
    .toMatchObject({
      clientWidth: expect.any(Number),
      scrollWidth: expect.any(Number),
    });
  const overflow = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(1);
}
