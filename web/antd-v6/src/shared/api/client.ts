import type { RequestConfig, RequestOptions } from '@umijs/max';
import { getIntl, history } from '@umijs/max';
import { requestAuthorizationRefresh, shouldRefreshAuthorization } from '../auth/freshness';
import { clearBrowserSessionMetadata } from '../auth/sessionMetadata';
import { queryClient } from '../query/client';
import { clearUserThemeRuntime } from '../theme/runtime';
import { clearThemeIdentitySession } from '../theme/snapshot';
import { type ApiRequestFailure, getRequestErrorMessage, getRequestStatus } from './errors';
import { feedback } from './feedback';

export { getRequestErrorCode, getRequestErrorMessage, getRequestStatus } from './errors';

const mutationMethods = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);
const axiosHeaderBuckets = new Set([
  'common',
  'delete',
  'get',
  'head',
  'options',
  'patch',
  'post',
  'put',
]);
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
  initialHeaders?: HeadersInit | Readonly<Record<string, unknown>>,
  csrfToken = readCookie('mss_csrf'),
): Record<string, string> {
  const headers = new Headers();
  const append = (source: unknown, skipAxiosBuckets = false): void => {
    if (!source) return;
    if (source instanceof Headers) {
      source.forEach((value, name) => {
        headers.set(name, value);
      });
      return;
    }
    if (Array.isArray(source)) {
      for (const entry of source) {
        if (!Array.isArray(entry) || entry.length !== 2) continue;
        const [name, value] = entry;
        if (typeof name === 'string' && typeof value === 'string') headers.set(name, value);
      }
      return;
    }
    if (typeof source !== 'object') return;
    for (const [name, value] of Object.entries(source)) {
      if (skipAxiosBuckets && axiosHeaderBuckets.has(name.toLowerCase())) continue;
      if (typeof value === 'string') headers.set(name, value);
      else if (typeof value === 'number' || typeof value === 'boolean') {
        headers.set(name, String(value));
      } else if (Array.isArray(value) && value.every((entry) => typeof entry === 'string')) {
        headers.set(name, value.join(', '));
      }
    }
  };

  const rawHeaders = initialHeaders as Readonly<Record<string, unknown>> | undefined;
  if (rawHeaders && !Array.isArray(rawHeaders) && !(rawHeaders instanceof Headers)) {
    append(rawHeaders.common);
    append(rawHeaders[method.toLowerCase()]);
    append(rawHeaders, true);
  } else {
    append(initialHeaders);
  }
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
        clearBrowserSessionMetadata();
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
        feedback()?.message.error(getIntl().formatMessage({ id: 'errors.offline' }));
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
