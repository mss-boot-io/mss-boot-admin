import { useModel } from '@umijs/max';
import { useEffect } from 'react';
import { getRequestStatus } from '../api/errors';
import {
  requestAuthorizationRefresh,
  requestAuthorizationRevisionRefresh,
} from '../auth/freshness';
import { redirectToLogin } from '../auth/session';
import type { InitialState } from '../auth/types';
import { queryClient } from '../query/client';
import { clearUserThemeRuntime } from '../theme/runtime';
import { clearThemeIdentitySession } from '../theme/snapshot';
import { startAuthorizationRealtimeSession } from './socket';

type InitialStateUpdate =
  | InitialState
  | undefined
  | ((previous?: InitialState) => InitialState | undefined);

interface InitialStateModel {
  initialState?: InitialState;
  setInitialState: (update: InitialStateUpdate) => Promise<void>;
}

export default function AuthorizationRealtimeBridge() {
  const { initialState, setInitialState } = useModel('@@initialState') as InitialStateModel;
  const currentUserID = initialState?.currentUser?.id;

  useEffect(() => {
    if (!currentUserID) return;
    return startAuthorizationRealtimeSession({
      onAuthorizationRevision: requestAuthorizationRevisionRefresh,
      onReconnected: requestAuthorizationRefresh,
      onKick: () => {
        queryClient.clear();
        clearThemeIdentitySession();
        clearUserThemeRuntime();
        void setInitialState((previous) =>
          previous
            ? {
                ...previous,
                currentUser: undefined,
                authorizedMenu: [],
                authorizationVersion: (previous.authorizationVersion ?? 0) + 1,
                startupFailure: undefined,
              }
            : previous,
        ).catch(() => {});
        redirectToLogin();
      },
      shouldReconnect: (error) => getRequestStatus(error) !== 401,
    });
  }, [currentUserID, setInitialState]);

  return null;
}
