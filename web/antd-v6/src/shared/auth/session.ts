import { history, request } from '@umijs/max';
import { getRequestStatus } from '../api/errors';
import { normalizeCurrentUser } from './identity';
import {
  BROWSER_SESSION_REFRESH_LOCK,
  type BrowserSessionResponse,
  browserSessionRefreshDelay,
  clearBrowserSessionMetadata,
  readBrowserSessionExpiry,
  recordBrowserSessionResponse,
} from './sessionMetadata';
import type { CurrentUser } from './types';

export type { BrowserSessionResponse } from './sessionMetadata';
export {
  BROWSER_SESSION_METADATA_EVENT,
  BROWSER_SESSION_METADATA_KEY,
  BROWSER_SESSION_REFRESH_LOCK,
  BROWSER_SESSION_REFRESH_RETRY_MS,
  BROWSER_SESSION_REFRESH_SAFETY_MS,
  browserSessionRefreshDelay,
  clearBrowserSessionMetadata,
  readBrowserSessionExpiry,
  recordBrowserSessionResponse,
} from './sessionMetadata';

export const LOGIN_PATH = '/user/login';
export const BROWSER_SESSION_LOGIN_ENDPOINT = '/user/session/login';
export const BROWSER_SESSION_REFRESH_ENDPOINT = '/user/session/refresh-token';
export const BROWSER_SESSION_LOGOUT_ENDPOINT = '/online-sessions/logout';
export const BROWSER_SESSION_CLEAR_ENDPOINT = '/user/auth-cookie/clear';

export type BrowserSessionCredentials =
  | {
      username: string;
      password: string;
      type?: never;
    }
  | {
      type: 'email';
      email: string;
      captcha: string;
    }
  | {
      type: 'email_register';
      email: string;
      captcha: string;
      password: string;
    };

const PUBLIC_PATHS = new Set([LOGIN_PATH, '/user/register', '/user/forget']);
const OAUTH_CALLBACK_PATH = /^\/user\/(?:oauth\/)?callback\/[^/]+$/;

function browserLockManager(): LockManager | undefined {
  try {
    return typeof navigator !== 'undefined' && typeof navigator.locks?.request === 'function'
      ? navigator.locks
      : undefined;
  } catch {
    return undefined;
  }
}

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
  const response = value as Record<string, unknown>;
  if (response.code !== undefined && response.code !== 200) {
    throw new Error('Browser session response was not successful');
  }
  if (typeof response.expire !== 'string' || !Number.isFinite(Date.parse(response.expire))) {
    throw new Error('Browser session response has an invalid expiry');
  }
  return {
    ...(response.code === undefined ? {} : { code: response.code as number }),
    expire: response.expire,
  };
}

export async function createBrowserSession(
  credentials: BrowserSessionCredentials,
): Promise<BrowserSessionResponse> {
  const result = await request<unknown>(BROWSER_SESSION_LOGIN_ENDPOINT, {
    method: 'POST',
    data: credentials,
    skipErrorHandler: true,
  });
  const response = requireCredentialFreeSessionResponse(result);
  recordBrowserSessionResponse(response);
  return response;
}

export async function refreshBrowserSession(): Promise<BrowserSessionResponse> {
  const result = await request<unknown>(BROWSER_SESSION_REFRESH_ENDPOINT, {
    method: 'POST',
    skipErrorHandler: true,
  });
  const response = requireCredentialFreeSessionResponse(result);
  recordBrowserSessionResponse(response);
  return response;
}

export async function refreshBrowserSessionIfDue(now = Date.now()): Promise<number> {
  const currentExpiry = readBrowserSessionExpiry();
  if (currentExpiry !== undefined && browserSessionRefreshDelay(currentExpiry, now) > 0) {
    return currentExpiry;
  }

  const refresh = async () => {
    const lockedNow = Date.now();
    const lockedExpiry = readBrowserSessionExpiry();
    if (lockedExpiry !== undefined && browserSessionRefreshDelay(lockedExpiry, lockedNow) > 0) {
      return lockedExpiry;
    }
    const response = await refreshBrowserSession();
    return Date.parse(response.expire);
  };
  const locks = browserLockManager();
  return locks ? locks.request(BROWSER_SESSION_REFRESH_LOCK, refresh) : refresh();
}

export async function clearStaleBrowserAuthCookie(): Promise<void> {
  try {
    await request<void>(BROWSER_SESSION_CLEAR_ENDPOINT, {
      method: 'POST',
      skipErrorHandler: true,
    });
  } finally {
    clearBrowserSessionMetadata();
  }
}

export async function clearServerSession(): Promise<void> {
  try {
    await request<void>(BROWSER_SESSION_LOGOUT_ENDPOINT, {
      method: 'POST',
      skipErrorHandler: true,
    }).catch(async () => {
      await request<void>(BROWSER_SESSION_CLEAR_ENDPOINT, {
        method: 'POST',
        skipErrorHandler: true,
      });
    });
  } finally {
    clearBrowserSessionMetadata();
  }
}
