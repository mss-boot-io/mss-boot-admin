import { getRequestStatus } from '@mss-admin-core/shared/api/errors';
import type { MonitorSnapshot } from './contract';

export const MONITOR_BASE_REFRESH_MS = 5_000;
export const MONITOR_MAX_REFRESH_MS = 60_000;

function responseHeaders(error: unknown): unknown {
  if (!error || typeof error !== 'object') return undefined;
  const candidate = error as { response?: { headers?: unknown }; headers?: unknown };
  return candidate.response?.headers ?? candidate.headers;
}

export function monitorRetryAfterDelay(error: unknown, now = Date.now()): number | undefined {
  const headers = responseHeaders(error);
  if (!headers || typeof headers !== 'object') return undefined;
  const getter = (headers as { get?: (name: string) => unknown }).get;
  const value =
    typeof getter === 'function'
      ? getter.call(headers, 'Retry-After')
      : Object.entries(headers).find(([key]) => key.toLowerCase() === 'retry-after')?.[1];
  if (value === undefined || value === null) return undefined;
  const raw = String(value).trim();
  if (!raw) return undefined;
  const seconds = Number(raw);
  const delay = Number.isFinite(seconds) ? seconds * 1_000 : Date.parse(raw) - now;
  if (!Number.isFinite(delay) || delay <= 0) return undefined;
  return Math.min(MONITOR_MAX_REFRESH_MS, Math.max(1_000, Math.ceil(delay)));
}

export function monitorRefetchInterval(
  snapshot: MonitorSnapshot | undefined,
  error: unknown,
  failureCount: number,
): number | false {
  const status = getRequestStatus(error);
  if (status === 401 || status === 403) return false;
  if (error) {
    if (status === 503) {
      return Math.max(MONITOR_BASE_REFRESH_MS, monitorRetryAfterDelay(error) ?? 0);
    }
    const exponent = Math.min(4, Math.max(0, failureCount - 1));
    return Math.min(MONITOR_MAX_REFRESH_MS, MONITOR_BASE_REFRESH_MS * 2 ** exponent);
  }
  return Math.min(
    MONITOR_MAX_REFRESH_MS,
    Math.max(MONITOR_BASE_REFRESH_MS, snapshot?.sampleIntervalMs ?? MONITOR_BASE_REFRESH_MS),
  );
}
