import type { ProLayoutProps } from '@ant-design/pro-components';
import type { QueryClient } from '@tanstack/react-query';
import type { InitialState } from '@/shared/auth/types';
import { queryKeys } from '@/shared/query/client';
import {
  type ApplicationProfile,
  buildLayoutSettings,
  type ThemeScopeResource,
} from '@/shared/theme/contract';
import {
  getThemeRuntimeSnapshot,
  reconcileThemeResource,
  type ThemeReconcileStatus,
} from '@/shared/theme/runtime';

export interface AppliedThemeResource {
  resource: ThemeScopeResource;
  settings: Partial<ProLayoutProps>;
  degradedScopes: ThemeScopeResource['scope'][];
  status: ThemeReconcileStatus;
}

/**
 * Reconcile one server resource into the single runtime and Query cache. An
 * older response can never roll back a healthy newer revision already applied
 * by another query or tab.
 */
export function applyCanonicalThemeResource(
  client: QueryClient,
  incoming: ThemeScopeResource,
  owner = '',
  options: { authoritative?: boolean } = { authoritative: true },
): AppliedThemeResource {
  const status = reconcileThemeResource(incoming, options);
  const runtime = getThemeRuntimeSnapshot();
  const resource = runtime[incoming.scope] ?? incoming;
  client.setQueryData(queryKeys.theme(incoming.scope, owner), resource);

  const profile = client.getQueryData<ApplicationProfile>(queryKeys.applicationProfile);
  if (incoming.scope === 'application' && profile) {
    client.setQueryData<ApplicationProfile>(queryKeys.applicationProfile, {
      ...profile,
      theme: resource,
    });
  }

  return {
    resource,
    settings: buildLayoutSettings(runtime.resolved.settings, profile?.base),
    degradedScopes: runtime.degradedScopes,
    status,
  };
}

export function mergeAppliedThemeIntoInitialState(
  state: InitialState | undefined,
  applied: AppliedThemeResource,
): InitialState | undefined {
  if (!state) return state;
  return {
    ...state,
    settings: applied.settings,
    themeDegradedScopes:
      applied.degradedScopes.length > 0 ? [...applied.degradedScopes] : undefined,
  };
}
