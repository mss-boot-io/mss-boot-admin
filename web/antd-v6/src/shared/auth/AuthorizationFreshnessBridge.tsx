import { useModel } from '@umijs/max';
import { useCallback, useEffect, useRef } from 'react';
import { getRequestStatus } from '../api/errors';
import { queryClient, queryKeys } from '../query/client';
import { clearUserThemeRuntime } from '../theme/runtime';
import { clearThemeIdentitySession } from '../theme/snapshot';
import { fetchAuthorizedMenu } from './authorization';
import {
  AUTHORIZATION_REFRESH_EVENT,
  authorizationStateSignature,
  isAuthorizationBootstrapQuery,
  shouldRefreshAuthorization,
  subscribeAuthorizationRefreshBroadcast,
} from './freshness';
import type { AuthorizedMenuItem, CurrentUser, InitialState, StartupFailureArea } from './types';

type InitialStateUpdate =
  | InitialState
  | undefined
  | ((previous?: InitialState) => InitialState | undefined);

interface InitialStateModel {
  initialState?: InitialState;
  setInitialState: (update: InitialStateUpdate) => Promise<void>;
}

async function evictAuthorizationSensitiveQueries(): Promise<void> {
  const predicate = (query: { queryKey: readonly unknown[] }) =>
    !isAuthorizationBootstrapQuery(query.queryKey);
  await queryClient.cancelQueries({ predicate });
  queryClient.removeQueries({ predicate });
  queryClient.getMutationCache().clear();
}

function clearPersonalIdentityRuntime(): void {
  clearThemeIdentitySession();
  clearUserThemeRuntime();
}

export default function AuthorizationFreshnessBridge() {
  const { initialState, setInitialState } = useModel('@@initialState') as InitialStateModel;
  const stateRef = useRef(initialState);
  const lastRefreshAtRef = useRef(Date.now());
  const inFlightRef = useRef<Promise<void> | undefined>(undefined);
  const forceQueuedRef = useRef(false);
  stateRef.current = initialState;

  const failClosed = useCallback(
    async (area: StartupFailureArea, error?: unknown) => {
      const current = stateRef.current;
      await evictAuthorizationSensitiveQueries();
      if (area === 'identity') {
        queryClient.removeQueries({ queryKey: queryKeys.currentUser, exact: true });
      } else if (current?.currentUser?.id) {
        queryClient.removeQueries({
          queryKey: queryKeys.authorizedMenu(current.currentUser.id),
          exact: true,
        });
      }
      await setInitialState((previous) =>
        previous
          ? {
              ...previous,
              authorizedMenu: [],
              authorizationVersion: (previous.authorizationVersion ?? 0) + 1,
              startupFailure: { area, status: getRequestStatus(error) },
            }
          : previous,
      );
    },
    [setInitialState],
  );

  const reconcile = useCallback(async () => {
    const before = stateRef.current;
    if (!before?.currentUser || !before.fetchCurrentUser) return;

    let currentUser: CurrentUser | undefined;
    try {
      currentUser = await before.fetchCurrentUser();
    } catch (error) {
      await failClosed('identity', error);
      return;
    }
    if (!currentUser) {
      queryClient.clear();
      clearPersonalIdentityRuntime();
      await setInitialState((previous) =>
        previous
          ? {
              ...previous,
              currentUser: undefined,
              authorizedMenu: [],
              authorizationVersion: (previous.authorizationVersion ?? 0) + 1,
              startupFailure: undefined,
            }
          : previous,
      );
      return;
    }
    if (currentUser.id !== before.currentUser.id) {
      queryClient.clear();
      clearPersonalIdentityRuntime();
      await setInitialState((previous) =>
        previous
          ? {
              ...previous,
              currentUser: undefined,
              authorizedMenu: [],
              authorizationVersion: (previous.authorizationVersion ?? 0) + 1,
              startupFailure: { area: 'identity' },
            }
          : previous,
      );
      return;
    }

    let authorizedMenu: AuthorizedMenuItem[];
    try {
      authorizedMenu = await fetchAuthorizedMenu(currentUser);
    } catch (error) {
      queryClient.setQueryData(queryKeys.currentUser, currentUser);
      await failClosed('authorization', error);
      return;
    }

    const changed =
      authorizationStateSignature(before.currentUser, before.authorizedMenu) !==
      authorizationStateSignature(currentUser, authorizedMenu);
    if (changed) await evictAuthorizationSensitiveQueries();
    queryClient.setQueryData(queryKeys.currentUser, currentUser);
    queryClient.setQueryData(queryKeys.authorizedMenu(currentUser.id), authorizedMenu);
    await setInitialState((previous) =>
      previous
        ? {
            ...previous,
            currentUser,
            authorizedMenu,
            authorizationVersion: changed
              ? (previous.authorizationVersion ?? 0) + 1
              : previous.authorizationVersion,
            startupFailure: undefined,
          }
        : previous,
    );
  }, [failClosed, setInitialState]);

  const refresh = useCallback(
    async (force = false): Promise<void> => {
      const now = Date.now();
      if (!force && !shouldRefreshAuthorization(lastRefreshAtRef.current, now)) return;
      if (inFlightRef.current) {
        if (force) forceQueuedRef.current = true;
        await inFlightRef.current;
        return;
      }

      let runAgain = force;
      do {
        forceQueuedRef.current = false;
        lastRefreshAtRef.current = Date.now();
        const request = reconcile();
        inFlightRef.current = request;
        try {
          await request;
        } finally {
          if (inFlightRef.current === request) inFlightRef.current = undefined;
        }
        runAgain = forceQueuedRef.current;
      } while (runAgain);
    },
    [reconcile],
  );

  useEffect(() => {
    const refreshWhenVisible = () => {
      if (document.visibilityState === 'visible') void refresh(false);
    };
    const forceRefresh = () => void refresh(true);

    window.addEventListener('focus', refreshWhenVisible);
    window.addEventListener('online', forceRefresh);
    document.addEventListener('visibilitychange', refreshWhenVisible);
    window.addEventListener(AUTHORIZATION_REFRESH_EVENT, forceRefresh);
    const unsubscribeBroadcast = subscribeAuthorizationRefreshBroadcast(forceRefresh);
    return () => {
      window.removeEventListener('focus', refreshWhenVisible);
      window.removeEventListener('online', forceRefresh);
      document.removeEventListener('visibilitychange', refreshWhenVisible);
      window.removeEventListener(AUTHORIZATION_REFRESH_EVENT, forceRefresh);
      unsubscribeBroadcast();
    };
  }, [refresh]);

  return null;
}
