import { getAppConfigsProfile } from '@/services/admin/appConfig';
import { getUserConfigsProfile } from '@/services/admin/userConfig';
import {
  areThemeOverridesEqual,
  clearUserThemeProfile,
  getVerifiedThemeAuthSessionId,
  getThemeScopeResource,
  markThemeScopeDegraded,
  reconcileThemeScopeResource,
  type ThemeScopeResource,
  type ThemeSettingsScope,
} from '@/utils/themeSettings';
import {
  applyThemeDocumentHints,
  getThemeAuthSessionId,
  writeThemeSnapshot,
} from '@/utils/themeSession';
import {
  isThemeSyncEventFromCurrentDocument,
  shouldReconcileThemeScopeEvent,
  subscribeThemeSync,
  type ThemeSyncEvent,
} from '@/utils/themeSync';
import { history, useModel } from '@umijs/max';
import React, { useCallback, useEffect, useRef } from 'react';

const VISIBILITY_RECONCILE_INTERVAL_MS = 30_000;
export const THEME_RECONCILE_TIMEOUT_MS = 4_000;

type PendingAuthoritativeSnapshot = {
  target: ThemeScopeResource;
  previous: ThemeScopeResource;
  authSessionId?: string;
};

const areThemeResourcesEqual = (left: ThemeScopeResource, right: ThemeScopeResource) =>
  left.scope === right.scope &&
  left.revision === right.revision &&
  areThemeOverridesEqual(left.overrides, right.overrides);

async function loadAuthoritativeThemeProfile(scope: ThemeSettingsScope) {
  const controller = typeof AbortController === 'undefined' ? undefined : new AbortController();
  let timeout: ReturnType<typeof setTimeout> | undefined;
  try {
    return await Promise.race([
      scope === 'application'
        ? getAppConfigsProfile({ skipErrorHandler: true, signal: controller?.signal })
        : getUserConfigsProfile({ skipErrorHandler: true, signal: controller?.signal }),
      new Promise<never>((_, reject) => {
        timeout = setTimeout(() => {
          controller?.abort();
          reject(new Error(`Timed out reconciling the ${scope} theme`));
        }, THEME_RECONCILE_TIMEOUT_MS);
      }),
    ]);
  } finally {
    if (timeout) clearTimeout(timeout);
  }
}

