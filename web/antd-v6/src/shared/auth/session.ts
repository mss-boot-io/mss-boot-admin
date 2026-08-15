import { history, request } from '@umijs/max';
import { getRequestStatus } from '../api/client';
import { normalizeCurrentUser } from './identity';
import type { CurrentUser } from './types';

export const LOGIN_PATH = '/user/login';
export const BROWSER_SESSION_LOGIN_ENDPOINT = '/user/session/login';
export const BROWSER_SESSION_REFRESH_ENDPOINT = '/user/session/refresh-token';
export const BROWSER_SESSION_LOGOUT_ENDPOINT = '/online-sessions/logout';
export const BROWSER_SESSION_CLEAR_ENDPOINT = '/user/auth-cookie/clear';

export interface BrowserSessionResponse {
  code?: number;
  expire?: string;
}

export interface BrowserSessionCredentials {
  username: string;
  password: string;
}

const PUBLIC_PATHS = new Set([LOGIN_PATH, '/user/register', '/user/forget']);
const OAUTH_CALLBACK_PATH = /^\/user\/(?:oauth\/)?callback\/[^/]+$/;

export function isPublicPath(pathname: string): boolean {
  return PUBLIC_PATHS.has(pathname) || OAUTH_CALLBACK_PATH.test(pathname);
}

export function currentLocationRedirect(): string {
  const { pathname, search, hash } = history.location;
  return pathname + search + hash;
}

export function redirectToLogin(): void {
  const redirect = encodeURIComponent(currentLocationRedirect());
  if (history.location.pathname !== LOGIN_PATH) {
    history.replace(`${LOGIN_PATH}?redirect=${redirect}`);
  }
}

export async function fetchCurrentUser(): Promise<CurrentUser | undefined> {
  try {
    const value = await request<unknown>('/user/userInfo', {
      method: 'GET',
      skipErrorHandler: true,
      skipAuthorizationRefresh: true,
    });
    return normalizeCurrentUser(value);
  } catch (error) {
    if (getRequestStatus(error) === 401) return undefined;
    throw error;
  }
}

export function assertNoBrowserCredential(value: unknown): void {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return;
  for (const field of ['token', 'accessToken', 'refreshToken']) {
    if (Object.hasOwn(value, field)) {
      throw new Error(`Browser response exposed a credential in ${field}`);
    }
  }
}

export function requireCredentialFreeSessionResponse(value: unknown): BrowserSessionResponse {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('Invalid browser session response');
  }
  assertNoBrowserCredential(value);
  return value as BrowserSessionResponse;
}

export async function createBrowserSession(
  credentials: BrowserSessionCredentials,
): Promise<BrowserSessionResponse> {
  const result = await request<unknown>(BROWSER_SESSION_LOGIN_ENDPOINT, {
    method: 'POST',
    data: credentials,
    skipErrorHandler: true,
  });
  return requireCredentialFreeSessionResponse(result);
}

export async function refreshBrowserSession(): Promise<BrowserSessionResponse> {
  const result = await request<unknown>(BROWSER_SESSION_REFRESH_ENDPOINT, {
    method: 'POST',
    skipErrorHandler: true,
  });
  return requireCredentialFreeSessionResponse(result);
}

export async function clearStaleBrowserAuthCookie(): Promise<void> {
  await request<void>(BROWSER_SESSION_CLEAR_ENDPOINT, {
    method: 'POST',
    skipErrorHandler: true,
  });
}

export async function clearServerSession(): Promise<void> {
  await request<void>(BROWSER_SESSION_LOGOUT_ENDPOINT, {
    method: 'POST',
    skipErrorHandler: true,
  }).catch(async () => {
    await request<void>(BROWSER_SESSION_CLEAR_ENDPOINT, {
      method: 'POST',
      skipErrorHandler: true,
    });
  });
}
