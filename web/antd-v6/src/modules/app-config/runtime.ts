import type { InitialState } from '@mss-admin-core/shared/auth/types';
import { queryClient, queryKeys } from '@mss-admin-core/shared/query/client';
import { loadApplicationProfile } from '@mss-admin-core/shared/theme/api';
import { buildLayoutSettings } from '@mss-admin-core/shared/theme/contract';
import { getThemeRuntimeSnapshot } from '@mss-admin-core/shared/theme/runtime';

export type InitialStateSetter = (
  state: InitialState | ((previous?: InitialState) => InitialState | undefined),
) => Promise<void>;

export async function refreshApplicationRuntime(setInitialState: InitialStateSetter) {
  await queryClient.invalidateQueries({ queryKey: queryKeys.applicationProfile });
  const profile = await queryClient.fetchQuery({
    queryKey: queryKeys.applicationProfile,
    queryFn: loadApplicationProfile,
    staleTime: 0,
  });
  await setInitialState((previous) =>
    previous
      ? {
          ...previous,
          applicationProfile: profile,
          settings: buildLayoutSettings(getThemeRuntimeSnapshot().resolved.settings, profile.base),
        }
      : previous,
  );
}
