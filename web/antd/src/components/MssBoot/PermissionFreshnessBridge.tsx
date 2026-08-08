import { useModel } from '@umijs/max';
import React, { useCallback, useEffect, useRef } from 'react';
import { getAuthToken } from '@/utils/authStorage';
import { PERMISSION_REFRESH_EVENT, shouldRefreshPermissions } from '@/utils/permissionFreshness';

const PermissionFreshnessBridge: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const fetchUserInfo = initialState?.fetchUserInfo;
  const lastRefreshAtRef = useRef(Date.now());
  const inFlightRef = useRef<Promise<void>>();

  const refresh = useCallback(
    async function refreshPermissions(force = false): Promise<void> {
      if (!fetchUserInfo) return;
      const now = Date.now();
      if (!force && !shouldRefreshPermissions(lastRefreshAtRef.current, now)) return;
      if (inFlightRef.current) {
        const currentRequest = inFlightRef.current;
        await currentRequest;
        // A mutation-triggered refresh must not be swallowed by a passive
        // request that started before the authorization commit completed.
        if (force) return refreshPermissions(true);
        return;
      }

      lastRefreshAtRef.current = now;
      const request = (async () => {
        let currentUser: API.User | undefined;
        try {
          currentUser = await fetchUserInfo();
        } catch {
          // 401 cleanup and redirect already ran in the shared request error
          // handler. Other failures retain the last known identity below.
        }
        if (currentUser) {
          setInitialState((previous) => ({
            ...previous,
            currentUser,
            permissionRefreshVersion: (previous?.permissionRefreshVersion || 0) + 1,
          }));
          return;
        }
        // The shared request error handler owns 401 cleanup and redirect. Clear
        // the rendered identity only when that handler has removed the token;
        // transient network failures retain the last known UI state.
        if (!getAuthToken()) {
          setInitialState((previous) => ({
            ...previous,
            currentUser: undefined,
            permissionRefreshVersion: (previous?.permissionRefreshVersion || 0) + 1,
          }));
        }
      })();
      inFlightRef.current = request;
      try {
        await request;
      } finally {
        if (inFlightRef.current === request) inFlightRef.current = undefined;
      }
    },
    [fetchUserInfo, setInitialState],
  );

  useEffect(() => {
    const refreshWhenVisible = () => {
      if (document.visibilityState === 'visible') void refresh(false);
    };
    const forceRefresh = () => void refresh(true);

    window.addEventListener('focus', refreshWhenVisible);
    document.addEventListener('visibilitychange', refreshWhenVisible);
    window.addEventListener(PERMISSION_REFRESH_EVENT, forceRefresh);
    return () => {
      window.removeEventListener('focus', refreshWhenVisible);
      document.removeEventListener('visibilitychange', refreshWhenVisible);
      window.removeEventListener(PERMISSION_REFRESH_EVENT, forceRefresh);
    };
  }, [refresh]);

  return null;
};

export default PermissionFreshnessBridge;
