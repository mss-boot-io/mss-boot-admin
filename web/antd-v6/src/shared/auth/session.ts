import { history, request } from '@umijs/max';
import type { CurrentUser } from './types';

export const LOGIN_PATH = '/user/login';

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
    return await request<CurrentUser>('/user/userInfo', {
      method: 'GET',
      skipErrorHandler: true,
    });
  } catch {
    return undefined;
  }
}

export async function clearServerSession(): Promise<void> {
  await request<void>('/user/logout', {
    method: 'POST',
    skipErrorHandler: true,
  }).catch(async () => {
    await request<void>('/user/auth-cookie/clear', {
      method: 'POST',
      skipErrorHandler: true,
    });
  });
}
