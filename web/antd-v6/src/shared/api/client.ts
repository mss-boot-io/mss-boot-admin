import type { RequestConfig, RequestOptions } from '@umijs/max';
import { history } from '@umijs/max';
import { requestAuthorizationRefresh, shouldRefreshAuthorization } from '../auth/freshness';
import { queryClient } from '../query/client';
import { clearUserThemeRuntime } from '../theme/runtime';
import { clearThemeIdentitySession } from '../theme/snapshot';
import { type ApiRequestFailure, getRequestErrorMessage, getRequestStatus } from './errors';
import { feedback } from './feedback';

export { getRequestErrorCode, getRequestErrorMessage, getRequestStatus } from './errors';

const mutationMethods = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);
let lastForbiddenRefreshAt: number | undefined;

function refreshAuthorizationAfterForbidden(): void {
  const now = Date.now();
  if (
    lastForbiddenRefreshAt === undefined ||
    shouldRefreshAuthorization(lastForbiddenRefreshAt, now)
  ) {
    lastForbiddenRefreshAt = now;
    requestAuthorizationRefresh();
  }
}

function readCookie(name: string): string | undefined {
  if (typeof document === 'undefined') return undefined;
  const prefix = `${encodeURIComponent(name)}=`;
  const entry = document.cookie
    .split(';')
    .map((cookie) => cookie.trim())
    .find((cookie) => cookie.startsWith(prefix));
  return entry ? decodeURIComponent(entry.slice(prefix.length)) : undefined;
}

export function browserRequestHeaders(
  method: string,
  initialHeaders?: HeadersInit,
  csrfToken = readCookie('mss_csrf'),
): Record<string, string> {
  const headers = new Headers(initialHeaders);
  if (!headers.has('Accept')) headers.set('Accept', 'application/json');
  if (mutationMethods.has(method.toUpperCase())) {
    if (csrfToken) headers.set('X-CSRF-Token', csrfToken);
  }
  return Object.fromEntries(headers.entries());
}

export const requestConfig: RequestConfig = {
  baseURL: '/admin/api',
  timeout: 20_000,
  withCredentials: true,
  errorConfig: {
    errorHandler: (rawError, options) => {
      const error = rawError as ApiRequestFailure;
      const status = getRequestStatus(error);
      if (status === 401) {
        queryClient.clear();
        clearThemeIdentitySession();
        clearUserThemeRuntime();
        const redirect = encodeURIComponent(
          history.location.pathname + history.location.search + history.location.hash,
        );
        if (history.location.pathname !== '/user/login') {
          history.replace(`/user/login?redirect=${redirect}`);
        }
        if (options?.skipErrorHandler) throw error;
        return;
      }
      if (
        status === 403 &&
        !(options as { skipAuthorizationRefresh?: boolean } | undefined)?.skipAuthorizationRefresh
      ) {
        refreshAuthorizationAfterForbidden();
      }
      if (options?.skipErrorHandler) throw error;
      if (status === 403) {
        history.push('/403');
        return;
      }
      if (typeof navigator !== 'undefined' && !navigator.onLine) {
        feedback()?.message.error('网络不可用，请检查连接后重试');
        return;
      }
      feedback()?.message.error(getRequestErrorMessage(error));
    },
  },
  requestInterceptors: [
    (url, rawOptions) => {
      const options = rawOptions as RequestOptions;
      const method = (options.method ?? 'GET').toUpperCase();
      return {
        url,
        options: {
          ...options,
          credentials: 'include',
          headers: browserRequestHeaders(method, options.headers as HeadersInit | undefined),
        },
      };
    },
  ],
};
