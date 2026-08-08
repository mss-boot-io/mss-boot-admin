import {
  PERMISSION_REFRESH_EVENT,
  requestPermissionRefresh,
  shouldRefreshPermissions,
} from './permissionFreshness';

describe('permission freshness utilities', () => {
  it('throttles passive refreshes at the configured boundary', () => {
    expect(shouldRefreshPermissions(1_000, 30_999, 30_000)).toBe(false);
    expect(shouldRefreshPermissions(1_000, 31_000, 30_000)).toBe(true);
  });

  it('publishes an explicit refresh event after authorization mutations', () => {
    const listener = jest.fn();
    window.addEventListener(PERMISSION_REFRESH_EVENT, listener);

    requestPermissionRefresh();

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener(PERMISSION_REFRESH_EVENT, listener);
  });
});
