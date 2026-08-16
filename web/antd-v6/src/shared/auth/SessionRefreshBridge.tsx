import { useModel } from '@umijs/max';
import { useEffect } from 'react';
import { getRequestStatus } from '../api/errors';
import {
  BROWSER_SESSION_METADATA_EVENT,
  BROWSER_SESSION_METADATA_KEY,
  BROWSER_SESSION_REFRESH_RETRY_MS,
  browserSessionRefreshDelay,
  clearBrowserSessionMetadata,
  readBrowserSessionExpiry,
  redirectToLogin,
  refreshBrowserSessionIfDue,
} from './session';
import type { InitialState } from './types';

function isDocumentVisible(): boolean {
  return typeof document === 'undefined' || document.visibilityState !== 'hidden';
}

export default function SessionRefreshBridge() {
  const { initialState } = useModel('@@initialState') as { initialState?: InitialState };
  const subject = initialState?.currentUser?.id;

  useEffect(() => {
    if (!subject || typeof window === 'undefined') return;
    let stopped = false;
    let refreshing = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let lastKnownExpiry = readBrowserSessionExpiry();

    const clearTimer = () => {
      if (timer !== undefined) clearTimeout(timer);
      timer = undefined;
    };

    const schedule = (expiresAt = readBrowserSessionExpiry() ?? lastKnownExpiry) => {
      clearTimer();
      if (stopped || !isDocumentVisible()) return;
      if (expiresAt === undefined) {
        timer = setTimeout(runRefresh, 0);
        return;
      }
      lastKnownExpiry = expiresAt;
      const delay = browserSessionRefreshDelay(expiresAt);
      timer = setTimeout(runRefresh, delay);
    };

    const runRefresh = async () => {
      if (stopped || refreshing || !isDocumentVisible()) return;
      refreshing = true;
      try {
        lastKnownExpiry = await refreshBrowserSessionIfDue();
        if (browserSessionRefreshDelay(lastKnownExpiry) === 0) {
          timer = setTimeout(runRefresh, BROWSER_SESSION_REFRESH_RETRY_MS);
        } else {
          schedule(lastKnownExpiry);
        }
      } catch (error) {
        if (getRequestStatus(error) === 401) {
          clearBrowserSessionMetadata();
          redirectToLogin();
          return;
        }
        if (!stopped) timer = setTimeout(runRefresh, BROWSER_SESSION_REFRESH_RETRY_MS);
      } finally {
        refreshing = false;
      }
    };

    const rescheduleFromMetadata = () => {
      lastKnownExpiry = readBrowserSessionExpiry();
      schedule(lastKnownExpiry);
    };
    const onStorage = (event: StorageEvent) => {
      if (event.key === BROWSER_SESSION_METADATA_KEY) rescheduleFromMetadata();
    };
    const onVisibility = () => {
      if (isDocumentVisible()) rescheduleFromMetadata();
      else clearTimer();
    };

    window.addEventListener('storage', onStorage);
    window.addEventListener(BROWSER_SESSION_METADATA_EVENT, rescheduleFromMetadata);
    document.addEventListener('visibilitychange', onVisibility);
    schedule(lastKnownExpiry);
    return () => {
      stopped = true;
      clearTimer();
      window.removeEventListener('storage', onStorage);
      window.removeEventListener(BROWSER_SESSION_METADATA_EVENT, rescheduleFromMetadata);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [subject]);

  return null;
}
