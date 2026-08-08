export const PERMISSION_REFRESH_EVENT = 'mss:permission-refresh';
export const PERMISSION_REFRESH_THROTTLE_MS = 30_000;

export const shouldRefreshPermissions = (
  lastRefreshAt: number,
  now: number,
  throttleMs = PERMISSION_REFRESH_THROTTLE_MS,
) => now - lastRefreshAt >= throttleMs;

export const requestPermissionRefresh = () => {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event(PERMISSION_REFRESH_EVENT));
  }
};
