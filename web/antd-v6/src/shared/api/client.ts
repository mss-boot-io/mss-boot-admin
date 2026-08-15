import type { RequestConfig, RequestOptions } from '@umijs/max';
import { history } from '@umijs/max';
import { queryClient } from '../query/client';
import { feedback } from './feedback';

interface ApiErrorBody {
  code?: number | string;
  error?: string;
  errorMessage?: string;
  message?: string;
  msg?: string;
}

interface ResponseLike {
  status?: number;
  data?: ApiErrorBody;
}

interface RequestFailure extends Error {
  response?: ResponseLike;
  request?: unknown;
}

const mutationMethods = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

function readCookie(name: string): string | undefined {
  if (typeof document === 'undefined') return undefined;
  const prefix = `${encodeURIComponent(name)}=`;
  const entry = document.cookie
    .split(';')
    .map((cookie) => cookie.trim())
    .find((cookie) => cookie.startsWith(prefix));
  return entry ? decodeURIComponent(entry.slice(prefix.length)) : undefined;
}

function errorMessage(error: RequestFailure): string {
  const body = error.response?.data;
  return (
    body?.errorMessage ??
    body?.message ??
    body?.msg ??
    body?.error ??
    error.message ??
    'Request failed'
  );
}

export function browserRequestHeaders(
  method: string,
  initialHeaders?: HeadersInit,
  csrfToken = readCookie('mss_csrf'),
): Record<string, string> {
  const headers = new Headers(initialHeaders);
  headers.set('Accept', 'application/json');
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
      const error = rawError as RequestFailure;
      const status = error.response?.status;
      if (status === 401) {
        queryClient.clear();
        const redirect = encodeURIComponent(
          history.location.pathname + history.location.search + history.location.hash,
        );
        if (history.location.pathname !== '/user/login') {
          history.replace(`/user/login?redirect=${redirect}`);
        }
        if (options?.skipErrorHandler) throw error;
        return;
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
      feedback()?.message.error(errorMessage(error));
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
