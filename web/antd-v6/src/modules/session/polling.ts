import { getRequestStatus } from '@/shared/api/errors';

export const ONLINE_SESSION_REFRESH_MS = 30_000;

export function onlineSessionRefetchInterval(error: unknown): number | false {
  const status = getRequestStatus(error);
  return status === 401 || status === 403 ? false : ONLINE_SESSION_REFRESH_MS;
}
