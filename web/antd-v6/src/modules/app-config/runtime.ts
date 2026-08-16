import type { InitialState } from '@/shared/auth/types';
import { queryClient, queryKeys } from '@/shared/query/client';
import { loadApplicationProfile } from '@/shared/theme/api';
import { buildLayoutSettings } from '@/shared/theme/contract';
import { getThemeRuntimeSnapshot } from '@/shared/theme/runtime';

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
