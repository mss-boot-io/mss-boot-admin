import {
  applyCanonicalThemeResource,
  mergeAppliedThemeIntoInitialState,
} from '@mss-admin-core/modules/theme/apply';
import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { queryKeys } from '@mss-admin-core/shared/query/client';
import { useQueryClient } from '@tanstack/react-query';
import { useModel } from '@umijs/max';
import { useEffect } from 'react';
import { clearUserThemeRuntime, getThemeRuntimeSnapshot } from './runtime';

export function ThemeCrossTabBridge() {
  const client = useQueryClient();
  const { initialState, setInitialState } = useModel('@@initialState');
  const currentUserID = initialState?.currentUser?.id;
  const verifiedSessionID = initialState?.authSessionId;

  useEffect(() => {
    let active = true;
    let unsubscribe: (() => void) | undefined;
    void Promise.all([import('./sync'), import('./snapshot')])
      .then(
        ([
          { decideThemeScopeEvent, isThemeSyncEventFromCurrentDocument, subscribeThemeSync },
          snapshot,
        ]) => {
          if (!active) return;
          unsubscribe = subscribeThemeSync((event) => {
            if (event.type === 'identity-cleared') {
              if (
                isThemeSyncEventFromCurrentDocument(event) ||
                !verifiedSessionID ||
                event.previousAuthSessionId !== verifiedSessionID
              ) {
                return;
              }
              snapshot.clearThemeIdentitySession({
                broadcast: false,
                expectedSessionId: verifiedSessionID,
              });
              client.clear();
              clearUserThemeRuntime();
              window.location.reload();
              return;
            }

            const runtime = getThemeRuntimeSnapshot();
            const activeSessionID = snapshot.getVerifiedThemeAuthSessionId(currentUserID);
            const decision = decideThemeScopeEvent(event, runtime[event.scope], activeSessionID);
            if (decision === 'conflict') {
              void client.invalidateQueries({
                queryKey: queryKeys.theme(
                  event.scope,
                  event.scope === 'user' ? (currentUserID ?? '') : '',
                ),
              });
              return;
            }
            if (decision !== 'apply') return;
            const resource = {
              scope: event.scope,
              revision: event.revision,
              overrides: event.overrides,
            } as const;
            const owner = resource.scope === 'user' ? (currentUserID ?? '') : '';
            const applied = applyCanonicalThemeResource(client, resource, owner, {
              authoritative: false,
            });
            if (applied.status !== 'applied') return;
            void setInitialState((previous: InitialState | undefined) =>
              mergeAppliedThemeIntoInitialState(previous, applied),
            );
            void snapshot.writeThemeSnapshot(
              resource,
              resource.scope === 'user' ? activeSessionID : undefined,
            );
          });
        },
      )
      .catch(() => {
        // Authoritative page queries remain available when this optional
        // derived-state transport chunk cannot load.
      });
    return () => {
      active = false;
      unsubscribe?.();
    };
  }, [client, currentUserID, setInitialState, verifiedSessionID]);

  return null;
}