const ThemeRuntimeBridge: React.FC = () => {
  const { initialState, setInitialState } = useModel('@@initialState');
  const stateRef = useRef(initialState);
  const reconcilingRef = useRef<Partial<Record<ThemeSettingsScope, Promise<void>>>>({});
  const queuedReconcileRef = useRef<Partial<Record<ThemeSettingsScope, boolean>>>({});
  const lastVisibilityReconcileRef = useRef(0);
  const pendingAuthoritativeSnapshotRef = useRef<
    Partial<Record<ThemeSettingsScope, PendingAuthoritativeSnapshot>>
  >({});

  useEffect(() => {
    stateRef.current = initialState;
  }, [initialState]);

  const runtimeNavTheme = initialState?.settings?.navTheme;
  const runtimeColorPrimary = initialState?.settings?.colorPrimary;
  const runtimeApplicationResource = initialState?.themeRuntime?.layers.application;
  const runtimeUserResource = initialState?.themeRuntime?.layers.user;
  const runtimeApplicationDegraded =
    initialState?.themeRuntime?.degradedScopes?.includes('application') ?? false;
  const runtimeUserDegraded =
    initialState?.themeRuntime?.degradedScopes?.includes('user') ?? false;
  const runtimeSnapshotAuthSessionId = getVerifiedThemeAuthSessionId(
    initialState,
    getThemeAuthSessionId(),
  );

  useEffect(() => {
    if (!runtimeNavTheme || !runtimeColorPrimary) return;
    applyThemeDocumentHints({
      navTheme: runtimeNavTheme,
      colorPrimary: runtimeColorPrimary,
    });
  }, [runtimeColorPrimary, runtimeNavTheme]);

  useEffect(() => {
    // Persist only layers from the committed React runtime state. A request
    // preview can lose a race with a newer functional state update, so writing
    // inside the request callback could cache a resource the runtime rejected.
    if (runtimeApplicationResource) {
      const pending = pendingAuthoritativeSnapshotRef.current.application;
      delete pendingAuthoritativeSnapshotRef.current.application;
      const authoritativeOptions =
        pending && areThemeResourcesEqual(pending.target, runtimeApplicationResource)
          ? { authoritativePrevious: pending.previous }
          : undefined;
      if (!runtimeApplicationDegraded || authoritativeOptions) {
        void writeThemeSnapshot(
          runtimeApplicationResource,
          undefined,
          undefined,
          authoritativeOptions,
        );
      }
    }
    if (runtimeUserResource && runtimeSnapshotAuthSessionId) {
      const pending = pendingAuthoritativeSnapshotRef.current.user;
      delete pendingAuthoritativeSnapshotRef.current.user;
      const authoritativeOptions =
        pending &&
          pending.authSessionId === runtimeSnapshotAuthSessionId &&
          areThemeResourcesEqual(pending.target, runtimeUserResource)
          ? { authoritativePrevious: pending.previous }
          : undefined;
      if (!runtimeUserDegraded || authoritativeOptions) {
        void writeThemeSnapshot(
          runtimeUserResource,
          runtimeSnapshotAuthSessionId,
          undefined,
          authoritativeOptions,
        );
      }
    }
  }, [
    runtimeApplicationDegraded,
    runtimeApplicationResource,
    runtimeSnapshotAuthSessionId,
    runtimeUserDegraded,
    runtimeUserResource,
  ]);

  const applyResource = useCallback(
    (
      resource: ThemeScopeResource,
      options: {
        allowLegacyReplace?: boolean;
        authSessionId?: string;
        authoritative?: boolean;
      } = {},
    ) => {
      const activeStorageSessionId = getThemeAuthSessionId();
      const verifiedAuthSessionId = getVerifiedThemeAuthSessionId(
        stateRef.current,
        activeStorageSessionId,
      );
      const authSessionId =
        resource.scope === 'user' ? options.authSessionId ?? verifiedAuthSessionId : undefined;
      if (
        resource.scope === 'user' &&
        (!authSessionId || verifiedAuthSessionId !== authSessionId)
      ) {
        return 'stale' as const;
      }

      const preview = reconcileThemeScopeResource(stateRef.current || {}, resource, {
        ...options,
        authSessionId,
      });
      if (preview.status !== 'applied') {
        return preview.status;
      }
      stateRef.current = preview.state as typeof stateRef.current;
      setInitialState((state) => {
        const previous = state?.themeRuntime?.layers?.[resource.scope];
        const result = reconcileThemeScopeResource(state || {}, resource, {
          ...options,
          authSessionId,
        });
        if (result.status === 'applied') {
          if (options.authoritative && previous) {
            const existingPending =
              pendingAuthoritativeSnapshotRef.current[resource.scope];
            const continuesPendingReplacement =
              existingPending &&
              areThemeResourcesEqual(existingPending.target, previous) &&
              (resource.scope !== 'user' ||
                existingPending.authSessionId === authSessionId);
            const target =
              result.state.themeRuntime?.layers?.[resource.scope] || resource;
            pendingAuthoritativeSnapshotRef.current[resource.scope] = {
              target,
              previous: continuesPendingReplacement
                ? existingPending.previous
                : previous,
              authSessionId,
            };
          } else {
            delete pendingAuthoritativeSnapshotRef.current[resource.scope];
          }
        }
        stateRef.current = result.state as typeof stateRef.current;
        return result.state as typeof state;
      });
      return 'applied' as const;
    },
    [setInitialState],
  );

  const reconcileAuthoritative = useCallback(
    (scope: ThemeSettingsScope) => {
      if (reconcilingRef.current[scope]) {
        queuedReconcileRef.current[scope] = true;
        return reconcilingRef.current[scope]!;
      }
      const request = async () => {
        try {
          do {
            queuedReconcileRef.current[scope] = false;
            try {
              let expectedAuthSessionId: string | undefined;
              if (scope === 'user' && !stateRef.current?.currentUser) {
                return;
              }
              if (scope === 'user') {
                expectedAuthSessionId = getVerifiedThemeAuthSessionId(
                  stateRef.current,
                  getThemeAuthSessionId(),
                );
                if (!expectedAuthSessionId) return;
              }
              const profile = await loadAuthoritativeThemeProfile(scope);
              if (scope === 'user') {
                const currentAuthSessionId = getVerifiedThemeAuthSessionId(
                  stateRef.current,
                  getThemeAuthSessionId(),
                );
                if (currentAuthSessionId !== expectedAuthSessionId) {
                  if (currentAuthSessionId && stateRef.current?.currentUser) {
                    queuedReconcileRef.current[scope] = true;
                    continue;
                  }
                  return;
                }
              }
              const resource = getThemeScopeResource(profile, scope);
              applyResource(resource, {
                allowLegacyReplace: !resource.versioned,
                authSessionId: expectedAuthSessionId,
                authoritative: true,
              });
            } catch {
              setInitialState(
                (state) => markThemeScopeDegraded(state || {}, scope) as typeof state,
              );
            }
          } while (queuedReconcileRef.current[scope]);
        } finally {
          delete reconcilingRef.current[scope];
          delete queuedReconcileRef.current[scope];
        }
      };
      // Defer execution by one microtask so the in-flight marker is installed
      // even when a user-scope reconciliation returns before its first await.
      const promise = Promise.resolve().then(request);
      reconcilingRef.current[scope] = promise;
      return promise;
    },
    [applyResource, setInitialState],
  );

  useEffect(() => {
    const degradedScopes = stateRef.current?.themeRuntime?.degradedScopes || [];
    if (degradedScopes.includes('application')) {
      void reconcileAuthoritative('application');
    }
    if (
      degradedScopes.includes('user') &&
      stateRef.current?.currentUser &&
      getVerifiedThemeAuthSessionId(stateRef.current, getThemeAuthSessionId())
    ) {
      void reconcileAuthoritative('user');
    }
  }, [reconcileAuthoritative]);

  useEffect(() => {
    const unsubscribe = subscribeThemeSync((event: ThemeSyncEvent) => {
      const currentState = stateRef.current;
      if (event.type === 'identity-cleared') {
        if (currentState?.themeRuntime?.authSessionId === event.previousAuthSessionId) {
          setInitialState((state) => {
            const next = {
              ...clearUserThemeProfile(state || {}),
              currentUser: undefined,
            };
            stateRef.current = next as typeof stateRef.current;
            return next as typeof state;
          });
          // A remote logout or account switch changes more than the theme:
          // current user, RBAC and dynamic menus must all be bootstrapped from
          // the new shared token. The originating document already owns that
          // transition and must not reload itself mid-login/logout.
          if (!isThemeSyncEventFromCurrentDocument(event)) {
            history.go(0);
          }
        }
        return;
      }

      const authSessionId = getVerifiedThemeAuthSessionId(currentState, getThemeAuthSessionId());
      if (event.scope === 'user' && event.authSessionId !== authSessionId) {
        return;
      }
      const current = currentState?.themeRuntime?.layers?.[event.scope];
      if (shouldReconcileThemeScopeEvent(event, current, authSessionId)) {
        // Cross-document messages are untrusted invalidation hints. Only the
        // authoritative scope endpoint may supply values that enter runtime.
        void reconcileAuthoritative(event.scope);
      }
    });

    const onVisibility = () => {
      if (document.visibilityState !== 'visible') return;
      const now = Date.now();
      if (now - lastVisibilityReconcileRef.current < VISIBILITY_RECONCILE_INTERVAL_MS) return;
      lastVisibilityReconcileRef.current = now;
      void reconcileAuthoritative('application');
      const activeStorageSessionId = getThemeAuthSessionId();
      const verifiedSessionId = getVerifiedThemeAuthSessionId(
        stateRef.current,
        activeStorageSessionId,
      );
      if (stateRef.current?.currentUser && verifiedSessionId) {
        void reconcileAuthoritative('user');
      } else if (
        stateRef.current?.themeRuntime?.authSessionId &&
        stateRef.current.themeRuntime.authSessionId !== activeStorageSessionId
      ) {
        setInitialState((state) => {
          const next = clearUserThemeProfile(state || {});
          stateRef.current = next as typeof stateRef.current;
          return next as typeof state;
        });
      }
    };
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      unsubscribe();
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [applyResource, reconcileAuthoritative, setInitialState]);

  return null;
};

export default ThemeRuntimeBridge;
